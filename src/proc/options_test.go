package proc

import (
	"fmt"
	"strconv"
	"testing"

	"gotest.tools/assert"
)

func Test_Options(t *testing.T) {
	type args struct {
		value string
	}
	tests := []struct {
		name    string
		o       Options
		args    args
		want    string
		wantSlc int
		wantErr bool
	}{
		// Test cases.
		{name: "Nil Map", o: Options{}, args: args{value: "flag"}, want: "", wantErr: true},
		{name: "Default", o: NewOptions(1, "", true), args: args{value: "flag"}, want: "flag", wantSlc: 1, wantErr: false},
		{name: "Empty Value", o: NewOptions(0, "", true), args: args{value: ""}, want: "", wantErr: true},
		{name: "Multi Value", o: NewOptions(2, "&", true), args: args{value: "-f &-f&--option& & option with spaces"}, want: "-f&--option&option with spaces", wantSlc: 3, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				err error
				msg string
			)

			err = tt.o.Set(tt.args.value)
			msg = fmt.Sprintf("Options.Set() error = %v, wantErr %v", err, tt.wantErr)
			if (err != nil) != tt.wantErr {
				t.Error(msg)
			} else {
				t.Log(msg)
			}

			if tt.want != "" {
				assert.Assert(t, tt.o.Len() > 0)
			} else {
				assert.Assert(t, tt.o.Len() == 0)
			}

			got := tt.o.String()
			want := tt.want
			t.Logf("got = %s, want = %s", strconv.Quote(got), strconv.Quote(want))
			assert.Assert(t, got == want)

			gotSlice := tt.o.Slice()
			t.Logf("Slice() = %#v, gotSlice = %d, wantSlc = %d", gotSlice, len(gotSlice), tt.wantSlc)
			assert.Assert(t, len(gotSlice) == tt.wantSlc)
		})
	}
}
