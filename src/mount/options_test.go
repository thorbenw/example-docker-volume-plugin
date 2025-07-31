package mount

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
		wantErr bool
	}{
		// Test cases.
		{name: "Nil Map", o: Options{}, args: args{value: "falg"}, want: "", wantErr: true},
		{name: "Default", o: NewOptions(1), args: args{value: "flag"}, want: "flag", wantErr: false},
		{name: "Empty Value", o: NewOptions(0), args: args{value: ""}, want: "", wantErr: true},
		{name: "Multi Value", o: NewOptions(2), args: args{value: "key1=val0,,key2=val2,,key1=val1, key3 = "}, want: "key1=val1,key2=val2,key3", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.o.Set(tt.args.value)
			msg := fmt.Sprintf("Options.Set() error = %v, wantErr %v", err, tt.wantErr)
			if (err != nil) != tt.wantErr {
				t.Error(msg)
			} else {
				t.Log(msg)
			}

			got := tt.o.String()
			want := tt.want
			t.Logf("got = %s, want =%s", strconv.Quote(got), strconv.Quote(want))
			assert.Assert(t, got == want)
		})
	}

}
