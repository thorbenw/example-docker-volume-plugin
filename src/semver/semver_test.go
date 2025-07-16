package semver

import (
	"log/slog"
	"os"
	"reflect"
	"testing"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))

func TestParse(t *testing.T) {
	type args struct {
		v string
	}
	tests := []struct {
		name    string
		args    args
		want    *VersionInfo
		wantErr bool
	}{
		// Test cases.
		{name: "Valid", args: args{v: "1.2.3-pre.4+build.5"}, want: &VersionInfo{Major: 1, Minor: 2, Patch: 3, PreRelease: []any{"pre", uint64(4)}, Build: []any{"build", uint64(5)}}, wantErr: false},
		{name: "Invalid major version", args: args{v: "a.2.3-pre.4+build.5"}, want: nil, wantErr: true},
		{name: "Invalid major version", args: args{v: "1.2.3.4-pre.4+build.5"}, want: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args.v)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				} else {
					logger.Debug("Parse() failed expectedly.", "got", got, "err", err)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			} else {
				logger.Debug("Parse() succeeded.", "got", got)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	type args struct {
		a VersionInfo
		b VersionInfo
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		// Test cases.
		{name: "a < b Major", args: args{a: VersionInfo{Major: 1, Minor: 2, Patch: 3}, b: VersionInfo{Major: 2, Minor: 3, Patch: 4}}, want: -1},
		{name: "a > b Minor", args: args{a: VersionInfo{Major: 1, Minor: 2, Patch: 3}, b: VersionInfo{Major: 1, Minor: 1, Patch: 4}}, want: 1},
		{name: "a > b Patch", args: args{a: VersionInfo{Major: 1, Minor: 2, Patch: 3}, b: VersionInfo{Major: 1, Minor: 2, Patch: 2}}, want: 1},
		{name: "a == b", args: args{a: VersionInfo{Major: 1, Minor: 2, Patch: 3, Build: []any{4}}, b: VersionInfo{Major: 1, Minor: 2, Patch: 3, Build: []any{5}}}, want: 0},
		{name: "a < b Prerelease", args: args{a: VersionInfo{Major: 1, Minor: 2, Patch: 3, PreRelease: []any{"alpha"}}, b: VersionInfo{Major: 1, Minor: 2, Patch: 3}}, want: -1},
		{name: "a < b Prerelease different length", args: args{a: VersionInfo{PreRelease: []any{"a", "b", 0}}, b: VersionInfo{PreRelease: []any{"a", "b", 0, uint(0), int16(-1), true}}}, want: -1},
		{name: "a < b Prerelease string string", args: args{a: VersionInfo{PreRelease: []any{"a"}}, b: VersionInfo{PreRelease: []any{"b"}}}, want: -1},
		{name: "a < b Prerelease int int", args: args{a: VersionInfo{PreRelease: []any{0}}, b: VersionInfo{PreRelease: []any{1}}}, want: -1},
		{name: "a < b Prerelease int string", args: args{a: VersionInfo{PreRelease: []any{0}}, b: VersionInfo{PreRelease: []any{"a"}}}, want: -1},
		{name: "a > b Prerelease string int", args: args{a: VersionInfo{PreRelease: []any{"b"}}, b: VersionInfo{PreRelease: []any{1}}}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Compare(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			} else {
				logger.Debug("Compare() succeeded.", "test", tt, "got", got, "want", tt.want)
			}
		})
	}
}
