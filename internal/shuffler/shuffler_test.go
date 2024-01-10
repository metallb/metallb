// SPDX-License-Identifier:Apache-2.0

package shuffler

import (
	"flag"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
)

var (
	update = flag.Bool("update", false, "update the golden files of this test")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

type TestStruct struct {
	A int
	B []string
}

type TestStruct2 struct {
	A int
	B []string
	C []TestStruct
	D []*TestStruct
}

func TestShuffle(t *testing.T) {
	tests := []struct {
		desc      string
		toShuffle interface{}
	}{
		{
			desc:      "simple slice",
			toShuffle: []string{"a", "b", "c"},
		},
		{
			desc: "struct",
			toShuffle: TestStruct{
				A: 23,
				B: []string{"a", "c", "b"},
			},
		},
		{
			desc: "with sub slices",
			toShuffle: TestStruct2{
				A: 1,
				B: []string{"a", "b", "c"},
				C: []TestStruct{
					{
						A: 1,
						B: []string{"a", "b", "c"},
					}, {
						A: 2,
						B: []string{"a", "b", "c"},
					},
				},
				D: []*TestStruct{
					{
						A: 1,
						B: []string{"a", "b", "c"},
					}, {
						A: 2,
						B: []string{"a", "b", "c"},
					},
				},
			},
		},
	}

	rnd := rand.New(rand.NewSource(26))
	oldDisablePointer := spew.Config.DisablePointerAddresses
	spew.Config.DisablePointerAddresses = true
	defer func() { spew.Config.DisablePointerAddresses = oldDisablePointer }()
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			Shuffle(test.toShuffle, rnd)
			shuffled := spew.Sdump(test.toShuffle)
			golden := goldenValue(t, shuffled)
			if !cmp.Equal(golden, shuffled) {
				t.Fatalf("Golden different from shuffled %s", cmp.Diff(golden, shuffled))
			}
		})
	}
}

func goldenValue(t *testing.T, actual string) string {
	t.Helper()
	values := strings.Split(t.Name(), "/")
	goldenPath := "testdata/" + values[1] + ".golden"

	f, err := os.OpenFile(goldenPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open file %s: %s", goldenPath, err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			t.Fatalf("Failed to close file %s: %s", goldenPath, err)
		}
	}()

	if *update {
		_, err := f.WriteString(actual)
		if err != nil {
			t.Fatalf("Error writing to file %s: %s", goldenPath, err)
		}

		return actual
	}

	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("Error opening file %s: %s", goldenPath, err)
	}
	return string(content)
}
