package proc

import (
	"fmt"
	"strconv"
	"testing"

	"gotest.tools/assert"
)

func Test_Options(t *testing.T) {
	type args struct {
		valueString string
		valueSlice  *[]string
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
		{name: "Nil Options", o: Options{}, args: args{valueString: "flag", valueSlice: &[]string{"flag"}}, want: "", wantErr: true},
		{name: "Default", o: NewOptions(1, "", true), args: args{valueString: "flag", valueSlice: &[]string{"flag"}}, want: "flag", wantSlc: 1, wantErr: false},
		{name: "Empty Value", o: NewOptions(0, "", true), args: args{valueString: "", valueSlice: nil}, want: "", wantErr: true},
		{name: "Multi Value", o: NewOptions(2, "&", true), args: args{valueString: "-f &-f&--option& & option with spaces", valueSlice: &[]string{"-v"}}, want: "-f&--option&option with spaces&-v", wantSlc: 4, wantErr: false},
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

			err = tt.o.SetSlice(tt.args.valueSlice)
			msg = fmt.Sprintf("Options.SetSlice() error = %v, wantErr %v", err, tt.wantErr)
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

func TestOptionsString(t *testing.T) {
	type args struct {
		o        *Options
		goSyntax bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// Test cases.
		{name: "Nil", want: "<nil>"},
		{name: "Nil_GoSyntax", args: args{goSyntax: true}, want: "(*proc.Options)(nil)"},
		{name: "Default", args: args{o: &Options{options: &[]string{"option1", "option2"}}}, want: "[option1 option2]"},
		{name: "Default_GoSyntax", args: args{o: &Options{options: &[]string{"option1", "option2"}}, goSyntax: true}, want: "[]string{\"option1\", \"option2\"}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OptionsString(tt.args.o, tt.args.goSyntax); got != tt.want {
				t.Errorf("OptionsString() = %v, want %v", got, tt.want)
			} else {
				t.Logf("OptionsString() = %v", got)
			}
		})
	}
}
