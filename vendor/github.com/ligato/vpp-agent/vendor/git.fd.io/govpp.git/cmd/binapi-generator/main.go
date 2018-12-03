// Copyright (c) 2018 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bennyscetbun/jsongo"
	"github.com/sirupsen/logrus"
)

var (
	inputFile       = flag.String("input-file", "", "Input JSON file.")
	inputDir        = flag.String("input-dir", ".", "Input directory with JSON files.")
	outputDir       = flag.String("output-dir", ".", "Output directory where package folders will be generated.")
	includeAPIVer   = flag.Bool("include-apiver", false, "Whether to include VlAPIVersion in generated file.")
	debug           = flag.Bool("debug", false, "Turn on debug mode.")
	continueOnError = flag.Bool("continue-onerror", false, "Wheter to continue with next file on error.")
)

func init() {
	flag.Parse()
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func logf(f string, v ...interface{}) {
	if *debug {
		logrus.Debugf(f, v...)
	}
}

var log = logrus.Logger{
	Level:     logrus.InfoLevel,
	Formatter: &logrus.TextFormatter{},
	Out:       os.Stdout,
}

func main() {
	if *inputFile == "" && *inputDir == "" {
		fmt.Fprintln(os.Stderr, "ERROR: input-file or input-dir must be specified")
		os.Exit(1)
	}

	if *inputFile != "" {
		// process one input file
		if err := generateFromFile(*inputFile, *outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: code generation from %s failed: %v\n", *inputFile, err)
			os.Exit(1)
		}
	} else {
		// process all files in specified directory
		files, err := getInputFiles(*inputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: code generation failed: %v\n", err)
			os.Exit(1)
		}
		for _, file := range files {
			if err := generateFromFile(file, *outputDir); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: code generation from %s failed: %v\n", file, err)
				if *continueOnError {
					continue
				}
				os.Exit(1)
			}
		}
	}
}

// getInputFiles returns all input files located in specified directory
func getInputFiles(inputDir string) (res []string, err error) {
	files, err := ioutil.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s failed: %v", inputDir, err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), inputFileExt) {
			res = append(res, filepath.Join(inputDir, f.Name()))
		}
	}
	return res, nil
}

// generateFromFile generates Go package from one input JSON file
func generateFromFile(inputFile, outputDir string) error {
	logf("generating from file: %q", inputFile)
	defer logf("--------------------------------------")

	ctx, err := getContext(inputFile, outputDir)
	if err != nil {
		return err
	}

	// read input file contents
	ctx.inputData, err = readFile(inputFile)
	if err != nil {
		return err
	}
	// parse JSON data into objects
	jsonRoot, err := parseJSON(ctx.inputData)
	if err != nil {
		return err
	}
	ctx.packageData, err = parsePackage(ctx, jsonRoot)
	if err != nil {
		return err
	}

	// create output directory
	packageDir := filepath.Dir(ctx.outputFile)
	if err := os.MkdirAll(packageDir, 0777); err != nil {
		return fmt.Errorf("creating output directory %q failed: %v", packageDir, err)
	}
	// open output file
	f, err := os.Create(ctx.outputFile)
	if err != nil {
		return fmt.Errorf("creating output file %q failed: %v", ctx.outputFile, err)
	}
	defer f.Close()

	// generate Go package code
	w := bufio.NewWriter(f)
	if err := generatePackage(ctx, w); err != nil {
		return err
	}

	// go format the output file (fail probably means the output is not compilable)
	cmd := exec.Command("gofmt", "-w", ctx.outputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gofmt failed: %v\n%s", err, string(output))
	}

	// count number of lines in generated output file
	cmd = exec.Command("wc", "-l", ctx.outputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Warnf("wc command failed: %v\n%s", err, string(output))
	} else {
		logf("generated lines: %s", output)
	}

	return nil
}

// readFile reads content of a file into memory
func readFile(inputFile string) ([]byte, error) {
	inputData, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return nil, fmt.Errorf("reading data from file failed: %v", err)
	}

	return inputData, nil
}

// parseJSON parses a JSON data into an in-memory tree
func parseJSON(inputData []byte) (*jsongo.JSONNode, error) {
	root := jsongo.JSONNode{}

	if err := json.Unmarshal(inputData, &root); err != nil {
		return nil, fmt.Errorf("unmarshalling JSON failed: %v", err)
	}

	return &root, nil
}
