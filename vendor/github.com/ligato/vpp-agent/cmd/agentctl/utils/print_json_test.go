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
	"strings"
	"testing"

	data "github.com/ligato/vpp-agent/cmd/agentctl/testing"
	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/onsi/gomega"
)

// Test01VppPrintJsonData tests VPPs and config presence in the output and the presence
// of statistics data (the header in every interface and the data flags in active interfaces).
func Test01VppPrintJsonData(t *testing.T) {
	gomega.RegisterTestingT(t)
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.JSONData()

	result, _ := etcdDump.PrintDataAsJSON(nil)
	gomega.Expect(result).ToNot(gomega.BeNil())

	output := result.String()

	// Check Vpp flags.
	gomega.Expect(strings.Contains(output, "vpp1")).To(gomega.BeTrue())
	gomega.Expect(strings.Contains(output, "vpp2")).To(gomega.BeTrue())

	// Check interface config (should be one per vpp).
	gomega.Expect(strings.Contains(output, utils.IfConfig)).To(gomega.BeTrue())
	gomega.Expect(strings.Count(output, utils.IfConfig)).To(gomega.BeEquivalentTo(2))

	// Check interface state (should be once per vpp).
	gomega.Expect(strings.Contains(output, utils.IfState)).To(gomega.BeTrue())
	gomega.Expect(strings.Count(output, utils.IfState)).To(gomega.BeEquivalentTo(2))

	// Check bridge domain config (should be once per vpp).
	gomega.Expect(strings.Contains(output, utils.BdConfig)).To(gomega.BeTrue())
	gomega.Expect(strings.Count(output, utils.BdConfig)).To(gomega.BeEquivalentTo(2))

	// Check bridge domain state (should be once per vpp).
	gomega.Expect(strings.Contains(output, utils.BdState)).To(gomega.BeTrue())
	gomega.Expect(strings.Count(output, utils.BdState)).To(gomega.BeEquivalentTo(2))

	// Check L2 fib table (should be once per vpp).
	gomega.Expect(strings.Contains(output, utils.L2FibConfig)).To(gomega.BeTrue())
	gomega.Expect(strings.Count(output, utils.L2FibConfig)).To(gomega.BeEquivalentTo(2))

	// Check L3 fib table (should be once per vpp).
	gomega.Expect(strings.Contains(output, utils.L3FibConfig)).To(gomega.BeTrue())
	gomega.Expect(strings.Count(output, utils.L3FibConfig)).To(gomega.BeEquivalentTo(2))

	// Test statistics presence (including empty).
	gomega.Expect(strings.Contains(output, "statistics")).To(gomega.BeTrue())
	gomega.Expect(strings.Count(output, "statistics")).To(gomega.BeEquivalentTo(2)) // Interface count

	// Test statistics data.
	dataFlags := []string{"in_packets", "out_packets", "in_miss_packets"}
	for _, flag := range dataFlags {
		gomega.Expect(strings.Contains(output, flag)).To(gomega.BeTrue())
		// Interfaces with statistics data
		gomega.Expect(strings.Count(output, flag)).To(gomega.BeEquivalentTo(2))
	}
}

// Test02VppPrintJsonFilteredData tests VPPs and config presence in the output.
// The result is filtered.
func Test02VppPrintJsonFilteredData(t *testing.T) {
	gomega.RegisterTestingT(t)
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.JSONData()

	// Add filter to vpp2 (vpp1 data should be ignored) and non existing vpp3 filter
	// (should be ignored as well).
	result, _ := etcdDump.PrintDataAsJSON([]string{"vpp2", "vpp3"})
	gomega.Expect(result).ToNot(gomega.BeNil())

	output := result.String()

	// Check Vpp flags.
	gomega.Expect(strings.Contains(output, "vpp1")).To(gomega.BeFalse())
	gomega.Expect(strings.Contains(output, "vpp2")).To(gomega.BeTrue())
	// There should be nothing as 'vpp3'.
	gomega.Expect(strings.Contains(output, "vpp3")).To(gomega.BeFalse())

	// Test statistics data.
	dataFlags := []string{"in_packets", "out_packets", "in_miss_packets"}
	for _, flag := range dataFlags {
		gomega.Expect(strings.Contains(output, flag)).To(gomega.BeTrue())
		// Interfaces with statistics data
		gomega.Expect(strings.Count(output, flag)).To(gomega.BeEquivalentTo(1))
	}
}

// Test04PrintJsonMetadata tests presence of metadata in the output in case
// 'showEtcd' switch is set to true. The metadata should be present on every interface.
func Test04PrintJsonMetadata(t *testing.T) {
	gomega.RegisterTestingT(t)
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.JSONData()

	result, _ := etcdDump.PrintDataAsJSON(nil)
	gomega.Expect(result).ToNot(gomega.BeNil())
	output := result.String()

	gomega.Expect(strings.Contains(output, "Keys")).To(gomega.BeTrue())
	count := strings.Count(output, "Keys")
	gomega.Expect(count).To(gomega.BeEquivalentTo(12))
}

// Test05PrintJsonEmptyBuffer tests empty buffer using non-existing vpp filter.
func Test05PrintJsonEmptyBuffer(t *testing.T) {
	gomega.RegisterTestingT(t)
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.JSONData()

	result, err := etcdDump.PrintDataAsJSON([]string{"filter-all"})
	gomega.Expect(err).To(gomega.BeNil())
	gomega.ContainSubstring("No data to display", result.String())
}
