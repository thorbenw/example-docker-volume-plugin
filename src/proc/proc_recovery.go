//go:build linux

package proc

import (
	"strconv"
	"strings"
)

// Recovery mode for monitored processes.
//
// See Ignore, Recover and Fail.
type RecoveryMode int

const (
	// Return from the goroutine and do nothing else.
	RecoveryModeIgnore RecoveryMode = iota
	// Restart the process persistently.
	RecoveryModeRestart
	// Panic on unexpected (i.e. uncancelled) process termination.
	RecoveryModePanic
)

var recoveryModeNames = map[RecoveryMode]string{
	RecoveryModeIgnore:  "Ignore",
	RecoveryModeRestart: "Restart",
	RecoveryModePanic:   "Panic",
}

func RecoveryModeNames() map[RecoveryMode]string {
	return recoveryModeNames
}

func (r RecoveryMode) String() string {
	if v, ok := recoveryModeNames[r]; ok {
		return v
	} else {
		return strconv.Itoa(int(r))
	}
}

func RecoveryModeParse(name string, defaultRecoveryMode RecoveryMode) RecoveryMode {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultRecoveryMode
	}

	name = strings.ToLower(name)
	for k, v := range recoveryModeNames {
		if name == strings.ToLower(v) {
			return k
		}
	}

	return defaultRecoveryMode
}
