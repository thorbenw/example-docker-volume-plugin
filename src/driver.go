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
	"github.com/thorbenw/example-docker-volume-plugin/metric"
	"github.com/thorbenw/example-docker-volume-plugin/mount"
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

type GetVolumeProcess func() (*exec.Cmd, *proc.Options, *mount.Options)
type SetVolumeProcessOptions func(*exec.Cmd, *proc.Options, *mount.Options, string) error

type pluginDriverMount struct {
	ReferenceCount int
}

type pluginDriverVolume struct {
	BasePath   string
	Path       string
	mountPoint string
	CreatedAt  time.Time
	Mounts     *map[string]pluginDriverMount
	Options    *map[string]string
	Puid       string
}

func (v *pluginDriverVolume) MountPoint() string {
	if len(v.mountPoint) < 1 {
		v.mountPoint = filepath.Join(v.BasePath, v.Path)
	}

	return v.mountPoint
}

func (v *pluginDriverVolume) SetupProcess(d *pluginDriver) error {
	if strings.TrimSpace(v.Puid) == "" && d.GetVolumeProcess != nil {
		// Create and detach process

		cmd, volumeProcessOptions, mountOptions := (*d).GetVolumeProcess()
		d.Logger.Debug("Got volume process.", "cmd", fmt.Sprintf("%#v", cmd), "volumeProcessOptions", proc.OptionsString(volumeProcessOptions, true), "mountOptions", mount.OptionsString(mountOptions, true))
		if cmd.Cancel != nil || cmd.WaitDelay != 0 {
			return d.Tee(fmt.Errorf("command must not use a context"))
		}
		if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
			return d.Tee(fmt.Errorf("command must not use custom standard files"))
		}
		if cmd.Process != nil || cmd.ProcessState != nil {
			return d.Tee(fmt.Errorf("command has already been started or run"))
		}

		var len_volumeOptions int
		if v.Options != nil {
			len_volumeOptions = len(*v.Options)
		}
		if len_volumeOptions > 0 {
			if volumeProcessOptionsOption, ok := (*v.Options)["c"]; ok {
				if volumeProcessOptions == nil {
					mnt := proc.NewOptions(len_volumeOptions, VOLUME_PROCESS_OPTIONS_SEPARATOR, true)
					volumeProcessOptions = &mnt
				}

				if err := volumeProcessOptions.Set(volumeProcessOptionsOption); err != nil {
					return d.Tee(err)
				}
			}

			if mountOptionsOption, ok := (*v.Options)["o"]; ok {
				if mountOptions == nil {
					mnt := mount.NewOptions(len_volumeOptions)
					mountOptions = &mnt
				}

				if err := mountOptions.Set(mountOptionsOption); err != nil {
					return d.Tee(err)
				}
			}
		}

		d.Logger.Debug("Processed options.", "volumeProcessOptions", proc.OptionsString(volumeProcessOptions, true), "mountOptions", mount.OptionsString(mountOptions, true))
		var len_options int
		if volumeProcessOptions != nil {
			len_options += volumeProcessOptions.Len()
		}
		if mountOptions != nil {
			len_options += mountOptions.Len()
		}
		if len_options > 0 {
			if d.SetVolumeProcessOptions == nil {
				return d.Tee(fmt.Errorf("there are %d options present, but processing function is missing", len_options))
			}

			if err := (*d).SetVolumeProcessOptions(cmd, volumeProcessOptions, mountOptions, v.MountPoint()); err != nil {
				return d.Tee(err)
			}
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

			if prc, err := proc.GetProcessInfoWithTimeout(5*time.Second, 1*time.Second, wpid); err != nil {
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
					if processMonitor, err := proc.MonitorProcess(pid.Pid, d.VolumeProcessRecoveryMode, d.VolumeProcessRecoveryRateLimit); err != nil {
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

type pluginDriver struct {
	PropagatedMount string
	slog.Logger
	Volumes map[string]pluginDriverVolume
	*sync.Mutex
	ControlFile *os.File
	GetVolumeProcess
	SetVolumeProcessOptions
	VolumeProcessRecoveryMode      proc.RecoveryMode
	VolumeProcessRecoveryRateLimit *metric.MetricRateLimit
}

func pluginDriver_New(propagatedMount string, logger slog.Logger) (*pluginDriver, error) {
	return pluginDriver_NewWithVolumeProcess(propagatedMount, logger, nil, nil, proc.RecoveryModeIgnore, nil)
}

func pluginDriver_NewWithVolumeProcess(propagatedMount string, logger slog.Logger, getVolumeProcess GetVolumeProcess, setVolumeProcessOptions SetVolumeProcessOptions, recoveryMode proc.RecoveryMode, recoveryRateLimit *metric.MetricRateLimit) (*pluginDriver, error) {
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

	volumes, err := pluginDriver_Load(*controlFile)
	if err != nil {
		controlFile.Close()
		return nil, err
	}

	d := &pluginDriver{
		PropagatedMount: propagatedMount,
		Logger:          logger,
		//Volumes:               volumes,
		Mutex:                          &sync.Mutex{},
		ControlFile:                    controlFile,
		GetVolumeProcess:               getVolumeProcess,
		SetVolumeProcessOptions:        setVolumeProcessOptions,
		VolumeProcessRecoveryMode:      recoveryMode,
		VolumeProcessRecoveryRateLimit: recoveryRateLimit,
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

func pluginDriver_Load(file os.File) (data map[string]pluginDriverVolume, err error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	bytes, err := io.ReadAll(&file)
	if err != nil {
		return nil, err
	}

	if len(bytes) < 1 {
		return map[string]pluginDriverVolume{}, nil
	}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, err
	}

	return
}

func pluginDriver_Save(file os.File, data map[string]pluginDriverVolume) error {
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

func (d pluginDriver) Tee(err error, args ...any) error {
	args = append([]any{"err", fmt.Sprintf("%#v", err)}, args...)

	d.Logger.Error(err.Error(), args...)

	return err
}

func (d pluginDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	d.Logger.Debug("Get() has been called.", "req", req)

	if vol, ok := d.Volumes[req.Name]; !ok {
		return nil, d.Tee(fmt.Errorf("volume [%s] could not be found", req.Name))
	} else {
		res := volume.GetResponse{
			Volume: &volume.Volume{
				Name:       req.Name,
				Mountpoint: vol.MountPoint(),
				CreatedAt:  vol.CreatedAt.Format(time.RFC3339),
				//Status: map[string]slog.Attr{"key":"value"},
			},
		}

		d.Logger.Debug(fmt.Sprintf("Get() successfully looked up volume [%s].", req.Name), "res", res)
		return &res, nil
	}
}

func (d pluginDriver) Create(req *volume.CreateRequest) error {
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

	res := pluginDriverVolume{
		BasePath:  d.PropagatedMount,
		Path:      volumePathRel,
		CreatedAt: time.Now(),
		Mounts:    &map[string]pluginDriverMount{},
		Options:   &req.Options,
	}

	if err := res.SetupProcess(&d); err != nil {
		d.Logger.Warn("Setting up the volume process failed.", "volume", res)
	}

	d.Volumes[req.Name] = res

	if err := pluginDriver_Save(*d.ControlFile, d.Volumes); err != nil {
		return d.Tee(err)
	}

	d.Logger.Debug(fmt.Sprintf("Create() successfully created volume [%s].", req.Name), "res", res)
	return nil
}

func (d pluginDriver) Remove(req *volume.RemoveRequest) error {
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
			if err := proc.CancelProcess(processMonitor, 2*time.Second); err != nil {
				d.Logger.Warn("Failed terminating volume process.", "err", err)
			}
			delete(processMonitors, vol.Puid)
		}

		if err := os.Remove(vol.MountPoint()); err != nil {
			return d.Tee(err)
		}
		delete(d.Volumes, req.Name)

		if err := pluginDriver_Save(*d.ControlFile, d.Volumes); err != nil {
			return d.Tee(err)
		}
	}

	d.Logger.Debug(fmt.Sprintf("Remove() successfully removed volume [%s].", req.Name))
	return nil
}

func (d pluginDriver) List() (*volume.ListResponse, error) {
	d.Logger.Debug("List() has been called.")

	res := volume.ListResponse{Volumes: []*volume.Volume{}}
	for name, vol := range d.Volumes {
		res.Volumes = append(res.Volumes, &volume.Volume{Name: name, Mountpoint: vol.MountPoint(), CreatedAt: vol.CreatedAt.Format(time.RFC3339)})
	}

	d.Logger.Debug(fmt.Sprintf("List() successfully iterated [%d] volumes.", len(res.Volumes)), "res", res)
	return &res, nil
}

func (d pluginDriver) Capabilities() *volume.CapabilitiesResponse {
	d.Logger.Debug("Capabilities() has been called.")

	res := volume.CapabilitiesResponse{
		Capabilities: volume.Capability{Scope: "local"},
	}

	d.Logger.Debug("Capabilities() successfully gathered the requested information.", "res", res)
	return &res
}

func (d pluginDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
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
			mounts[req.ID] = pluginDriverMount{ReferenceCount: mount.ReferenceCount}

			d.Logger.Debug(fmt.Sprintf("Mount() successfully incremented reference count of the mount for ID [%s] in volume [%s] to %d.", req.ID, req.Name, mount.ReferenceCount))
		} else {
			mounts[req.ID] = pluginDriverMount{ReferenceCount: 1}
			d.Logger.Debug(fmt.Sprintf("Mount() successfully registered a mount for ID [%s] in volume [%s].", req.ID, req.Name), "res", res)
		}

		if err := pluginDriver_Save(*d.ControlFile, d.Volumes); err != nil {
			return nil, d.Tee(err)
		}

		return &res, nil
	}
}

func (d pluginDriver) Unmount(req *volume.UnmountRequest) error {
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
				mounts[req.ID] = pluginDriverMount{ReferenceCount: mount.ReferenceCount}
			}
			if mount.ReferenceCount > 0 {
				d.Logger.Debug(fmt.Sprintf("Unmount() successfully decremented reference count of the mount for ID [%s] in volume [%s] to %d.", req.ID, req.Name, mount.ReferenceCount))
			} else {
				delete(mounts, req.ID)
				d.Logger.Debug(fmt.Sprintf("Unmount() successfully unregistered the mount for ID [%s] in volume [%s].", req.ID, req.Name))
			}

			if err := pluginDriver_Save(*d.ControlFile, d.Volumes); err != nil {
				return d.Tee(err)
			}
		}

		return nil
	}
}

func (d pluginDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
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
