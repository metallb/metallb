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

const (
	testConfigMapSource  = false
	testDataOnlySource   = true
	configMapYAMLTestDir = "./testdata/configmap-yaml"
	configMapDataTestDir = "./testdata/configmap-data"
)

func TestGenerateResourcesWithConfigMapDefinition(t *testing.T) {
	testGenerate(t, configMapYAMLTestDir, testConfigMapSource)
}
func TestGenerateResourcesWithConfigData(t *testing.T) {
	testGenerate(t, configMapDataTestDir, testDataOnlySource)
}

func testGenerate(t *testing.T, testDir string, testOnlyData bool) {
	tests := []string{}
	files, err := ioutil.ReadDir(testDir)
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

			res := new(bytes.Buffer)
			// Override the container's internal path.
			inputDirPath = testDir
			onlyData = &testOnlyData

			err := generate(res, tc)

			if err != nil && !strings.Contains(tc, "bad") {
				t.Fatalf("test %s failed to generate resources: %s", tc, err)
			}

			goldenFile := filepath.Join(testDir, strings.TrimSuffix(tc, path.Ext(tc))+".golden")
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
