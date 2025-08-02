package mount

import (
	"fmt"
	"strconv"
	"testing"

	"gotest.tools/assert"
)

func Test_Options(t *testing.T) {
	type args struct {
		valueString string
		valueMap    *map[string]string
	}
	tests := []struct {
		name    string
		o       Options
		args    args
		want    string
		wantMap int
		wantErr bool
	}{
		// Test cases.
		{name: "Nil Map", o: Options{}, args: args{valueString: "flag", valueMap: &map[string]string{"flag": ""}}, want: "", wantErr: true},
		{name: "Default", o: NewOptions(1), args: args{valueString: "flag", valueMap: &map[string]string{"flag": ""}}, want: "flag", wantMap: 1, wantErr: false},
		{name: "Empty Value", o: NewOptions(0), args: args{valueString: "", valueMap: nil}, want: "", wantErr: true},
		{name: "Multi Value", o: NewOptions(2), args: args{valueString: "key1=val0,,key2=val2,,key4, key3 = ", valueMap: &map[string]string{"key1": "val1", "key4": "-"}}, want: "key1=val1,key2=val2,key3", wantMap: 3, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				err error
				msg string
			)

			err = tt.o.Set(tt.args.valueString)
			msg = fmt.Sprintf("Options.Set() error = %v, wantErr %v", err, tt.wantErr)
			if (err != nil) != tt.wantErr {
				t.Error(msg)
			} else {
				t.Log(msg)
			}

			err = tt.o.SetMap(tt.args.valueMap)
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

			gotMap := tt.o.Map()
			t.Logf("Map() = %#v, gotMap = %d, wantMap = %d", gotMap, len(gotMap), tt.wantMap)
			assert.Assert(t, len(gotMap) == tt.wantMap)
		})
	}

}
