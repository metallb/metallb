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
		{
			name: "raw option marshal symmetry",
			s: "\x860000000000000000!00" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000" +
				"00000000000000000000",
		},
		{
			name: "rdnss no servers",
			s:    "\x850000000\x19\x01000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = fuzz([]byte(tt.s))
		})
	}
}
