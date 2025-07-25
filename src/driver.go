package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/thorbenw/example-docker-volume-plugin/proc"
	"github.com/thorbenw/example-docker-volume-plugin/utils"
)

const (
	DefaultControlFileName  = "volumes.json"
	DefaultControlFileMode  = 0o664
	MinimumVolumeFolderMode = os.ModeDir | 0o700
	DefaultVolumeFolderMode = os.ModeDir | 0o764
)

var (
	// Package internal map of process monitors in order to have process
	// monitors running only once per volume.
	//
	// If process monitors were stored as a field in a volume object, their
	// goroutines continue running even if a driver object is recreated (and
	// thus all volume objects).
	processMonitors map[string]*proc.ProcessMonitor = make(map[string]*proc.ProcessMonitor)
)

type VolumeProcess func(string) *exec.Cmd

type exampleDriverMount struct {
	ReferenceCount int
}

type exampleDriverVolume struct {
	BasePath   string
	Path       string
	mountPoint string
	CreatedAt  time.Time
	Mounts     *map[string]exampleDriverMount
	Options    map[string]string
	Puid       string
}

func (v *exampleDriverVolume) MountPoint() string {
	if len(v.mountPoint) < 1 {
		v.mountPoint = filepath.Join(v.BasePath, v.Path)
	}

	return v.mountPoint
}

func (v *exampleDriverVolume) SetupProcess(d *exampleDriver) error {
	if strings.TrimSpace(v.Puid) == "" && d.VolumeProcess != nil {
		// Create and detach process

		cmd := *d.VolumeProcess(v.MountPoint())
		if cmd.Cancel != nil || cmd.WaitDelay != 0 {
			return d.Tee(fmt.Errorf("command must not use a context"))
		}
		if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
			return d.Tee(fmt.Errorf("command must not use custom standard files"))
		}
		if cmd.Process != nil || cmd.ProcessState != nil {
			return d.Tee(fmt.Errorf("command has already been started or run"))
		}

		attr := os.ProcAttr{
			Dir:   cmd.Dir,
			Env:   cmd.Env,
			Files: append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, cmd.ExtraFiles...),
			Sys:   cmd.SysProcAttr,
		}
		if pid, err := os.StartProcess(cmd.Path, cmd.Args, &attr); err != nil {
			return d.Tee(err)
		} else {
			wpid := pid.Pid
			if err := pid.Release(); err != nil {
				return d.Tee(err)
			}

			if prc, err := proc.GetProcessInfo(wpid); err != nil {
				return d.Tee(err)
			} else {
				v.Puid = prc.UniqueId()
				d.Logger.Debug("Started a new volume process.", "process", prc, "volume", v)
			}
		}
	}

	if strings.TrimSpace(v.Puid) != "" {
		// Pick up process
		if prc, err := proc.GetProcessInfoFromUniqueId(v.Puid); err != nil {
			d.Logger.Warn("PUID is invalid.", "err", err, "volume", v)
		} else {
			if pid, err := os.FindProcess(int(prc.Pid)); err != nil {
				d.Logger.Warn("PID is invalid.", "err", err, "volume", v, "prc", prc)
			} else {
				if _, ok := processMonitors[v.Puid]; !ok {
					if processMonitor, err := proc.MonitorProcess(pid.Pid, d.VolumeProcessRecoveryMode); err != nil {
						d.Logger.Warn("Faild to monitor process.", "err", err, "volume", v, "prc", prc, "pid", pid)
					} else {
						processMonitors[v.Puid] = processMonitor
					}
				}

			}
		}
	}

	return nil
}

type exampleDriver struct {
	PropagatedMount string
	slog.Logger
	Volumes map[string]exampleDriverVolume
	*sync.Mutex
	ControlFile *os.File
	VolumeProcess
	VolumeProcessRecoveryMode proc.RecoveryMode
}

func exampleDriver_New(propagatedMount string, logger slog.Logger) (*exampleDriver, error) {
	return exampleDriver_NewWithVolumeProcess(propagatedMount, logger, nil, proc.RecoveryModeIgnore)
}

