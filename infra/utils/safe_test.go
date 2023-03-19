package utils

import "testing"

func TestGoWithAction(t *testing.T) {
	type args struct {
		fn          func()
		panicAction func(r interface{})
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "should-panic",
			args: args{
				fn: func() {
					panic("should-panic")
				},
				panicAction: func(r interface{}) {
					t.Log(r)
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GoWithAction(tt.args.fn, tt.args.panicAction)
		})
	}
}
