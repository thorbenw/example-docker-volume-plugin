//go:build linux

package proc

// #cgo LDFLAGS: -l:libc.a
// #include <unistd.h>
//import "C"

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/keebits/example-docker-volume-plugin/semver"
	"github.com/keebits/example-docker-volume-plugin/utils"
)

// region: noCopy struct

// noCopy may be embedded into structs which must not be copied after the first
// use.
//
// See https://stackoverflow.com/questions/68183168/how-to-force-compiler-error-if-struct-shallow-copy
// for details.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

//endregion

// region: ProcessStatus enum
type ProcessStatus string

const (
	Running  ProcessStatus = "R"
	Sleeping ProcessStatus = "S"
	Waiting  ProcessStatus = "D"
	Zombie   ProcessStatus = "Z"
	Stopped  ProcessStatus = "T"
	Tracing  ProcessStatus = "t"
	Paging   ProcessStatus = "W"
	Dead     ProcessStatus = "X"
	dead     ProcessStatus = "x"
	Wakekill ProcessStatus = "K"
	Waking   ProcessStatus = "W"
	Parked   ProcessStatus = "P"
	Idle     ProcessStatus = "I"
)

// Is populated by calling loadProcessStatusNames(), usually when calling Reset()
// or during init().
var ProcessStatusNames = map[ProcessStatus]string{}

func (ps ProcessStatus) String() string {
	if processStatusName, ok := ProcessStatusNames[ps]; ok {
		return processStatusName
	}

	return string(ps)
}

//endregion

// region: ProcessInfo struct and interface
type IProcessInfo interface {
	UniqueId() string
}

type ProcessInfo struct {
	Pid       uint64
	Comm      string
	State     ProcessStatus
	StartTime time.Time
	Cmdline   []string
}

func (pi ProcessInfo) UniqueId() (uniqueId string) {
	bytes_pid := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes_pid, pi.Pid)

	bytes_stt := make([]byte, 8)
	unix_nano := uint64(pi.StartTime.UnixNano())
	binary.BigEndian.PutUint64(bytes_stt, unix_nano)

	uniqueId = fmt.Sprintf("%x", slices.Concat(bytes_pid, bytes_stt))

	return
}

//endregion

const (
	DEFAULT_PROC_PATH                  = "/proc"
	DEFAULT_PROC_STAT_NAME             = "stat"
	DEFAULT_PROC_CMDLINE_NAME          = "cmdline"
	TASK_COMM_LEN                      = 16
	MIN_CANCEL_PROCESS_TIMEOUT_SECONDS = time.Duration(1)
	MIN_KILL___PROCESS_TIMEOUT_SECONDS = time.Duration(1)
)

var (
	// Boot time, in seconds since the Epoch, 1970-01-01T00:00:00+0000 (UTC).
	//
	// See https://man7.org/linux/man-pages/man5/proc_stat.5.html
	BTime time.Time
	// The number of clock ticks per second the kernel is configured to.
	//
	// See https://stackoverflow.com/questions/19919881/sysconf-sc-clk-tck-what-does-it-return
	ClockTicksPerSecond = 100
	// Tells if init() for this package (proc) has been run.
	//
	// If init() has been skipped, NoInit is set to true.
	NoInit = false
	// A logger to be used throughout all functions of the proc package.
	//
	// If not otherwise set, this is the default logger (slog.Logger.Default()).
	// However, once set manually, Logger won't become the default logger again
	// when callig Reset().
	Logger *slog.Logger
	// Ther current operating system release as returned by uname.
	Release *semver.VersionInfo
	// Root path for all subsequent procfs based operations.
	ProcPath = DEFAULT_PROC_PATH
	// Name of `stat` file(s) for all subsequent procfs based operations.
	ProcStatName = DEFAULT_PROC_STAT_NAME
	// Name of `cmdline` file(s) for all subsequent procfs based operations.
	ProcCmdlineName = DEFAULT_PROC_CMDLINE_NAME
	// Unsigned(!) integer fields in /proc/<pid>/stat .
	//
	// See https://man7.org/linux/man-pages/man5/proc_pid_stat.5.html
	procPidStatsUInt = map[string]int{
		"pid":       1,
		"starttime": 22,
	}
	// String fields in /proc/<pid>/stat .
	//
	// See https://man7.org/linux/man-pages/man5/proc_pid_stat.5.html
	procPidStatsString = map[string]int{
		"comm":  2,
		"state": 3,
	}
)

func init() {
	if !NoInit {
		if err := Reset(); err != nil {
			panic(err)
		}
	}
}

