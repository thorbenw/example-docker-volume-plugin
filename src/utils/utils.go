package utils

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/user"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

func SHA256BytesToBytes(bytes []byte) []byte {
	var sha256 = sha256.New()

	sha256.Write(bytes)

	return sha256.Sum(nil)
}

func SHA256StringToString(str string) string {
	return fmt.Sprintf("%x", SHA256BytesToBytes([]byte(str)))
}

type CheckAccessUserSpec string

const CheckAccessCurrentUser = CheckAccessUserSpec("")

func checkAccess(usr user.User, perm os.FileMode, file os.FileInfo) error {
	switch sys := file.Sys().(type) {
	case *syscall.Stat_t:
		mod := file.Mode() & 0o777
		act := mod & 0o7
		exp := perm & 0o777

		uid, err := strconv.ParseUint(usr.Uid, 10, 32)
		if err != nil {
			return err
		}
		if uint64(sys.Uid) == uid {
			act |= os.FileMode(mod >> 6)
		}

		gids, err := usr.GroupIds()
		if err != nil {
			return err
		}

		for _, g := range gids {
			gid, err := strconv.ParseUint(g, 10, 32)
			if err != nil {
				return err
			}

			if uint64(sys.Gid) == gid {
				act |= os.FileMode((mod & 0o70) >> 3)
				break
			}
		}

		if (act & exp) != exp {
			return fmt.Errorf("CheckAcces %s: required permissions %s are not met by actual permissions %s", file.Name(), exp, act)
		}
	default:
		return fmt.Errorf("%T.Sys() returned unknown type %T", file, sys)
	}

	return nil
}

// Checks if `userName` has `perm` access to `path`.
func CheckAccess(userName CheckAccessUserSpec, perm os.FileMode, path string) error {
	var un = string(userName)
	var err error
	var usr *user.User
	if strings.TrimSpace(un) == "" {
		usr, err = user.Current()
	} else {
		usr, err = user.Lookup(un)
	}
	if err != nil {
		return err
	}

	file, err := os.Lstat(path)
	if err != nil {
		return err
	}

	return checkAccess(*usr, perm, file)
}

// Interprets a string as a hexadecimal expression and converts it to a sequence
// of bytes.
func Atob(hexString string) (bytes []byte, fail error) {
	var bytestr string

	pos := len(hexString) - 2
	bytes = make([]byte, 0, (pos+3)/2)
	for i := pos; i >= -1; i -= 2 {
		if i >= 0 {
			bytestr = hexString[i : i+2]
		} else {
			bytestr = hexString[0:1]
		}
		ubyte, err := strconv.ParseUint(bytestr, 16, 8)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, byte(ubyte))
	}

	slices.Reverse(bytes)
	return
}

// Converts the array values to a proper string.
func Int8ToStr(slice []int8) string {
	b := make([]byte, 0, len(slice))
	for _, v := range slice {
		if v == 0x00 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}

func ToInt64[T int | int8 | int16 | int32 | int64](i T) int64 {
	return int64(i)
}

func AsInt64(i any) (int64, error) {
	switch in := i.(type) {
	case int:
		return int64(in), nil
	case int8:
		return int64(in), nil
	case int16:
		return int64(in), nil
	case int32:
		return int64(in), nil
	case int64:
		return in, nil
	default:
		return 0, fmt.Errorf("non-integer type %T is invalid", i)
	}
}

func ToUInt64[T uint | uint8 | uint16 | uint32 | uint64](i T) uint64 {
	return uint64(i)
}

func AsUInt64(i any) (uint64, error) {
	switch in := i.(type) {
	case uint:
		return uint64(in), nil
	case uint8:
		return uint64(in), nil
	case uint16:
		return uint64(in), nil
	case uint32:
		return uint64(in), nil
	case uint64:
		return in, nil
	default:
		return 0, fmt.Errorf("non-unsigned-integer type %T is invalid", i)
	}
}

// ScanStrings is a split function for a [Scanner] that returns each null
// terminated string in a buffer, stripped of the trailing null character. The
// returned string may be empty, if multiple consecutive null characters exist.
// The last non-empty string of input will be returned even if it has no
// terminating null character.
//
// Based on https://stackoverflow.com/questions/33068644/how-a-scanner-can-be-implemented-with-a-custom-split
func ScanStrings(data []byte, atEOF bool) (advance int, token []byte, err error) {

	if atEOF {
		if len(data) == 0 {
			// Return nothing if at end of file and no data passed
			return 0, nil, nil
		} else {
			// If at end of file with data return the data
			return len(data), data, nil
		}
	}

	// Find the index of the input of a null character.
	for i, v := range data {
		if v == 0 {
			return i + 1, data[0:i], nil
		}
	}

	return
}

// Select transforms all elements of a `slice` using the specified `function`.
func Select[T any](slice []T, action func(T) T) []T {
	var result = make([]T, 0, len(slice))

	for _, element := range slice {
		result = append(result, action(element))
	}

	return result
}
