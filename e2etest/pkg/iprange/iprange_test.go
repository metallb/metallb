// SPDX-License-Identifier:Apache-2.0

package iprange

import (
	"net/netip"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []netip.Prefix
		wantErr bool
	}{
		{
			name:  "masks unmasked CIDR",
			input: "192.0.2.17/24",
			want:  []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		},
		{
			name:  "summarizes IPv4 range",
			input: "192.0.2.1-192.0.2.4",
			want: []netip.Prefix{
				netip.MustParsePrefix("192.0.2.1/32"),
				netip.MustParsePrefix("192.0.2.2/31"),
				netip.MustParsePrefix("192.0.2.4/32"),
			},
		},
		{
			name:  "normalizes mapped CIDR",
			input: "::ffff:192.0.2.17/120",
			want:  []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		},
		{
			name:  "normalizes mapped range",
			input: "::ffff:192.0.2.1-::ffff:192.0.2.3",
			want: []netip.Prefix{
				netip.MustParsePrefix("192.0.2.1/32"),
				netip.MustParsePrefix("192.0.2.2/31"),
			},
		},
		{name: "rejects invalid CIDR", input: "not-a-prefix", wantErr: true},
		{name: "rejects mixed-family range", input: "192.0.2.1-2001:db8::1", wantErr: true},
		{name: "rejects reversed range", input: "192.0.2.2-192.0.2.1", wantErr: true},
		{name: "rejects mapped CIDR crossing family boundary", input: "::ffff:192.0.2.1/95", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %t", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
