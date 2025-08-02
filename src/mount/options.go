package mount

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/thorbenw/example-docker-volume-plugin/utils"
)

// region Package globals

const (
	DEFAULT_DELETION_MARK = "-"
)

var (
	DeletionMark = DEFAULT_DELETION_MARK
)

// region Option struct

type Option struct {
	Key   string
	Value string
}

func ParseOption(option string) Option {
	kvp := strings.SplitN(option, "=", 2)

	result := Option{Key: strings.TrimSpace(kvp[0])}

	if len(kvp) > 1 {
		kvp[1] = strings.TrimSpace(kvp[1])
		if kvp[1] != "" {
			result.Value = kvp[1]
		}
	}

	return result
}

func (o Option) String() string {
	result := o.Key

	if o.Value != "" {
		result += fmt.Sprintf("=%s", o.Value)
	}

	return result
}

type Options struct {
	options *[]Option
}

func NewOptions(capacity int) Options {
	options := make([]Option, 0, capacity)

	return Options{options: &options}
}

func (o Options) SetMap(value *map[string]string) error {
	if o.options == nil {
		return errors.New("mount.Options.Set(): options must be initialized using NewOptions()")
	}

	if value == nil {
		return errors.New("mount.Options.Set(): value must not be nil")
	}

	options := make([]string, 0, len(*value))
	for k, v := range *value {
		options = append(options, fmt.Sprintf("%s=%s", strings.TrimSpace(k), strings.TrimSpace(v)))
	}

	return o.Set(strings.Join(options, ","))
}

func (o Options) Set(value string) error {
	if o.options == nil {
		return errors.New("mount.Options.Set(): options must be initialized using NewOptions()")
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("mount.Options.Set(): value must not be empty")
	}

	options := strings.Split(value, ",")
	options = utils.Select(options, func(str string) string { return strings.TrimSpace(str) })
	options = utils.Where(options, func(str string) bool { return str != "" })

	for _, option := range options {
		opt := ParseOption(option)
		idx := slices.IndexFunc(*o.options, func(option Option) bool {
			return option.Key == opt.Key
		})

		if opt.Value == DeletionMark {
			if idx >= 0 {
				*o.options = slices.Delete(*o.options, idx, idx+1)
			}
		} else {
			if idx >= 0 {
				(*o.options)[idx] = opt
			} else {
				*o.options = append(*o.options, opt)
			}
		}
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

	options := make([]string, 0, len_options)
	for _, option := range *o.options {
		options = append(options, option.String())
	}

	return strings.Join(options, ",")
}

func (o Options) Map() map[string]string {
	var len_options int
	if o.options != nil {
		len_options = len(*o.options)
	}
	if len_options < 1 {
		return map[string]string{}
	}

	options := make(map[string]string, len_options)
	for _, option := range *o.options {
		options[option.Key] = option.Value
	}

	return options
}

func (o Options) Len() int {
	var result int

	if o.options != nil {
		result = len(*o.options)
	}

	return result
}