func loadProcStat() (fail error) {
	path := filepath.Join(ProcPath, ProcStatName)
	file, err := os.OpenFile(path, os.O_RDONLY, os.FileMode(0))
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); (err != nil) && (fail == nil) {
			fail = err
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if a, t, e := bufio.ScanWords(line, false); e == nil {
			key := string(t)
			switch key {
			case "btime":
				if btime, err := strconv.ParseInt(string(line[a:]), 10, 64); err != nil {
					fail = err
				} else {
					local := time.FixedZone("Local", 2*60*60)
					BTime = time.Unix(btime, 0).In(local)
				}
			default:
				continue
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return
}

func loadUname() error {
	var uname syscall.Utsname
	var err error = syscall.Uname(&uname)
	if err != nil {
		return err
	}

	Release, err = semver.Parse(utils.Int8ToStr(uname.Release[:]))
	if err != nil {
		return err
	}

	/*
	 	logger.Debug(utils.Int8ToStr(uname.Domainname[:]))
	   	logger.Debug(utils.Int8ToStr(uname.Machine[:]))
	   	logger.Debug(utils.Int8ToStr(uname.Nodename[:]))
	   	logger.Debug(utils.Int8ToStr(uname.Release[:]))
	   	logger.Debug(utils.Int8ToStr(uname.Sysname[:]))
	   	logger.Debug(utils.Int8ToStr(uname.Version[:]))
	*/

	return nil
}

func loadProcessStatusNames(release *semver.VersionInfo) error {
	if nil == release {
		release = Release
	}

	linux_2_6_0, err := semver.Parse("2.6.0")
	if err != nil {
		return err
	}

	linux_2_6_33, err := semver.Parse("2.6.33")
	if err != nil {
		return err
	}

	linux_3_9, err := semver.Parse("3.9")
	if err != nil {
		return err
	}

	linux_3_13, err := semver.Parse("3.13")
	if err != nil {
		return err
	}

	linux_4_14, err := semver.Parse("4.14")
	if err != nil {
		return err
	}

	type addState struct {
		state ProcessStatus
		text  string
	}
	addNew := func(add ...addState) error {
		for _, ad := range add {
			if _, ok := ProcessStatusNames[ad.state]; ok {
				return fmt.Errorf("process status %s already exists in ProcessStatusNames", ad.state)
			}

			ProcessStatusNames[ad.state] = ad.text
		}
		return nil
	}

	ProcessStatusNames = make(map[ProcessStatus]string, 13)

	if err := addNew(
		addState{Running, "Running"},
		addState{Sleeping, "Sleeping in an interruptible wait"},
		addState{Waiting, "Waiting in uninterruptible disk sleep"},
		addState{Zombie, "Zombie"},
		addState{Stopped, "Stopped (on a signal) or (before Linux 2.6.33) trace stopped"},
	); err != nil {
		return err
	}

	if semver.Compare(*release, *linux_2_6_33) >= 0 {
		if err := addNew(
			addState{Tracing, "Tracing stop (Linux 2.6.33 onward)"},
		); err != nil {
			return err
		}
	}
	if semver.Compare(*release, *linux_2_6_0) < 0 {
		if err := addNew(
			addState{Paging, "Paging (only before Linux 2.6.0)"},
		); err != nil {
			return err
		}
	}
	if semver.Compare(*release, *linux_2_6_0) >= 0 {
		if err := addNew(
			addState{Dead, "Dead (from Linux 2.6.0 onward)"},
		); err != nil {
			return err
		}
	}
	if semver.Compare(*release, *linux_2_6_33) >= 0 && semver.Compare(*release, *linux_3_13) <= 0 {
		if err := addNew(
			addState{dead, "Dead (Linux 2.6.33 to 3.13 only)"},
			addState{Wakekill, "Wakekill (Linux 2.6.33 to 3.13 only)"},
			addState{Waking, "Waking (Linux 2.6.33 to 3.13 only)"},
		); err != nil {
			return err
		}
	}
	if semver.Compare(*release, *linux_3_9) >= 0 && semver.Compare(*release, *linux_3_13) <= 0 {
		if err := addNew(
			addState{Parked, "Parked (Linux 3.9 to 3.13 only)"},
		); err != nil {
			return err
		}
	}
	if semver.Compare(*release, *linux_4_14) >= 0 {
		if err := addNew(
			addState{Idle, "Idle (Linux 4.14 onward)"},
		); err != nil {
			return err
		}
	}
	/*
		ProcessStatusNames[] =

		Running:  "Running",
		Sleeping: "Sleeping in an interruptible wait",
		Waiting:  "Waiting in uninterruptible disk sleep",
		Zombie:   "Zombie",
		Stopped:  "Stopped (on a signal) or (before Linux 2.6.33) trace stopped",
		Tracing:  "Tracing stop (Linux 2.6.33 onward)",
		Paging:   "Paging (only before Linux 2.6.0)",
		Dead:     "Dead (from Linux 2.6.0 onward)",
		dead:     "dead (Linux 2.6.33 to 3.13 only)",
		Wakekill: "Wakekill (Linux 2.6.33 to 3.13 only)",
		Waking:   "Waking (Linux 2.6.33 to 3.13 only)",
		Parked: "Parked (Linux 3.9 to 3.13 only)",
		Idle:   "Idle (Linux 4.14 onward)",
	*/
	return nil
}

func Reset() error {
	if Logger == nil {
		Logger = slog.Default()
	}

	if err := loadProcStat(); err != nil {
		return err
	}

	if err := loadUname(); err != nil {
		return err
	}

	if err := loadProcessStatusNames(nil); err != nil {
		return err
	}

	return nil
}

func GetProcessInfo(pid int) (*ProcessInfo, error) {
	path := filepath.Join(ProcPath, strconv.Itoa(pid))

	if fileInfo, err := os.Lstat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("GetProcessInfo %d: no such PID", pid)
		} else {
			return nil, err
		}
	} else {
		if !fileInfo.IsDir() {
			return nil, fmt.Errorf("GetProcessInfo %d: path [%s] is not a directory", pid, path)
		}
	}

	var (
		fileStat    = filepath.Join(path, ProcStatName)
		fileCmdline = filepath.Join(path, ProcCmdlineName)
	)

	errLn := 2
	errCh := make(chan error, errLn)

	stat := make(map[string]any, 100)
	getStat := func(errCh chan<- error) {
		file, err := os.OpenFile(fileStat, os.O_RDONLY, os.FileMode(0))
		if err != nil {
			errCh <- err
			return
		}
		defer func() {
			errCh <- file.Close()
		}()

		fields := make([]string, 0, 100)
		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanWords)
		for scanner.Scan() {
			fields = append(fields, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			errCh <- err
		}

		parseUintField := func(fieldName string, fieldNum int) (val uint64, fail error) {
			val, err := strconv.ParseUint(fields[fieldNum-1], 10, 64)
			if err != nil {
				fail = fmt.Errorf("failed parsing field %d (%s): %s", fieldNum, fieldName, err.Error())
			}

			return
		}

		for k, v := range procPidStatsUInt {
			val, err := parseUintField(k, v)
			if err != nil {
				errCh <- err
			} else {
				stat[k] = val
			}
		}

		for k, v := range procPidStatsString {
			stat[k] = fields[v-1]
		}
	}

	cmdline := make([]string, 0, 1)
	getCmdline := func(errCh chan<- error) {
		file, err := os.OpenFile(fileCmdline, os.O_RDONLY, os.FileMode(0))
		if err != nil {
			errCh <- err
			return
		}
		defer func() {
			errCh <- file.Close()
		}()

		scanner := bufio.NewScanner(file)
		scanner.Split(utils.ScanStrings)
		for scanner.Scan() {
			cmdline = append(cmdline, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}

	go getStat(errCh)
	go getCmdline(errCh)

	for errNo := 0; errNo < errLn; errNo++ {
		err := <-errCh
		if err != nil {
			return nil, err
		}
	}

	processInfo := &ProcessInfo{
		Pid:       stat["pid"].(uint64),
		Comm:      strings.Trim(stat["comm"].(string), "()"),
		State:     ProcessStatus(stat["state"].(string)),
		StartTime: BTime.Add(time.Duration((float64(stat["starttime"].(uint64)) / float64(ClockTicksPerSecond)) * float64(time.Second))),
		Cmdline:   cmdline,
	}

	if processInfo.Pid != uint64(pid) {
		return nil, fmt.Errorf("unexpected process id %d found in %s", processInfo.Pid, fileStat)
	}

	if lenCmdline := len(processInfo.Cmdline); lenCmdline < 1 {
		return nil, fmt.Errorf("unexpected argument count %d < 1 for process id %d", lenCmdline, processInfo.Pid)
	}

	return processInfo, nil
}

func GetProcessInfoFromUniqueId(uniqueId string) (processInfo *ProcessInfo, fail error) {
	bytes, err := utils.Atob(uniqueId)
	if err != nil {
		return nil, err
	}

	pid := int(binary.BigEndian.Uint64(bytes))

	processInfo, err = GetProcessInfo(pid)
	if err != nil {
		return nil, err
	}

	bytes = bytes[8:]
	stt := int64(binary.BigEndian.Uint64(bytes))
	if stt != processInfo.StartTime.UnixNano() {
		return nil, fmt.Errorf("new process with same PID (%d) has different start time", processInfo.Pid)
	}

	return
}

func GetProcessUniqueId(processInfo IProcessInfo) string {
	return processInfo.UniqueId()
}

type ProcessMonitor struct {
	noCopy       noCopy
	cancel       bool
	chError      chan error
	Process      *os.Process
	ProcessInfo  *ProcessInfo
	RecoveryMode RecoveryMode
}

// Starts a goroutine that keeps track of the processes status. The [recovery]
// parameter controls what happend if the process terminates (i.e. either exits
// normally or is killed).
//
// The returned ProcessMonitor object is meant to be used in calls to
// CancelProcess() and KillProcess().
func MonitorProcess(pid int, recoveryMode RecoveryMode) (*ProcessMonitor, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	processInfo, err := GetProcessInfo(process.Pid)
	if err != nil {
		return nil, err
	} else if _, err := os.Stat(processInfo.Cmdline[0]); os.IsNotExist(err) {
		return nil, err
	}

	var monitor = ProcessMonitor{
		chError:      make(chan error, 1),
		Process:      process,
		ProcessInfo:  processInfo,
		RecoveryMode: recoveryMode,
	}

	go func(monitor *ProcessMonitor) {
		for {
			processState, err := monitor.Process.Wait()
			if err != nil {
				monitor.chError <- err
				break
			}
			Logger.Debug(processState.String(), "processName", processInfo.Cmdline[0], "processState", fmt.Sprintf("%#v", processState))

			if monitor.cancel || monitor.RecoveryMode == RecoveryModeIgnore {
				monitor.chError <- err // may be even nil
				break
			} else if monitor.RecoveryMode == RecoveryModePanic {
				panic(errors.New(processState.String()))
			}

			process, err := os.StartProcess(processInfo.Cmdline[0], processInfo.Cmdline, &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
			if err != nil {
				monitor.chError <- err
				break
			}

			processInfo, err := GetProcessInfo(process.Pid)
			if err != nil {
				monitor.chError <- err
				break
			}

			monitor.Process = process
			monitor.ProcessInfo = processInfo

			Logger.Debug("restarted", "processName", processInfo.Cmdline[0], "process", process)
		}
	}(&monitor)

	return &monitor, nil
}

// Sends a SIGINT signal to processMonitor.Process and waits [timeout] for the
// process to terminate.
//
// If the process doesn't terminate before [timeout], KillProcess() is called
// with the same timeout and it's result being passed through.
func CancelProcess(processMonitor *ProcessMonitor, timeout time.Duration) error {
	if timeout < (MIN_CANCEL_PROCESS_TIMEOUT_SECONDS * time.Second) {
		return fmt.Errorf("cancel process: timeout must be at least %d second(s)", MIN_CANCEL_PROCESS_TIMEOUT_SECONDS)
	}

	processMonitor.cancel = true

	if err := processMonitor.Process.Signal(os.Interrupt); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		Logger.Debug("cancel process: timeout elapsed, killing process", "timeout", timeout, "process", processMonitor.Process)
		return KillProcess(processMonitor, timeout)
	case err := <-processMonitor.chError:
		Logger.Debug("cancel process: cancelled process", "err", err, "process", processMonitor.Process)
		return err
	}
}

// Sends a SIGKILL signal to processMonitor.Process and waits [timeout] for the
// process to terminate.
//
// If the process doesn't terminate before [timeout], an according error is
// returned.
func KillProcess(processMonitor *ProcessMonitor, timeout time.Duration) error {
	if timeout < (MIN_KILL___PROCESS_TIMEOUT_SECONDS * time.Second) {
		return fmt.Errorf("kill process: timeout must be at least %d second(s)", MIN_KILL___PROCESS_TIMEOUT_SECONDS)
	}

	processMonitor.cancel = true

	if err := processMonitor.Process.Kill(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return fmt.Errorf("kill process: %d timeout elapsed", timeout)
	case err := <-processMonitor.chError:
		Logger.Debug("kill process: killed process", "err", err, "process", processMonitor.Process)
		return err
	}
}
