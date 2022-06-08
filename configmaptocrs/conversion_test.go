// SPDX-License-Identifier:Apache-2.0

package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update .golden files")

func TestGenerateResources(t *testing.T) {
	tests := []string{}
	files, err := ioutil.ReadDir("./testdata")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		if path.Ext(file.Name()) == ".yaml" {
			tests = append(tests, path.Base(file.Name()))
		}
	}

	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			log.SetOutput(ioutil.Discard)

			// Override the container's internal path.
			inputDirPath = "testdata"

			res := new(bytes.Buffer)

			err := generate(res, tc)

			if strings.Contains(tc, "bad") && !strings.Contains(err.Error(), "not enough addresses") {
				t.Fatalf("test %s failed, expecting error: %s", tc, err.Error())
				return
			}
			if err != nil && !strings.Contains(tc, "bad") {
				t.Fatalf("test %s failed to generate resources: %s", tc, err)
			}

			goldenFile := filepath.Join("testdata", strings.TrimSuffix(tc, path.Ext(tc))+".golden")
			if *update {
				t.Log("update golden file")
				if err := ioutil.WriteFile(goldenFile, res.Bytes(), 0644); err != nil {
					t.Fatalf("test %s failed to update golden file: %s", tc, err)
				}
			}

			expected, err := ioutil.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("test %s failed reading .golden file: %s", tc, err)
			}

			if !cmp.Equal(string(expected), res.String()) {
				t.Fatalf("test %s failed. (-want +got):\n%s", tc, cmp.Diff(string(expected), res.String()))
			}
		})
	}
}
