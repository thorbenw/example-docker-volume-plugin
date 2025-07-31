package metric

import (
	"fmt"
	"testing"
	"time"

	"gotest.tools/assert"
)

func Test_MetricBase(t *testing.T) {
	type args struct {
		rateLimit MetricRateLimit
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Test cases.
		{name: "Default", args: args{rateLimit: MetricRateLimit{Limit: 3, Duration: time.Minute}}, wantErr: false},
		{name: "Fail", args: args{rateLimit: MetricRateLimit{Limit: 3, Duration: 0}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMetricBase[int](tt.args.rateLimit)
			if got == nil {
				msg := fmt.Sprintf("NewMetricBase() error = %v, wantErr %v", err, tt.wantErr)
				if !tt.wantErr {
					t.Error(msg)
				} else {
					t.Log(msg)
				}
				return
			}

			var (
				rate         uint
				duration     time.Duration
				limitReached bool
			)

			var i uint
			for i = range uint(6) {
				len_events := len(got.events)
				cap_events := cap(got.events)
				rate, duration, limitReached = got.Rate()
				t.Logf("rate = %d, duration = %s, limitReached = %t, len(events) = %d, cap(events) = %d", rate, duration, limitReached, len_events, cap_events)

				if rate != i {
					assert.Assert(t, rate == uint(len_events))
				}
				assert.Assert(t, duration == tt.args.rateLimit.Duration)
				assert.Assert(t, limitReached == (rate >= tt.args.rateLimit.Limit))
				assert.Assert(t, cap_events == got.capacity)
				assert.Assert(t, len_events <= cap_events)

				got.Update(0)
			}
		})
	}
}