func exampleDriver_NewWithVolumeProcess(propagatedMount string, logger slog.Logger, volumeProcess VolumeProcess, recoveryMode proc.RecoveryMode) (*exampleDriver, error) {
	if fileInfo, err := os.Lstat(propagatedMount); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if err := os.MkdirAll(propagatedMount, os.ModeDir); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		if !fileInfo.IsDir() {
			return nil, fmt.Errorf("path [%s] is not a directory", propagatedMount)
		}
	}

	controlFileName := DefaultControlFileName
	controlFile, err := os.OpenFile(filepath.Join(propagatedMount, controlFileName), os.O_RDWR|os.O_CREATE, DefaultControlFileMode)
	if err != nil {
		return nil, err
	}

	volumes, err := exampleDriver_Load(*controlFile)
	if err != nil {
		controlFile.Close()
		return nil, err
	}

	d := &exampleDriver{
		PropagatedMount: propagatedMount,
		Logger:          logger,
		//Volumes:               volumes,
		Mutex:                     &sync.Mutex{},
		ControlFile:               controlFile,
		VolumeProcess:             volumeProcess,
		VolumeProcessRecoveryMode: recoveryMode,
	}

	mountCount := 0
	for name, vol := range volumes {
		//filepath.EvalSymlinks()
		if err := utils.CheckAccess(utils.CheckAccessCurrentUser, os.FileMode(0o7), vol.MountPoint()); err != nil {
			return nil, err
		}
		mountCount += len(*vol.Mounts)

		if err := vol.SetupProcess(d); err != nil {
			d.Logger.Warn("Setting up the volume process failed.", "volume", vol)
		}

		volumes[name] = vol
	}
	logger.Debug("Loaded volume information.", "volumeCount", len(volumes), "mountCount", mountCount)

	d.Volumes = volumes
	return d, nil
}

func exampleDriver_Load(file os.File) (data map[string]exampleDriverVolume, err error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	bytes, err := io.ReadAll(&file)
	if err != nil {
		return nil, err
	}

	if len(bytes) < 1 {
		return map[string]exampleDriverVolume{}, nil
	}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, err
	}

	return
}

func exampleDriver_Save(file os.File, data map[string]exampleDriverVolume) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if err := file.Truncate(0); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if _, err := file.Write(bytes); err != nil {
		return err
	}

	return nil
}

func (d exampleDriver) Tee(err error) error {
	d.Logger.Error(err.Error())

	return err
}

func (d exampleDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	d.Logger.Debug("Get() has been called.", "req", req)

	if vol, ok := d.Volumes[req.Name]; !ok {
		return nil, d.Tee(fmt.Errorf("volume [%s] could not be found", req.Name))
	} else {
		res := volume.GetResponse{
			Volume: &volume.Volume{
				Name:       req.Name,
				Mountpoint: vol.MountPoint(),
				CreatedAt:  vol.CreatedAt.Format(time.RFC3339),
				//Status: map[string]slog.Attr{"example":"value"},
			},
		}

		d.Logger.Debug(fmt.Sprintf("Get() successfully looked up volume [%s].", req.Name), "res", res)
		return &res, nil
	}
}

func (d exampleDriver) Create(req *volume.CreateRequest) error {
	d.Logger.Debug("Create() has been called.", "req", req)

	if _, ok := d.Volumes[req.Name]; ok {
		return d.Tee(fmt.Errorf("volume [%s] already exists", req.Name))
	}
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	volumePathRel := utils.SHA256StringToString(req.Name)
	volumePathAbs := filepath.Join(d.PropagatedMount, volumePathRel)
	if _, err := os.Lstat(volumePathAbs); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(volumePathAbs, DefaultVolumeFolderMode); err != nil {
				return d.Tee(err)
			}
		} else {
			return d.Tee(err)
		}
	} else {
		return d.Tee(fmt.Errorf("path [%s] already exists", volumePathAbs))
	}

	res := exampleDriverVolume{
		BasePath:  d.PropagatedMount,
		Path:      volumePathRel,
		CreatedAt: time.Now(),
		Mounts:    &map[string]exampleDriverMount{},
		Options:   req.Options,
	}

	if err := res.SetupProcess(&d); err != nil {
		d.Logger.Warn("Setting up the volume process failed.", "volume", res)
	}

	d.Volumes[req.Name] = res

	if err := exampleDriver_Save(*d.ControlFile, d.Volumes); err != nil {
		return d.Tee(err)
	}

	d.Logger.Debug(fmt.Sprintf("Create() successfully created volume [%s].", req.Name), "res", res)
	return nil
}

func (d exampleDriver) Remove(req *volume.RemoveRequest) error {
	d.Logger.Debug("Remove() has been called.", "req", req)

	if vol, ok := d.Volumes[req.Name]; !ok {
		return d.Tee(fmt.Errorf("volume [%s] could not be found", req.Name))
	} else {
		if l := len(*vol.Mounts); l > 0 {
			return d.Tee(fmt.Errorf("volume [%s] has %d active mounts", req.Name, l))
		}

		d.Mutex.Lock()
		defer d.Mutex.Unlock()

		if processMonitor, ok := processMonitors[vol.Puid]; ok {
			if err := proc.CancelProcess(processMonitor, 5*time.Second); err != nil {
				d.Logger.Warn("Failed terminating volume process.", "err", err)
			}
			delete(processMonitors, vol.Puid)
		}

		if err := os.Remove(vol.MountPoint()); err != nil {
			return d.Tee(err)
		}
		delete(d.Volumes, req.Name)

		if err := exampleDriver_Save(*d.ControlFile, d.Volumes); err != nil {
			return d.Tee(err)
		}
	}

	d.Logger.Debug(fmt.Sprintf("Remove() successfully removed volume [%s].", req.Name))
	return nil
}

