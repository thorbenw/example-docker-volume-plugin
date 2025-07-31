package mount

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/thorbenw/example-docker-volume-plugin/utils"
)

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

		if idx >= 0 {
			(*o.options)[idx] = opt
		} else {
			*o.options = append(*o.options, opt)
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
