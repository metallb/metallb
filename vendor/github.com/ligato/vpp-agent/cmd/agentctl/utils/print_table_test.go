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
	"testing"

	"github.com/buger/goterm"
	data "github.com/ligato/vpp-agent/cmd/agentctl/testing"
	"github.com/ligato/vpp-agent/cmd/agentctl/testing/then"
	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/onsi/gomega"
)

// Test01TableWithoutData tests basic output without any data (no data message).
func Test01TableWithoutData(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	table := goterm.NewTable(0, 0, 0, ' ', 0)

	table, err := etcdDump.PrintDataAsTable(table, nil, false, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	gomega.Expect(table.String()).To(gomega.BeEquivalentTo(utils.NoData))
}

// Test02TableIncorrectFilter tests filtering which is not permitted.
func Test02TableIncorrectFilter(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	table := goterm.NewTable(0, 0, 0, ' ', 0)

	filter := []string{"vpp1,vpp2", "interface"}
	table, err := etcdDump.PrintDataAsTable(table, filter, false, false)
	gomega.Expect(err).ToNot(gomega.BeNil())
}

// Test03TableHeaderData tests that the default output contains all required headers
// and does not contain those that should be missing.
func Test03TableHeaderData(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	table, err := etcdDump.PrintDataAsTable(table, nil, false, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Test flags which should be a part of the output.
	then.ContainsItems(table.String(), utils.InPkt, utils.InBytes, utils.InErrPkt, utils.OutPkt, utils.OutBytes,
		utils.OutErrPkt, utils.Drop, utils.Punt)
	// Test flags which should NOT be a part of the output.
	then.DoesNotContainItems(table.String(), utils.InNoBuf, utils.Ipv4Pkt, utils.Ipv6Pkt)
}

// Test04TableVppData verifies that the table output contains all agent labels.
func Test04TableVppData(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	table, err := etcdDump.PrintDataAsTable(table, nil, false, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// All agent labels should be in the table
	then.ContainsItems(table.String(), "vpp-1", "vpp-2", "vpp-3")
}

// Test05TableInterfaceData verifies that the table output contains all interfaces.
func Test05TableInterfaceData(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	table, err := etcdDump.PrintDataAsTable(table, nil, false, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// All interface labels should be in the table
	then.ContainsItems(table.String(), "vpp-1-interface-1", "vpp-1-interface-2", "vpp-1-interface-3", "vpp-2-interface-1",
		"vpp-2-interface-2", "vpp-2-interface-3", "vpp-3-interface-1", "vpp-3-interface-2", "vpp-3-interface-3")
}

// Test06TableShortFormat tests that 'short' output contains all required
// headers and does not contain those that should be missing.
func Test06TableShortFormat(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	table, err := etcdDump.PrintDataAsTable(table, nil, true, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Short format contains only InPackets, and OutPackets, and Drop.
	then.ContainsItems(table.String(), utils.InPkt, utils.OutPkt, utils.Drop)
	then.DoesNotContainItems(table.String(), utils.InBytes, utils.InErrPkt, utils.InNoBuf, utils.OutBytes,
		utils.OutErrPkt, utils.Ipv4Pkt, utils.Ipv6Pkt, utils.Punt)
}

// Test07TableSingleVppFilterFormat tests correct vpp filtering.
func Test07TableSingleVppFilterFormat(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	filter := []string{"vpp-1"}
	table, err := etcdDump.PrintDataAsTable(table, filter, false, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Verify that only filtered VPP is present in the table.
	then.ContainsItems(table.String(), "vpp-1")
	then.DoesNotContainItems(table.String(), "vpp-2", "vpp-3")
}

// Test08TableMultipleVppFilterFormat tests correct vpp filtering (multiple values).
func Test08TableMultipleVppFilterFormat(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	filter := []string{"vpp-1,vpp-2"}
	table, err := etcdDump.PrintDataAsTable(table, filter, false, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Verify that only filtered VPPs are present in the table.
	then.ContainsItems(table.String(), "vpp-1", "vpp-2")
	then.DoesNotContainItems(table.String(), "vpp-3")
}

// Test09TableSingleInterfaceFilterFormat tests correct interface filtering.
func Test09TableSingleInterfaceFilterFormat(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	filter := []string{"vpp-1", "vpp-1-interface-2"}
	table, err := etcdDump.PrintDataAsTable(table, filter, false, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Verify that only filtered interface in particular VPP is present in the table.
	then.ContainsItems(table.String(), "vpp-1", "vpp-1-interface-2")
	then.DoesNotContainItems(table.String(), "vpp-2", "vpp-3", "vpp-1-interface-1", "vpp-1-interface-3")
}

// Test10TableMultipleInterfacesFilterFormat tests correct vpp filtering (multiple values).
func Test10TableMultipleInterfacesFilterFormat(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	filter := []string{"vpp-1", "vpp-1-interface-2,vpp-1-interface-3"}
	table, err := etcdDump.PrintDataAsTable(table, filter, false, false)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Verify that only filtered interfaces in particular VPP are present in the table.
	then.ContainsItems(table.String(), "vpp-1", "vpp-1-interface-2", "vpp-1-interface-3")
	then.DoesNotContainItems(table.String(), "vpp-2", "vpp-3", "vpp-1-interface-1")
}

// Test11TableActiveFlag tests correct functionality of 'active' flag.
func Test11TableActiveFlag(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	table, err := etcdDump.PrintDataAsTable(table, nil, false, true)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Active flag - only non-zero columns and rows should be in the table.
	then.ContainsItems(table.String(), utils.InPkt, utils.InMissPkt, utils.OutPkt)
	then.DoesNotContainItems(table.String(), utils.InBytes, utils.InErrPkt, utils.InNoBuf, utils.OutBytes,
		utils.OutErrPkt, utils.Drop, utils.Ipv4Pkt, utils.Ipv6Pkt, utils.Punt)
	then.DoesNotContainItems(table.String(), "vpp-1-interface-2", "vpp-2-interface-2", "vpp-3-interface-2")
}

// Test12TableActiveShortFlag tests correct functionality of 'active' and 'short' flags together.
func Test12TableActiveShortFlag(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	table, err := etcdDump.PrintDataAsTable(table, nil, true, true)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Active and short flags
	then.ContainsItems(table.String(), utils.InPkt, utils.OutPkt)
	then.DoesNotContainItems(table.String(), utils.InBytes, utils.InErrPkt, utils.InMissPkt, utils.InNoBuf, utils.OutBytes,
		utils.OutErrPkt, utils.Drop, utils.Ipv4Pkt, utils.Ipv6Pkt, utils.Punt)
	then.DoesNotContainItems(table.String(), "vpp-1-interface-2", "vpp-2-interface-2", "vpp-3-interface-2")
}

// Test13TableActiveShortFlagWithFilter tests correct functionality of 'active'
// and 'short' flags together and filtering.
func Test13TableActiveShortFlagWithFilter(t *testing.T) {
	etcdDump := utils.NewEtcdDump()
	etcdDump = data.TableData()
	table := goterm.NewTable(0, 0, 1, ' ', 0)

	filter := []string{"vpp-1", "vpp-1-interface-1"}
	table, err := etcdDump.PrintDataAsTable(table, filter, true, true)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(table).ToNot(gomega.BeNil())
	// Active and short and filter vpp and interface
	then.ContainsItems(table.String(), utils.InPkt, utils.OutPkt)
	then.DoesNotContainItems(table.String(), utils.InBytes, utils.InErrPkt, utils.InMissPkt, utils.InNoBuf, utils.OutBytes,
		utils.OutErrPkt, utils.Drop, utils.Ipv4Pkt, utils.Ipv6Pkt, utils.Punt)
	then.DoesNotContainItems(table.String(), "vpp-2", "vpp-3", "vpp-1-interface-2", "vpp-1-interface-3")
}
