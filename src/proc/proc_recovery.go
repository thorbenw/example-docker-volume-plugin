//go:build linux

package proc

import "strconv"

// Recovery mode for monitored processes.
//
// See Ignore, Recover and Fail.
type ProcessRecovery int

const (
	// Return from the goroutine and do nothing else.
	Ignore ProcessRecovery = iota
	// Restart the process persistently.
	Recover
	// Panic on unexpected (i.e. uncancelled) process termination.
	Fail
)

var exampleDriverRecoveryNames = map[ProcessRecovery]string{
	Ignore:  "Ignore",
	Recover: "Recover",
	Fail:    "Fail",
}

func (r ProcessRecovery) String() string {
	if v, ok := exampleDriverRecoveryNames[r]; ok {
		return v
	} else {
		return strconv.Itoa(int(r))
	}
}
