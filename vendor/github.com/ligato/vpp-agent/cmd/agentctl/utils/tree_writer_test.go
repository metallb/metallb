// Copyright (c) 2017 Cisco and/or its affiliates.
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

package utils_test

import (
	"fmt"
	"testing"
	"text/template"

	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/onsi/gomega"
)

// Test01TreeWriter tests functionality of FlushTree() called with tree writer.
// If the provided template is processed without any failures and
// FlushTree() doesn't throw an error, the test is successful.
func Test01TreeWriter(t *testing.T) {
	gomega.RegisterTestingT(t)

	treeWriter := utils.NewTreeWriter(1, "├─", "│ ", "└─")

	testTemplate, err := getTemplate()
	gomega.Expect(err).To(gomega.BeNil())
	err = testTemplate.Execute(treeWriter, testData())
	gomega.Expect(err).To(gomega.BeNil())

	treeWriter.FlushTree()

	// Succeed if test was not exited.
	gomega.Succeed()
}

func testData() *interfaces.Interfaces_Interface {
	return &interfaces.Interfaces_Interface{
		Name:        "test-iface",
		Enabled:     true,
		PhysAddress: "ff:ff:ff:ff:ff:ff",
	}
}

func getTemplate() (*template.Template, error) {
	testFuncMap := template.FuncMap{
		"pfx": getPrefix,
	}

	return template.New("test-template").Funcs(testFuncMap).Parse(
		"{{with .Name}}\n{{pfx 1}}Name: {{.}}{{end}}" +
			"{{with .Enabled}}\n{{pfx 1}}IsEnabled: {{.}}{{end}}" +
			"{{if .PhysAddress}}\n{{pfx 1}}PhysAddr: {{.PhysAddress}}{{end}}")
}

func getPrefix(level int) string {
	prefix := ""
	for i := 1; i <= level; i++ {
		prefix = prefix + " "
	}
	return fmt.Sprintf("%d^@%s", level, prefix)
}
