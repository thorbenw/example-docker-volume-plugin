package utils

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSHA256StringToString(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// Test cases.
		{name: "Hash string first run.", args: args{str: "sha256 this string"}, want: "1af1dfa857bf1d8814fe1af8983c18080019922e557f15a8a0d3db739d77aacb"},
		{name: "Hash string  repeated.", args: args{str: "sha256 this string"}, want: "1af1dfa857bf1d8814fe1af8983c18080019922e557f15a8a0d3db739d77aacb"},
		{name: "Hash string   encoded.", args: args{str: "sha256 äÄöÖüÜßshit"}, want: "b254893091135c4727ef8fa106d0b5035a19097617595dc61311d52097754f3d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SHA256StringToString(tt.args.str); got != tt.want {
				t.Errorf("SHA256StringToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mock_FileInfo struct {
}

func (m mock_FileInfo) IsDir() bool {
	return false
}

func (m mock_FileInfo) ModTime() time.Time {
	return time.Now()
}

func (m mock_FileInfo) Mode() fs.FileMode {
	return fs.FileMode(0)
}

func (m mock_FileInfo) Name() string {
	return "Mock"
}

func (m mock_FileInfo) Size() int64 {
	return 0
}

func (m mock_FileInfo) Sys() any {
	return 0
}

func Test_checkAccess(t *testing.T) {
	testFile, err := os.OpenFile(filepath.Join(t.TempDir(), SHA256StringToString("TestCheckAccess")), os.O_CREATE, os.FileMode(0o240))
	if err != nil {
		t.Fatal(err)
	} else {
		if err := testFile.Close(); err != nil {
			t.Fatal(err)
		}
	}
	testFileInfo, err := os.Lstat(testFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	mockFileInfo := mock_FileInfo{}

	testUser, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		usr  user.User
		perm os.FileMode
		file os.FileInfo
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Test cases.
		{name: "Invalid Sys() type", args: args{usr: *testUser, perm: os.FileMode(0), file: mockFileInfo}, wantErr: true},
		{name: "Invalid uid", args: args{usr: user.User{Uid: "invalid"}, perm: os.FileMode(0), file: testFileInfo}, wantErr: true},
		{name: "Invalid uid", args: args{usr: user.User{Uid: testUser.Uid}, perm: os.FileMode(0), file: testFileInfo}, wantErr: true},
		{name: "Fail", args: args{usr: *testUser, perm: os.FileMode(0o7), file: testFileInfo}, wantErr: true},
		{name: "Success", args: args{usr: *testUser, perm: os.FileMode(0o6), file: testFileInfo}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkAccess(tt.args.usr, tt.args.perm, tt.args.file); err != nil {
				if !tt.wantErr {
					t.Errorf("CheckAccess() error = %v, wantErr %v", err, tt.wantErr)
				} else {
					t.Logf("CheckAccess() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestCheckAccess(t *testing.T) {
	testFile, err := os.OpenFile(filepath.Join(t.TempDir(), SHA256StringToString("TestCheckAccess")), os.O_CREATE, os.FileMode(0o241))
	if err != nil {
		t.Fatal(err)
	} else {
		if err := testFile.Close(); err != nil {
			t.Fatal(err)
		}
	}
	defaultPluginSockDir := "/run/docker/plugins" // main.DEFAULT_PLUGIN_SOCK_DIR

	type args struct {
		user CheckAccessUserSpec
		perm os.FileMode
		path string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Test cases.
		{name: "Invalid user name", args: args{user: CheckAccessUserSpec(testFile.Name()), perm: os.FileMode(0o000), path: ""}, wantErr: true},
		{name: "Let LStat fail", args: args{user: "", perm: os.FileMode(0o000), path: defaultPluginSockDir}, wantErr: true},
		{name: "Success", args: args{user: "", perm: os.FileMode(0o007), path: testFile.Name()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckAccess(tt.args.user, tt.args.perm, tt.args.path); err != nil {
				if !tt.wantErr {
					t.Errorf("CheckAccess() error = %v, wantErr %v", err, tt.wantErr)
				} else {
					t.Logf("CheckAccess() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestAtob(t *testing.T) {
	t.Parallel()

	type args struct {
		hexString string
	}
	tests := []struct {
		name      string
		args      args
		wantBytes []byte
		wantErr   bool
	}{
		// Test cases.
		{name: "Empty", args: args{hexString: ""}, wantBytes: []byte{}},
		{name: "Zero1", args: args{hexString: "0"}, wantBytes: []byte{0x0}},
		{name: "Zero2", args: args{hexString: "00"}, wantBytes: []byte{0x0}},
		{name: "Zero3", args: args{hexString: "000"}, wantBytes: []byte{0x0, 0x0}},
		{name: "Regular2", args: args{hexString: "ff"}, wantBytes: []byte{0xff}},
		{name: "Leading Zero", args: args{hexString: "0ff"}, wantBytes: []byte{0x00, 0xff}},
		{name: "Regular3", args: args{hexString: "fff"}, wantBytes: []byte{0xf, 0xff}},
		{name: "Regular4", args: args{hexString: "afff"}, wantBytes: []byte{0xaf, 0xff}},
		{name: "Fail", args: args{hexString: "g"}, wantBytes: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := Atob(tt.args.hexString)
			if (err != nil) != tt.wantErr {
				t.Errorf("Atob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotBytes, tt.wantBytes) {
				t.Errorf("Atob() = %v, want %v", gotBytes, tt.wantBytes)
			}
		})
	}
}

func TestScanStrings(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantToken []string
		wantErr   bool
	}{
		// Test cases.
		{name: "1 element,  unterminated", data: []byte("Here is a string..."), wantToken: []string{"Here is a string..."}},
		{name: "2 elements, unterminated", data: append(append([]byte("one"), 0), []byte("two")...), wantToken: []string{"one", "two"}},
		{name: "2 elements,   terminated", data: []byte("one\x00two\x00"), wantToken: []string{"one", "two"}},
		{name: "3 elements, trail. empty", data: []byte("o\x00t\x00\x00"), wantToken: []string{"o", "t", ""}},
		{name: "3 elements,  lead. empty", data: []byte("\x00o\x00t\x00"), wantToken: []string{"", "o", "t"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(bytes.NewReader(tt.data))
			scanner.Split(ScanStrings)

			gotToken := []string{}
			for scanner.Scan() {
				gotToken = append(gotToken, scanner.Text())
			}

			if err := scanner.Err(); (err != nil) != tt.wantErr {
				t.Errorf("ScanStrings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotToken, tt.wantToken) {
				t.Errorf("ScanStrings() gotToken = %v, want %v", gotToken, tt.wantToken)
			}
		})
	}
}

func TestSelect(t *testing.T) {
	type args[T any] struct {
		slice  []T
		action func(T) T
	}
	tests := []struct {
		name string
		args args[string]
		want []string
	}{
		// Test cases.
		{name: "String", args: args[string]{slice: []string{"MiXeD"}, action: strings.ToLower}, want: []string{"mixed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Select(tt.args.slice, tt.args.action); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Select() = %v, want %v", got, tt.want)
			}
		})
	}
}
