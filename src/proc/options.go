package proc

import (
	"fmt"
	"slices"
	"strings"

	"github.com/thorbenw/example-docker-volume-plugin/utils"
)

// region Package globals

const (
	DEFAULT_SEPARATOR = " "
)

// region Options struct

type Options struct {
	options   *[]string
	separator string
	// Specifies whether to perform convenience operations while processing
	// options:
	//
	// - Separator specification in NewOptions() will be trimmed (may lead to
	//   default separator application!).
	// - Value specification as well as every single option will be trimmed from
	//   any leading or trailing white space characters.
	// - Empty options will be omitted.
	// - Duplicate options will be discarded
	trim bool
}

func NewOptions(capacity int, separator string, trim bool) Options {
	options := make([]string, 0, capacity)

	if trim {
		separator = strings.TrimSpace(separator)
	}
	if separator == "" {
		separator = DEFAULT_SEPARATOR
	}

	return Options{options: &options, separator: separator, trim: trim}
}

func OptionsString(o *Options, goSyntax bool) string {
	format := "%v"
	if goSyntax {
		format = "%#v"
	}

	if o == nil {
		return fmt.Sprintf(format, o)
	} else {
		return fmt.Sprintf(format, o.Slice())
	}
}

func (o Options) SetSlice(value *[]string) error {
	if o.options == nil {
		return fmt.Errorf("%T.SetSlice(): options must be initialized using NewOptions()", o)
	}

	if value == nil {
		return fmt.Errorf("%T.SetSlice(): value must not be nil", o)
	}

	return o.Set(strings.Join(*value, o.separator))
}

func (o Options) Set(value string) error {
	if o.options == nil {
		return fmt.Errorf("%T.Set(): options must be initialized using NewOptions()", o)
	}

	if o.trim {
		value = strings.TrimSpace(value)
	}
	if value == "" {
		return fmt.Errorf("%T.Set(): value must not be nil", o)
	}

	options := strings.Split(value, o.separator)
	if o.trim {
		options = utils.Select(options, func(str string) string { return strings.TrimSpace(str) })
		options = utils.Where(options, func(str string) bool { return str != "" })
	}

	for _, option := range options {
		if o.trim {
			option = strings.TrimSpace(option)
			if slices.Contains(*o.options, option) {
				continue
			}
		}

		*o.options = append(*o.options, option)
	}

	return nil
}

func (o Options) String() string {
	var len_options int
	if o.options != nil {
		len_options = len(*o.options)
	}
	if len_options < 1 {
		return ""
	}

	return strings.Join(*o.options, o.separator)
}

func (o Options) Slice() []string {
	if o.options != nil {
		return *o.options
	} else {
		return []string{}
	}
}

func (o Options) Len() int {
	var result int

	if o.options != nil {
		result = len(*o.options)
	}

	return result
}
