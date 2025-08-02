package proc

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/thorbenw/example-docker-volume-plugin/semver"
	"gotest.tools/assert"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))

var doInit = func() bool {
	NoInit = true
	return NoInit
}
var noInit = doInit()

func init() {
	_ = noInit
}

func Test_init(t *testing.T) {
	assert.Assert(t, NoInit)

	if err := loadProcStat(); err != nil {
		t.Error(err)
	} else {
		assert.Assert(t, ClockTicksPerSecond == 100)
	}

	ProcPath = "/"
	if err := loadProcStat(); err == nil {
		t.Error("loadProcStat() succeeded unexpectedly.")
	} else {
		logger.Debug("loadProcStat() returned error.", "err", err)
	}

	ProcPath = t.TempDir()
	if file, err := os.CreateTemp(ProcPath, "stat*"); err != nil {
		t.Error(err)
	} else {
		ProcStatName = filepath.Base(file.Name())
		var writeErr error
		_, writeErr = io.WriteString(file, "btime invalid\n")
		_1kbString := strings.Repeat("0123456789abcdef", 2*2*2*2*2*2)
		for i := 0; i < 100; i++ {
			_, writeErr = io.WriteString(file, _1kbString)
		}
		if writeErr != nil {
			t.Error(writeErr)
		} else {
			if err := loadProcStat(); err == nil {
				t.Error("loadProcStat() succeeded unexpectedly.")
			} else {
				logger.Debug("loadProcStat() returned error.", "err", err)
			}
		}
		if err := file.Close(); err != nil {
			t.Error(err)
		}
		ProcStatName = DEFAULT_PROC_STAT_NAME
	}
	ProcPath = DEFAULT_PROC_PATH

	if err := loadUname(); err != nil {
		t.Error(err)
	}

	ProcessStatusNamesContains := func(state ProcessStatus) bool {
		_, ok := ProcessStatusNames[state]

		return ok
	}
	if err := loadProcessStatusNames(nil); err != nil {
		t.Error(err)
	} else {
		assert.Assert(t, len(ProcessStatusNames) == 8)
		assert.Assert(t, ProcessStatusNamesContains(Running))
		assert.Assert(t, ProcessStatusNamesContains(Sleeping))
		assert.Assert(t, ProcessStatusNamesContains(Waiting))
		assert.Assert(t, ProcessStatusNamesContains(Zombie))
		assert.Assert(t, ProcessStatusNamesContains(Stopped))

		assert.Assert(t, ProcessStatusNamesContains(Tracing))
		assert.Assert(t, ProcessStatusNamesContains(Dead))
		assert.Assert(t, ProcessStatusNamesContains(Idle))
	}

	if linux_1, err := semver.Parse("1"); err != nil {
		t.Error(err)
	} else {
		if err := loadProcessStatusNames(linux_1); err != nil {
			t.Error(err)
		} else {
			assert.Assert(t, len(ProcessStatusNames) == 6)
			assert.Assert(t, ProcessStatusNamesContains(Running))
			assert.Assert(t, ProcessStatusNamesContains(Sleeping))
			assert.Assert(t, ProcessStatusNamesContains(Waiting))
			assert.Assert(t, ProcessStatusNamesContains(Zombie))
			assert.Assert(t, ProcessStatusNamesContains(Stopped))

			assert.Assert(t, ProcessStatusNamesContains(Paging))
		}
	}

	if linux_2_6_15, err := semver.Parse("2.6.15"); err != nil {
		t.Error(err)
	} else {
		if err := loadProcessStatusNames(linux_2_6_15); err != nil {
			t.Error(err)
		} else {
			assert.Assert(t, len(ProcessStatusNames) == 6)
			assert.Assert(t, ProcessStatusNamesContains(Running))
			assert.Assert(t, ProcessStatusNamesContains(Sleeping))
			assert.Assert(t, ProcessStatusNamesContains(Waiting))
			assert.Assert(t, ProcessStatusNamesContains(Zombie))
			assert.Assert(t, ProcessStatusNamesContains(Stopped))

			assert.Assert(t, ProcessStatusNamesContains(Dead))
		}
	}

	if linux_3, err := semver.Parse("3"); err != nil {
		t.Error(err)
	} else {
		if err := loadProcessStatusNames(linux_3); err != nil {
			t.Error(err)
		} else {
			assert.Assert(t, len(ProcessStatusNames) == 10)
			assert.Assert(t, ProcessStatusNamesContains(Running))
			assert.Assert(t, ProcessStatusNamesContains(Sleeping))
			assert.Assert(t, ProcessStatusNamesContains(Waiting))
			assert.Assert(t, ProcessStatusNamesContains(Zombie))
			assert.Assert(t, ProcessStatusNamesContains(Stopped))

			assert.Assert(t, ProcessStatusNamesContains(Tracing))
			assert.Assert(t, ProcessStatusNamesContains(Dead))
			assert.Assert(t, ProcessStatusNamesContains(dead))
			assert.Assert(t, ProcessStatusNamesContains(Wakekill))
			assert.Assert(t, ProcessStatusNamesContains(Waking))
		}
	}

	if linux_3_11, err := semver.Parse("3.11"); err != nil {
		t.Error(err)
	} else {
		if err := loadProcessStatusNames(linux_3_11); err != nil {
			t.Error(err)
		} else {
			assert.Assert(t, len(ProcessStatusNames) == 11)
			assert.Assert(t, ProcessStatusNamesContains(Running))
			assert.Assert(t, ProcessStatusNamesContains(Sleeping))
			assert.Assert(t, ProcessStatusNamesContains(Waiting))
			assert.Assert(t, ProcessStatusNamesContains(Zombie))
			assert.Assert(t, ProcessStatusNamesContains(Stopped))

			assert.Assert(t, ProcessStatusNamesContains(Tracing))
			assert.Assert(t, ProcessStatusNamesContains(Dead))
			assert.Assert(t, ProcessStatusNamesContains(dead))
			assert.Assert(t, ProcessStatusNamesContains(Wakekill))
			assert.Assert(t, ProcessStatusNamesContains(Waking))

			assert.Assert(t, ProcessStatusNamesContains(Parked))
		}
	}

	if linux_4, err := semver.Parse("4"); err != nil {
		t.Error(err)
	} else {
		if err := loadProcessStatusNames(linux_4); err != nil {
			t.Error(err)
		} else {
			assert.Assert(t, len(ProcessStatusNames) == 7)
			assert.Assert(t, ProcessStatusNamesContains(Running))
			assert.Assert(t, ProcessStatusNamesContains(Sleeping))
			assert.Assert(t, ProcessStatusNamesContains(Waiting))
			assert.Assert(t, ProcessStatusNamesContains(Zombie))
			assert.Assert(t, ProcessStatusNamesContains(Stopped))

			assert.Assert(t, ProcessStatusNamesContains(Tracing))
			assert.Assert(t, ProcessStatusNamesContains(Dead))
		}
	}
}

