package ndp

import "testing"

func Test_fuzz(t *testing.T) {
	tests := []struct {
		name string
		s    string
	}{
		{
			name: "parse option length",
			s:    "\x86000000000000000\x01\xc0",
		},
		{
			name: "prefix information length",
			s: "\x86000000000000000\x03\x0100" +
				"0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = fuzz([]byte(tt.s))
		})
	}
}
