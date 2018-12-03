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

	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/onsi/gomega"
)

// Test01UpdateIpv4Address tests correct functioning of UpdateIpv4Address().
func Test01UpdateIpv4Address(t *testing.T) {
	gomega.RegisterTestingT(t)

	oldIps := []string{"192.168.1.1/24", "192.168.1.2/24", "192.168.1.3/24"}
	update := []string{"192.168.1.4/24", "192.168.1.5/24"}

	newIps := utils.UpdateIpv4Address(oldIps, update)

	gomega.Expect(isContained(oldIps[0], newIps)).To(gomega.BeTrue())
	gomega.Expect(isContained(oldIps[1], newIps)).To(gomega.BeTrue())
	gomega.Expect(isContained(oldIps[2], newIps)).To(gomega.BeTrue())
	gomega.Expect(isContained(update[0], newIps)).To(gomega.BeTrue())
	gomega.Expect(isContained(update[1], newIps)).To(gomega.BeTrue())
}

// Test02UpdateIpv6Address tests correct functioning of UpdateIpv6Address().
func Test02UpdateIpv6Address(t *testing.T) {
	gomega.RegisterTestingT(t)

	oldIps := []string{"2001:0db8:0a0b:12f0:0000:0000:0000:0001/64", "2001:db8:0:1:1:1:1:1/64"}
	update := []string{"2001:1::1/128"}

	newIps := utils.UpdateIpv6Address(oldIps, update)

	gomega.Expect(isContained(oldIps[0], newIps)).To(gomega.BeTrue())
	gomega.Expect(isContained(oldIps[1], newIps)).To(gomega.BeTrue())
	gomega.Expect(isContained(update[0], newIps)).To(gomega.BeTrue())
}

// Test03ValidateIpv4Address tests correct functioning of ValidateIpv4Addr().
func Test03ValidateIpv4Address(t *testing.T) {
	gomega.RegisterTestingT(t)

	utils.ValidateIpv4Addr("192.168.1.1/24")

	// Program is closed if IPv4 address is not valid.
	gomega.Succeed()
}

// Test04ValidateIPv6Address tests correct functioning of ValidateIpv6Addr().
func Test04ValidateIPv6Address(t *testing.T) {
	gomega.RegisterTestingT(t)

	utils.ValidateIpv6Addr("2001:db8:0:1:1:1:1:1/64")

	// Program is closed if IPv6 address is not valid.
	gomega.Succeed()
}

// Test05ValidatePhysAddress tests correct functioning of ValidatePhyAddr().
func Test05ValidatePhysAddress(t *testing.T) {
	gomega.RegisterTestingT(t)

	utils.ValidatePhyAddr("F8:CF:E6:E8:CC:2F")

	// Program is closed if MAC address is not valid.
	gomega.Succeed()

}

func isContained(item string, list []string) bool {
	for _, listItem := range list {
		if listItem == item {
			return true
		}
	}
	return false
}
