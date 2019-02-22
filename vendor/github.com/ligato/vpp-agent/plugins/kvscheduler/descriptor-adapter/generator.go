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

// descriptor-generator generates all boiler-plate code needed to adapt type-safe
// KV descriptor for the KVDescriptor structure definition.
//
// To use the generator, add go generate command into your descriptor as a comment:
//  //go:generate descriptor-adapter --descriptor-name <descriptor-name> --value-type <typename> [--meta-type <typename>] [--output-dir <path>] [--import <path>]...
//
// Note: import paths can be relative to the file with the go:generate comment.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ArrayFlag implements repeated flag.
type ArrayFlag struct {
	values []string
}

// String return human-readable string representation of the array of flags.
func (af *ArrayFlag) String() string {
	str := "["
	for idx, value := range af.values {
		str += value
		if idx < len(af.values)-1 {
			str += ", "
		}
	}
	str += "]"
	return str
}

// Set add value into the array.
func (af *ArrayFlag) Set(value string) error {
	af.values = append(af.values, value)
	return nil
}

var (
	imports ArrayFlag

	outputDirFlag      = flag.String("output-dir", ".", "Output directory where adapter package will be generated.")
	descriptorNameFlag = flag.String("descriptor-name", "", "Name of the descriptor.")
	valueTypeFlag      = flag.String("value-type", "", "Type of the described values.")
	metaTypeFlag       = flag.String("meta-type", "interface{}", "Type of the metadata used by the descriptor.")
)

// TemplateData encapsulates input arguments for the template.
type TemplateData struct {
	DescriptorName string
	ValueT         string
	MetadataT      string
	Imports        []string
}

// PathExists return true if the given path already exist in the file system.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func main() {
	flag.Var(&imports, "import", "Package to be imported in the generated adapter (can be relative path).")
	flag.Parse()

	// prepare input data for the template
	inputData := TemplateData{
		DescriptorName: *descriptorNameFlag,
		ValueT:         *valueTypeFlag,
		MetadataT:      *metaTypeFlag,
	}

	// expand relative import paths
	gopath := os.Getenv("GOPATH")
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: ", err)
		os.Exit(2)
	}

	for _, importPath := range imports.values {
		if !PathExists(filepath.Join(gopath, "src", importPath)) {
			asRelative := filepath.Join(cwd, importPath)
			if PathExists(asRelative) {
				importPath = filepath.Clean(asRelative)
				importPath = strings.TrimPrefix(importPath, gopath+"/src")
				importPath = strings.TrimLeft(importPath, "/")
			}
		}
		inputData.Imports = append(inputData.Imports, importPath)
	}

	if inputData.ValueT == "" || inputData.DescriptorName == "" {
		fmt.Fprintln(os.Stderr, "ERROR: value-type and descriptor-name must be specified")
		os.Exit(1)
	}

	// generate adapter source code from the template
	var buf bytes.Buffer
	t := template.Must(template.New("").Parse(adapterTemplate))
	err = t.Execute(&buf, inputData)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: ", err)
		os.Exit(2)
	}

	// prepare directory for the generated adapter
	directory := *outputDirFlag + "/adapter/"
	err = os.MkdirAll(directory, 0777)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: ", err)
		os.Exit(3)
	}

	// output the generated adapter into the file
	filename := directory + "/" + strings.ToLower(*descriptorNameFlag) + ".go"
	err = ioutil.WriteFile(filename, buf.Bytes(), 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: ", err)
		os.Exit(4)
	}
}