func TestGetProcessInfo(t *testing.T) {
	t.Parallel()

	if processInfo, err := GetProcessInfo(0); err == nil {
		t.Errorf("GetProcessInfo(0) succeeded unexpectedly (%v).", processInfo)
	} else {
		logger.Debug("GetProcessInfo(0) returned error.", "err", err)
	}

	pid := os.Getpid()
	if processInfo, err := GetProcessInfo(pid); err != nil {
		t.Error(err)
	} else {
		logger.Debug("GetProcessInfo(pid)", "pid", pid, "processInfo", processInfo)
		uniqueId := GetProcessUniqueId(*processInfo)

		assert.Assert(t, processInfo.UniqueId() == uniqueId)
		assert.Assert(t, ClockTicksPerSecond == 100)

		if _, err := GetProcessInfoFromUniqueId(uniqueId); err != nil {
			t.Error(err)
		}
	}
}

func TestGetProcessInfoWithTimeout(t *testing.T) {
	t.Parallel()

	if processInfo, err := GetProcessInfoWithTimeout(2*time.Second, 1*time.Second, 0); err == nil {
		t.Errorf("GetProcessInfoWithTimeout(2, 1, 0) succeeded unexpectedly (%v).", processInfo)
	} else {
		logger.Debug("GetProcessInfoWithTimeout(2, 1, 0) returned error.", "err", err)
	}

	pid := os.Getpid()
	if processInfo, err := GetProcessInfoWithTimeout(2*time.Second, 1*time.Second, pid); err != nil {
		t.Error(err)
	} else {
		logger.Debug("GetProcessInfoWithTimeout(2, 1, pid)", "pid", pid, "processInfo", processInfo)
	}
}

