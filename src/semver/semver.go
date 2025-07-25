package semver

//go:description Implementation of
import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/thorbenw/example-docker-volume-plugin/utils"
)

type VersionInfo struct {
	Major      uint64
	Minor      uint64
	Patch      uint64
	PreRelease []any
	Build      []any
}

/*
	 func (vi VersionInfo) String() string {
		return ""
	}
*/

// Converts an arbitrary slice into a slice containing only `string` and
// `uint64` elements.
//
// Elements of the input slice having an unsigned integer type are converted to
// `uint64`. Elements having a signed integer type and a value greater than 0
// are converted to `uint64`, or to their `string` representation if negative.
// Elements having any other type are also converted to `string`s, except those
// already being `string` anyway.
//
// The returned sclice has the same lenght and capacity as the input slice.
func MakeComparableSlice(in []any) (out []any) {
	out = make([]any, 0, cap(in))
	for _, v := range in[:] {
		switch a := v.(type) {
		case string:
			out = append(out, a)
		case uint, uint8, uint16, uint32, uint64:
			as, _ := utils.AsUInt64(a)
			out = append(out, as)
		case int, int8, int16, int32, int64:
			as, _ := utils.AsInt64(a)
			if as < 0 {
				out = append(out, fmt.Sprintf("%d", as))
			} else {
				out = append(out, uint64(as))
			}
		default:
			out = append(out, fmt.Sprintf("%v", a))
		}
	}

	return
}

func Parse(v string) (*VersionInfo, error) {
	var result VersionInfo

	slice := strings.SplitN(v, "+", 2)
	if len(slice) > 1 {
		for _, e := range strings.Split(slice[1], ".") {
			if v, err := strconv.ParseUint(e, 10, 64); err != nil {
				result.Build = append(result.Build, e)
			} else {
				result.Build = append(result.Build, v)
			}
		}
	}

	slice = strings.SplitN(slice[0], "-", 2)
	if len(slice) > 1 {
		for _, e := range strings.Split(slice[1], ".") {
			if v, err := strconv.ParseUint(e, 10, 64); err != nil {
				result.PreRelease = append(result.PreRelease, e)
			} else {
				result.PreRelease = append(result.PreRelease, v)
			}
		}
	}

	for i, e := range strings.Split(slice[0], ".") {
		if v, err := strconv.ParseUint(e, 10, 64); err != nil {
			return nil, fmt.Errorf("version spec [%s] is invalid at position %d (`%s`)", slice[0], i, e)
		} else {
			switch i {
			case 0:
				result.Major = v
			case 1:
				result.Minor = v
			case 2:
				result.Patch = v
			default:
				return nil, fmt.Errorf("version spec [%s] is invalid (more than 3 digits)", slice[0])
			}
		}
	}

	return &result, nil
}

// Compare returns an integer comparing two version informations following the
// precedence rules specified in https://semver.org/#spec-item-11.
//
// The result will be 0 if a == b, -1 if a < b, and +1 if a > b.
func Compare(a, b VersionInfo) int {
	cmpUint64 := func(ui1, ui2 uint64) int {
		if ui1 > ui2 {
			return 1
		}
		if ui1 < ui2 {
			return -1
		}

		return 0
	}

	cmpAny := func(any1, any2 []any) int {
		l1 := float64(len(any1))
		l2 := float64(len(any2))

		lm := math.Min(l1, l2)
		if lm == 0 {
			return 0 - cmpUint64(uint64(l1), uint64(l2))
		}

		arr1 := MakeComparableSlice(any1)
		arr2 := MakeComparableSlice(any2)
		for i, v1 := range arr1[:int(lm)] {
			v2 := arr2[i]

			switch a1 := v1.(type) {
			case string:
				switch a2 := v2.(type) {
				case string:
					if c := strings.Compare(a1, a2); c != 0 {
						return c
					}
				default:
					return 1
				}
			case uint64:
				switch a2 := v2.(type) {
				case uint64:
					if c := cmpUint64(a1, a2); c != 0 {
						return c
					}
				default:
					return -1
				}
				//default:
				//	panic(fmt.Errorf("Type %T wasn't expected to ever turn up here.", a1))
			}
		}

		return cmpUint64(uint64(l1), uint64(l2))
	}

	if c := cmpUint64(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmpUint64(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmpUint64(a.Patch, b.Patch); c != 0 {
		return c
	}

	return cmpAny(a.PreRelease, b.PreRelease)
}
