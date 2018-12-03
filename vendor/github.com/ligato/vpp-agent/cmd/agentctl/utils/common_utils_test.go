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

	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	"github.com/onsi/gomega"
)

// Test01ParseKeyAgentPrefix tests whether all parameters for ParseKey()
// functions are correct for the provided agent key.
func Test01ParseKeyAgentPrefix(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, plugStatCfgRev := utils.
		ParseKey("/vnf-agent/{agent-label}/check/status/v1/agent")

	gomega.Expect(label).To(gomega.BeEquivalentTo("{agent-label}"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(status.AgentStatusPrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo(""))
	gomega.Expect(plugStatCfgRev).To(gomega.BeEquivalentTo(status.StatusPrefix))
}

// Test02ParseKeyInterfaceConfig tests whether all parameters for ParseKey()
// functions are correct for the provided interface config key.
func Test02ParseKeyInterfaceConfig(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/{agent-label}/vpp/config/v1/interface/{interface-name}")

	gomega.Expect(label).To(gomega.BeEquivalentTo("{agent-label}"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(interfaces.Prefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("{interface-name}"))
}

// Test03ParseKeyInterfaceStatus tests whether all parameters for ParseKey()
// functions are correct for the provided interface status key.
func Test03ParseKeyInterfaceStatus(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/{agent-label}/vpp/status/v1/interface/{interface-name}")

	gomega.Expect(label).To(gomega.BeEquivalentTo("{agent-label}"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(interfaces.StatePrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("{interface-name}"))
}

// Test04ParseKeyInterfaceError tests whether all parameters for ParseKey()
// functions are correct for the provided interface error key.
func Test04ParseKeyInterfaceError(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/{agent-label}/vpp/status/v1/interface/error/{interface-name}")

	gomega.Expect(label).To(gomega.BeEquivalentTo("{agent-label}"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(interfaces.ErrorPrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("{interface-name}"))
}

// Test05ParseKeyBdConfig tests whether all parameters for ParseKey() functions
// are correct for the provided bridge domain config key.
func Test05ParseKeyBdConfig(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/{agent-label}/vpp/config/v1/bd/{bd-name}")

	gomega.Expect(label).To(gomega.BeEquivalentTo("{agent-label}"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(l2.BdPrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("{bd-name}"))
}

// Test06ParseKeyBdState tests whether all parameters for ParseKey() functions
// are correct for the provided bridge domain status key.
func Test06ParseKeyBdState(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/{agent-label}/vpp/status/v1/bd/{bd-name}")

	gomega.Expect(label).To(gomega.BeEquivalentTo("{agent-label}"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(l2.BdStatePrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("{bd-name}"))
}

// Test06ParseKeyBdState tests whether all parameters for ParseKey() functions
// are correct for the provided bridge domain error key.
func Test07ParseKeyBdError(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/{agent-label}/vpp/status/v1/bd/error/{bd-name}")

	gomega.Expect(label).To(gomega.BeEquivalentTo("{agent-label}"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(l2.BdErrPrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("{bd-name}"))
}

// Test06ParseKeyBdState tests whether all parameters for ParseKey() functions
// are correct for the provided fib table key.
func Test08ParseKeyFib(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/{agent-label}/vpp/config/v1/bd/{bd-label}/fib/{mac-address}")

	gomega.Expect(label).To(gomega.BeEquivalentTo("{agent-label}"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(l2.FibPrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("{mac-address}"))
}

// Test09ParseKeyRoute tests whether all parameters for ParseKey() functions
// are correct for the provided route key.
func Test09ParseKeyRoute(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/agent1/vpp/config/v1/vrf/vrf1/fib/192.168.1.0/24/192.168.2.1")

	gomega.Expect(label).To(gomega.BeEquivalentTo("agent1"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(l3.RoutesPrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("192.168.1.0/24/192.168.2.1"))

	label, dataType, params, _ = utils.
		ParseKey("/vnf-agent/agent2/vpp/config/v1/vrf/vrf2/fib/2001:db8:abcd:0012::0/64/2001:db8::1")

	gomega.Expect(label).To(gomega.BeEquivalentTo("agent2"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(l3.RoutesPrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("2001:db8:abcd:0012::0/64/2001:db8::1"))
}

// Test10ParseKeyVrf tests whether all parameters for ParseKey() functions
// are correct for the provided vrf key.
func Test10ParseKeyVrf(t *testing.T) {
	gomega.RegisterTestingT(t)
	label, dataType, params, _ := utils.
		ParseKey("/vnf-agent/agent1/vpp/config/v1/vrf/vrf1")

	gomega.Expect(label).To(gomega.BeEquivalentTo("agent1"))
	gomega.Expect(dataType).To(gomega.BeEquivalentTo(l3.VrfPrefix))
	gomega.Expect(params).To(gomega.BeEquivalentTo("vrf1"))
}
