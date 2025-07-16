package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/keebits/example-docker-volume-plugin/proc"
	"github.com/keebits/example-docker-volume-plugin/utils"
)

const (
	DefaultControlFileName  = "volumes.json"
	DefaultControlFileMode  = 0o664
	MinimumVolumeFolderMode = os.ModeDir | 0o700
	DefaultVolumeFolderMode = os.ModeDir | 0o764
)

type exampleDriverRecovery int

const (
	Ignore exampleDriverRecovery = iota
	Recover
	Fail
)

var exampleDriverRecoveryNames = map[exampleDriverRecovery]string{
	Ignore:  "Ignore",
	Recover: "Recover",
	Fail:    "Fail",
}

func (r exampleDriverRecovery) String() string {
	if v, ok := exampleDriverRecoveryNames[r]; ok {
		return v
	} else {
		return strconv.Itoa(int(r))
	}
}

type exampleDriverMount struct {
	ReferenceCount int
}

type exampleDriverVolume struct {
	BasePath    string
	Path        string
	mountPoint  string
	CreatedAt   time.Time
	Mounts      *map[string]exampleDriverMount
	Options     map[string]string
	Puid        string
	process     *os.Process
	processChan chan bool
}

func (v *exampleDriverVolume) MountPoint() string {
	if len(v.mountPoint) < 1 {
		v.mountPoint = filepath.Join(v.BasePath, v.Path)
	}

	return v.mountPoint
}

func (v *exampleDriverVolume) SetupProcess(d *exampleDriver) error {
	if v.process != nil {
		d.Logger.Warn("Volume already has a process.")
	}

	if strings.TrimSpace(v.Puid) == "" && strings.TrimSpace(d.RunBinary) != "" {
		// Create and detach process
		if pid, err := os.StartProcess(d.RunBinary, []string{d.RunBinary, v.MountPoint()}, &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}}); err != nil {
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
				/*
					switch prc.State {
					case proc.Running, proc.Sleeping:
						d.Logger.Debug("Started a new volume process.", "process", prc, "volume", v)
					default:
						d.Logger.Warn("New volume process has an unexpected state.", "process", prc, "volume", v)
					}
				*/
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
				v.process = pid
				v.processChan = make(chan bool, 1)
				go func() {
					d.Logger.Debug("Monitoring volume process.", "volume", v, "pid", pid)

					for {
						ps, err := pid.Wait()
						var cancel bool
						select {
						case cancel = <-v.processChan:
						default:
						}
						if err != nil {
							d.Logger.Warn("Monitoring volume process failed.", "err", err, "volume", v, "pid", pid)
							break
						}
						d.Logger.Debug("Examining volume process.", "volume", v, "pid", pid, "state", ps)
						if ps.Exited() && !cancel {
							//v.SetupProcess(d)
						}
						v.processChan <- true
						break //anyway
					}
				}()
				/*
					if err := pid.Signal(os.Interrupt); err != nil {
						d.Logger.Warn("Signal SIGINT failed.", "err", err, "pid", pid, "prc", prc, "volume", v)
					}
					if pst, err := pid.Wait(); err != nil {
						d.Logger.Warn("Process wait failed.", "err", err, "pid", pid, "prc", prc, "volume", v)
					} else {
						if !pst.Exited() {
							d.Logger.Warn("Volume process still running.", "pst", pst, "pid", pid, "prc", prc, "volume", v)
						} else {
							if pst.ExitCode() != 0 {
								d.Logger.Warn("Volume process returned error code.", "pst", pst, "pid", pid, "prc", prc, "volume", v)
							} else {
								d.Logger.Debug("Volume process terminated.", "pst", pst, "pid", pid, "prc", prc, "volume", v)
							}
						}
					}
				*/
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
	ControlFile           *os.File
	RunBinary             string
	VolumeProcessRecovery exampleDriverRecovery
}

//var volumeProcesses map[string]*os.Process

func exampleDriver_New(propagatedMount string, logger slog.Logger) (*exampleDriver, error) {
	return exampleDriver_NewWithVolumeProcess(propagatedMount, logger, "", Ignore)
}

