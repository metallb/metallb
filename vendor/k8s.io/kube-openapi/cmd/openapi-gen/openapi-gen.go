/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"log"
	"os"
	"path/filepath"

	"k8s.io/gengo/args"
	"k8s.io/kube-openapi/pkg/generators"
)

const (
	outputBase         = "pkg"
	outputPackage      = "generated"
	outputBaseFilename = "openapi_generated"
)

func main() {

	// Assumes all parameters are input directories.
	// TODO: Input directories as flags; add initial flag parsing.
	testdataDirs := []string{}
	if len(os.Args) == 1 {
		log.Fatalln("Missing parameter: no input directories")
	} else {
		for _, dir := range os.Args[1:] {
			testdataDirs = append(testdataDirs, dir)
		}
	}
	log.Printf("Input directories: %s", testdataDirs)

	// Set up the gengo arguments for code generation.
	// TODO: Generated output filename as a parameter.
	generatedFilepath := filepath.Join(outputBase, outputPackage, outputBaseFilename+".go")
	log.Printf("Generated File: %s", generatedFilepath)
	arguments := args.Default()
	arguments.InputDirs = testdataDirs
	arguments.OutputBase = outputBase
	arguments.OutputPackagePath = outputPackage
	arguments.OutputFileBaseName = outputBaseFilename

	// Generates the code for the OpenAPIDefinitions.
	if err := arguments.Execute(
		generators.NameSystems(),
		generators.DefaultNameSystem(),
		generators.Packages,
	); err != nil {
		log.Fatalf("OpenAPI code generation error: %v", err)
	}
	log.Println("Code for OpenAPI definitions generated")
}
