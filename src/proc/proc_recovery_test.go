//go:build linux

package proc

import (
	"strconv"
	"strings"
	"testing"

	"gotest.tools/assert"
)

func TestRecoveryMode(t *testing.T) {
	type args struct {
		name                string
		defaultRecoveryMode RecoveryMode
	}
	tests := []struct {
		name       string
		args       args
		want       RecoveryMode
		assertName int // 0 = no, 1 = yes, 2 = yes_trim_space_and_ignore_case
	}{
		// Test cases.
		{name: "Empty", args: args{name: "", defaultRecoveryMode: RecoveryMode(-1)}, want: RecoveryMode(-1)},
		{name: "Mixedcase", args: args{name: "\tpANiC ", defaultRecoveryMode: RecoveryModePanic}, want: RecoveryModePanic, assertName: 2},
		{name: "Unknown", args: args{name: "unknown", defaultRecoveryMode: RecoveryModeRestart}, want: RecoveryModeRestart},
		{name: "Default", args: args{name: "Ignore", defaultRecoveryMode: RecoveryModeRestart}, want: RecoveryModeIgnore, assertName: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RecoveryModeParse(tt.args.name, tt.args.defaultRecoveryMode); got != tt.want {
				t.Errorf("RecoveryModeParse() = %v, want %v", got, tt.want)
			} else {
				if tt.assertName < 1 {
					t.Log("got.String() =", strconv.Quote(got.String()))
				} else {
					got_string := got.String()
					tt_args_name := tt.args.name

					if tt.assertName == 2 {
						got_string = strings.ToLower(strings.TrimSpace(got_string))
						tt_args_name = strings.ToLower(strings.TrimSpace(tt_args_name))
					}

					t.Log("got_string =", got_string)
					t.Log("tt_args_name =", tt_args_name)

					assert.Assert(t, got_string == tt_args_name)
				}
			}
		})
	}
}
