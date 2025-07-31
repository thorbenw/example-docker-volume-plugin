package metric

import (
	"fmt"
	"slices"
	"sync"
	"time"
)

// region noCopy struct

// noCopy may be embedded into structs which must not be copied after the first
// use.
//
// See https://stackoverflow.com/questions/68183168/how-to-force-compiler-error-if-struct-shallow-copy
// for details.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// region Package globals

const (
	MIN_RATE_LIMIT_DURATION = time.Second
)

// region Metric interface

type Metric[T any] interface {
	Update(value T)
	Rate() (uint, time.Duration, bool)
}

// region Metric structs

type MetricEvent[T any] struct {
	time.Time
	Value T
}

type MetricRateLimit struct {
	Limit    uint
	Duration time.Duration
}

// region MetricBase struct

type MetricBase[T any] struct {
	noCopy    noCopy
	RateLimit MetricRateLimit
	capacity  int
	events    []MetricEvent[T]
	mutex     sync.Mutex
}

func NewMetricBase[T any](rateLimit MetricRateLimit) (*MetricBase[T], error) {
	if rateLimit.Duration < MIN_RATE_LIMIT_DURATION {
		return nil, fmt.Errorf("rate limit duration must not be less than %s", MIN_RATE_LIMIT_DURATION)
	}

	cap := int(rateLimit.Limit) + 1

	return &MetricBase[T]{RateLimit: rateLimit, capacity: cap, events: make([]MetricEvent[T], 0, cap), mutex: sync.Mutex{}}, nil
}

func (m *MetricBase[T]) Update(value T) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	len_events := len(m.events)
	if len_events >= m.capacity {
		m.events = slices.Delete(m.events, 0, (len_events-m.capacity)+1)
	}

	m.events = append(m.events, MetricEvent[T]{Time: time.Now(), Value: value})
}

func (m *MetricBase[T]) Rate() (uint, time.Duration, bool) {
	var limit = time.Now().Add(0 - m.RateLimit.Duration)
	var rate uint = 0

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, event := range m.events {
		if event.Time.After(limit) {
			rate++
		}
	}

	return rate, m.RateLimit.Duration, rate >= m.RateLimit.Limit
}
