// SPDX-License-Identifier:Apache-2.0

package community

import (
	"strings"
	"testing"
)

func TestNewBGPCommunity(t *testing.T) {
	tcs := map[string]struct {
		input       string
		output      BGPCommunity
		errorString string
	}{
		"valid legacy community in new-format": {
			input:  "12345:12345",
			output: BGPCommunityLegacy{upperVal: 12345, lowerVal: 12345},
		},
		"legacy community in old-format not supported": {
			input:       "12345",
			errorString: "invalid community format: 12345",
		},
		"valid large community": {
			input: "large:12345:12345:12345",
			output: BGPCommunityLarge{
				globalAdministrator: 12345,
				localDataPart1:      12345,
				localDataPart2:      12345,
			},
		},
		"invalid large community 1": {
			input:       "larg:12345:12345:12345",
			errorString: "invalid community value: invalid marker for large community",
		},
		"invalid large community 2": {
			input:       "large:12345:wrong:12345",
			errorString: "invalid community value: invalid section",
		},
		"invalid extended community not yet implemented": {
			input:       "12345:12345:12345",
			errorString: "invalid community format: 12345:12345:12345",
		},
	}
	for d, tc := range tcs {
		c, err := New(tc.input)
		if tc.errorString != "" {
			if err == nil {
				t.Fatalf("%s(%s): Expected to see an error but got <nil> instead", t.Name(), d)
			}
			if !strings.Contains(err.Error(), tc.errorString) {
				t.Fatalf("%s(%s): Expected returned error to contain '%s', but got %q instead",
					t.Name(), d, tc.errorString, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s(%s): Expected to see no error, but got %q instead", t.Name(), d, err)
		}
		if c != tc.output {
			t.Fatalf("%s(%s): Parsed community and expected community do not match.\n"+
				"Expected:\n%v\nGot:\n%v\n", t.Name(), d, tc.output, c)
		}
	}
}

func TestBGPCommunityLessThan(t *testing.T) {
	tcs := map[string]struct {
		left            string
		right           string
		expectedOutcome bool
	}{
		"compares legacy communities":           {left: "0:1234", right: "0:2345", expectedOutcome: true},
		"compares large communities 1":          {left: "large:1234:0:0", right: "large:1234:0:1", expectedOutcome: true},
		"compares large communities 2":          {left: "large:1235:0:0", right: "large:1234:1:0", expectedOutcome: false},
		"compares legacy and large communities": {left: "0:1234", right: "large:123:456:789", expectedOutcome: false},
	}
	for d, tc := range tcs {
		leftCommunity, _ := New(tc.left)
		rightCommunity, _ := New(tc.right)
		if leftCommunity.LessThan(rightCommunity) != tc.expectedOutcome {
			t.Fatalf("%s(%s): did not get expected outcome %t when comparing communities %q and %q",
				t.Name(), d, tc.expectedOutcome, tc.left, tc.right)
		}
	}
}

func TestBGPCommunityString(t *testing.T) {
	tcs := map[string]struct {
		input  string
		output string
	}{
		"legacy community": {input: "0:1234", output: "0:1234"},
		"large community":  {input: "large:1:2:3", output: "1:2:3"},
	}
	for d, tc := range tcs {
		community, _ := New(tc.input)
		if community.String() != tc.output {
			t.Fatalf("%s(%s): did not get expected output string %q, got: %q", t.Name(), d, tc.output, community)
		}
	}
}

func FuzzNew(f *testing.F) {
	f.Add("0:1234")
	f.Add("large:1:2:3")
	f.Add("large:1235:0:0")
	f.Add("large:12345:12345:12345")

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = New(input)
	})
}
