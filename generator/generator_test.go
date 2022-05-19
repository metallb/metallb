// SPDX-License-Identifier:Apache-2.0

package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update .golden files")

func TestGeneratreResources(t *testing.T) {
	testtable := []struct {
		tname string
	}{
		{
			tname: "full-config",
		},
		{
			tname: "layer2-config",
		},
		{
			tname: "config",
		},
	}
	for _, tc := range testtable {
		t.Run(tc.tname, func(t *testing.T) {
			log.SetOutput(ioutil.Discard)

			// Override the container's internal path.
			inputDirPath = "testdata"
			source := t.Name() + ".yaml"
			res := new(bytes.Buffer)

			err := generate(res, source)
			if err != nil {
				t.Fatalf("test %s failed to generate resources: %s", tc.tname, err)
			}

			goldenFile := filepath.Join("testdata", t.Name()+".golden")
			if *update {
				t.Log("update golden file")
				if err := ioutil.WriteFile(goldenFile, res.Bytes(), 0644); err != nil {
					t.Fatalf("test %s failed to update golden file: %s", tc.tname, err)
				}
			}

			expected, err := ioutil.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("test %s failed reading .golden file: %s", tc.tname, err)
			}

			if !cmp.Equal(string(expected), res.String()) {
				t.Fatalf("test %s failed. (-want +got):\n%s", tc.tname, cmp.Diff(string(expected), res.String()))
			}
		})
	}
}