func exampleDriver_NewWithVolumeProcess(propagatedMount string, logger slog.Logger, runBinary string, recovery exampleDriverRecovery) (*exampleDriver, error) {
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
		Mutex:                 &sync.Mutex{},
		ControlFile:           controlFile,
		RunBinary:             runBinary,
		VolumeProcessRecovery: recovery,
	}

	mountCount := 0
	for name, vol := range volumes {
		//filepath.EvalSymlinks()
		if err := utils.CheckAccess(utils.CheckAccessCurrentUser, os.FileMode(0o7), vol.MountPoint()); err != nil {
			return nil, err
		}
		mountCount += len(*vol.Mounts)

		//var v *exampleDriverVolume = &(volumes[name])->se
		if err := vol.SetupProcess(d); err != nil {
			d.Logger.Warn("Setting up the volume process failed.", "volume", vol)
		} else {
			volumes[name] = vol
		}
		/*
			if strings.TrimSpace(vol.Puid) != "" {
				if prc, err := proc.GetProcessInfoFromUniqueId(vol.Puid); err != nil {
					logger.Warn("PUID is invalid.", "err", err, "volume", vol)
				} else {
					switch prc.State {
					case proc.Running, proc.Sleeping:
					default:
						switch recovery {
						case Ignore:
						case Recover:
						default: //case Fail:
							return nil, fmt.Errorf("volume process is missing")
						}
					}

					if _, err := os.FindProcess(int(prc.Pid)); err != nil {
						logger.Warn("PID is invalid.", "err", err, "volume", vol, "prc", prc)
					}

					if pid, err := os.FindProcess(int(prc.Pid)); err != nil {
						logger.Warn("PID is invalid.", "err", err, "volume", vol, "prc", prc)
					} else {
						if err := pid.Signal(os.Interrupt); err != nil {
							logger.Warn("Signal SIGINT failed.", "err", err, "pid", pid, "prc", prc, "volume", vol)
						}
						if pst, err := pid.Wait(); err != nil {
							logger.Warn("Process wait failed.", "err", err, "pid", pid, "prc", prc, "volume", vol)
						} else {
							if !pst.Exited() {
								logger.Warn("Volume process still running.", "pst", pst, "pid", pid, "prc", prc, "volume", vol)
							} else {
								if pst.ExitCode() != 0 {
									logger.Warn("Volume process returned error code.", "pst", pst, "pid", pid, "prc", prc, "volume", vol)
								} else {
									logger.Debug("Volume process terminated.", "pst", pst, "pid", pid, "prc", prc, "volume", vol)
								}
							}
						}
					}

				}
			}
		*/
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
		//processChan: make(chan bool, 2),
	}

	if err := res.SetupProcess(&d); err != nil {
		d.Logger.Warn("Setting up the volume process failed.", "volume", res)
	}
	/*

		if strings.TrimSpace(d.RunBinary) != "" {
			if pid, err := os.StartProcess(d.RunBinary, []string{d.RunBinary, volumePathAbs}, &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}}); err != nil {
				return d.Tee(err)
			} else {
				if prc, err := proc.GetProcessInfo(pid.Pid); err != nil {
					return d.Tee(err)
				} else {
					res.Puid = prc.UniqueId()

					switch prc.State {
					case proc.Running, proc.Sleeping:
						d.Logger.Debug("Started a new volume process.", "process", prc, "volume", res)
					default:
						d.Logger.Warn("New volume process has an unexpected state.", "process", prc, "volume", res)
					}
				}
			}
		}
	*/
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

		if vol.process != nil {
			//vol.Puid = ""
			//d.Volumes[req.Name] = vol
			vol.processChan <- true
			if err := vol.process.Signal(os.Interrupt); err != nil {
				d.Logger.Warn("Signalling failed.")
			} else {
				d.Logger.Debug("Signalled volume process.")
				<-vol.processChan
			}
		}
		/*
			if strings.TrimSpace(vol.Puid) != "" {
				if prc, err := proc.GetProcessInfoFromUniqueId(vol.Puid); err != nil {
					d.Logger.Warn("PUID is invalid.", "err", err, "volume", vol)
				} else {
					if pid, err := os.FindProcess(int(prc.Pid)); err != nil {
						d.Logger.Warn("PID is invalid.", "err", err, "volume", vol, "prc", prc)
					} else {
						if err := pid.Signal(os.Interrupt); err != nil {
							d.Logger.Warn("Signal SIGINT failed.", "err", err, "pid", pid, "prc", prc, "volume", vol)
						}
						if pst, err := pid.Wait(); err != nil {
							d.Logger.Warn("Process wait failed.", "err", err, "pid", pid, "prc", prc, "volume", vol)
						} else {
							if !pst.Exited() {
								d.Logger.Warn("Volume process still running.", "pst", pst, "pid", pid, "prc", prc, "volume", vol)
							} else {
								if pst.ExitCode() != 0 {
									d.Logger.Warn("Volume process returned error code.", "pst", pst, "pid", pid, "prc", prc, "volume", vol)
								} else {
									d.Logger.Debug("Volume process terminated.", "pst", pst, "pid", pid, "prc", prc, "volume", vol)
								}
							}
						}
					}
				}
			}
		*/

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
		res.Volumes = append(res.Volumes, &volume.Volume{Name: name, Mountpoint: vol.MountPoint(), CreatedAt: vol.CreatedAt.String()})
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