func TestMonitorProcess(t *testing.T) {
	t.Parallel()

	if Logger == nil {
		Logger = logger
	}

	type cmd struct {
		name string
		argv []string
		attr *os.ProcAttr
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	srcPath := filepath.Join(cwd, "../../test/ignoresignal.c")
	if _, err := os.Stat(srcPath); err != nil {
		t.Fatal(err)
	}

	gccPath, err := exec.LookPath("gcc")
	if err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(t.TempDir(), "ignoresignal")
	if err := exec.Command(gccPath, srcPath, "-o", binPath, "-static").Run(); err != nil {
		t.Fatal(err)
	}

	attr := &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}}
	cmds := []cmd{
		{name: binPath, argv: []string{}, attr: attr},
		{name: binPath, argv: []string{}, attr: attr},
		{name: binPath, argv: []string{"SIGINT"}, attr: attr},
		{name: binPath, argv: []string{}, attr: attr},
	}

	processes := make([]*os.Process, len(cmds))
	for i, cmd := range cmds {
		process, err := os.StartProcess(cmd.name, append([]string{cmd.name}, cmd.argv...), cmd.attr)
		if err != nil {
			t.Fatal(err)
		}
		processes[i] = process
	}

	type args struct {
		process      *os.Process
		recoveryMode RecoveryMode
	}
	tests := []struct {
		name    string
		args    args
		action  func(*testing.T, bool, *ProcessMonitor)
		want    *ProcessMonitor
		wantErr bool
	}{
		// Test cases.
		{name: "Default", args: args{process: processes[0], recoveryMode: RecoveryModeIgnore}, action: func(t *testing.T, wantErr bool, monitor *ProcessMonitor) {
			time.Sleep(2 * time.Second)
			monitor.cancel = true

			if err := monitor.Process.Signal(os.Interrupt); err != nil {
				t.Errorf("Signal() error = %v, wantErr %v", err, wantErr)
				return
			}

			timeout := time.Duration(10)
			ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Second)
			defer cancel()

			select {
			case <-ctx.Done():
				t.Errorf("Process didn't exit within %d seconds timeout.", timeout)
			case err := <-monitor.chError:
				if err != nil {
					t.Errorf("MonitorProcess() chError = %v, wantErr %v", err, wantErr)
				}
			}
		}},
		{name: "Restart", args: args{process: processes[1], recoveryMode: RecoveryModeRestart}, action: func(t *testing.T, wantErr bool, monitor *ProcessMonitor) {
			time.Sleep(2 * time.Second)

			if err := monitor.Process.Signal(os.Interrupt); err != nil {
				t.Errorf("Signal() error = %v, wantErr %v", err, wantErr)
				return
			}

			time.Sleep(2 * time.Second)

			if err := CancelProcess(monitor, 10*time.Second); err != nil {
				t.Errorf("CancelProces() error = %v, wantErr %v", err, wantErr)
				return
			}
		}},
		{name: "Ignore Interrupt", args: args{process: processes[2], recoveryMode: RecoveryModeIgnore}, action: func(t *testing.T, wantErr bool, monitor *ProcessMonitor) {
			time.Sleep(2 * time.Second)

			if err := CancelProcess(monitor, 5*time.Second); err != nil {
				t.Errorf("CancelProces() error = %v, wantErr %v", err, wantErr)
				return
			}
		}},
		{name: "Restart too often", args: args{process: processes[3], recoveryMode: RecoveryModeRestart}, action: func(t *testing.T, wantErr bool, monitor *ProcessMonitor) {
			for i := 0; i < 4; i++ {
				if err := monitor.Process.Signal(os.Interrupt); err != nil {
					t.Errorf("Signal() error = %v, wantErr %v", err, wantErr)
					break
				}
				time.Sleep(2 * time.Second)
			}

			if err := CancelProcess(monitor, 10*time.Second); err == nil {
				t.Errorf("MonitorProcess() unexpectedly didn't give up: chError = %v, wantErr %v", err, wantErr)
			} else {
				logger.Debug("CancelProcess() expectedly failed.", "err", err)
				assert.Assert(t, err.Error() == "monitored process has been restarted 3 times within the last 1m0s: giving up recovery")
			}

		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MonitorProcess(tt.args.process.Pid, tt.args.recoveryMode, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("MonitorProcess() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if (got != nil && tt.want != nil) && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MonitorProcess() = %v, want %v", got, tt.want)
			}

			tt.action(t, tt.wantErr, got)
		})
	}
}