func (d exampleDriver) List() (*volume.ListResponse, error) {
	d.Logger.Debug("List() has been called.")

	res := volume.ListResponse{Volumes: []*volume.Volume{}}
	for name, vol := range d.Volumes {
		res.Volumes = append(res.Volumes, &volume.Volume{Name: name, Mountpoint: vol.MountPoint(), CreatedAt: vol.CreatedAt.Format(time.RFC3339)})
	}

	d.Logger.Debug(fmt.Sprintf("List() successfully iterated [%d] volumes.", len(res.Volumes)), "res", res)
	return &res, nil
}

func (d exampleDriver) Capabilities() *volume.CapabilitiesResponse {
	d.Logger.Debug("Capabilities() has been called.")

	res := volume.CapabilitiesResponse{
		Capabilities: volume.Capability{Scope: "local"},
	}

	d.Logger.Debug("Capabilities() successfully gathered the requested information.", "res", res)
	return &res
}

func (d exampleDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	d.Logger.Debug("Mount() has been called.", "req", req)

	if vol, ok := d.Volumes[req.Name]; !ok {
		return nil, d.Tee(fmt.Errorf("volume [%s] could not be found", req.Name))
	} else {
		res := volume.MountResponse{
			Mountpoint: vol.MountPoint(),
		}
		d.Mutex.Lock()
		defer d.Mutex.Unlock()

		mounts := *vol.Mounts
		if mount, ok := mounts[req.ID]; ok {
			mount.ReferenceCount++
			mounts[req.ID] = exampleDriverMount{ReferenceCount: mount.ReferenceCount}

			d.Logger.Debug(fmt.Sprintf("Mount() successfully incremented reference count of the mount for ID [%s] in volume [%s] to %d.", req.ID, req.Name, mount.ReferenceCount))
		} else {
			mounts[req.ID] = exampleDriverMount{ReferenceCount: 1}
			d.Logger.Debug(fmt.Sprintf("Mount() successfully registered a mount for ID [%s] in volume [%s].", req.ID, req.Name), "res", res)
		}

		if err := exampleDriver_Save(*d.ControlFile, d.Volumes); err != nil {
			return nil, d.Tee(err)
		}

		return &res, nil
	}
}

func (d exampleDriver) Unmount(req *volume.UnmountRequest) error {
	d.Logger.Debug("Unmount() has been called.", "req", req)

	if vol, ok := d.Volumes[req.Name]; !ok {
		return d.Tee(fmt.Errorf("volume [%s] could not be found", req.Name))
	} else {
		mounts := *vol.Mounts
		if mount, ok := mounts[req.ID]; !ok {
			d.Logger.Warn(fmt.Sprintf("Unmount() didn't find any mount for ID [%s] in volume [%s].", req.ID, req.Name))
			return nil
		} else {
			d.Mutex.Lock()
			defer d.Mutex.Unlock()

			if mount.ReferenceCount > 0 {
				mount.ReferenceCount--
				mounts[req.ID] = exampleDriverMount{ReferenceCount: mount.ReferenceCount}
			}
			if mount.ReferenceCount > 0 {
				d.Logger.Debug(fmt.Sprintf("Unmount() successfully decremented reference count of the mount for ID [%s] in volume [%s] to %d.", req.ID, req.Name, mount.ReferenceCount))
			} else {
				delete(mounts, req.ID)
				d.Logger.Debug(fmt.Sprintf("Unmount() successfully unregistered the mount for ID [%s] in volume [%s].", req.ID, req.Name))
			}

			if err := exampleDriver_Save(*d.ControlFile, d.Volumes); err != nil {
				return d.Tee(err)
			}
		}

		return nil
	}
}

func (d exampleDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	d.Logger.Debug("Path() has been called.", "req", req)

	if vol, ok := d.Volumes[req.Name]; !ok {
		return nil, d.Tee(fmt.Errorf("volume [%s] could not be found", req.Name))
	} else {
		res := volume.PathResponse{
			Mountpoint: vol.MountPoint(),
		}

		d.Logger.Debug(fmt.Sprintf("Path() successfully looked up volume [%s].", req.Name), "res", res)
		return &res, nil
	}
}
