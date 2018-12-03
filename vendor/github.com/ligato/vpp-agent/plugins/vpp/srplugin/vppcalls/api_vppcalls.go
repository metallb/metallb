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

package vppcalls

import (
	"net"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
)

// SRv6VppAPI is API boundary for vppcall package access, introduced to properly test code dependent on vppcalls package
type SRv6VppAPI interface {
	SRv6VPPWrite
	SRv6VPPRead
}

// SRv6VPPWrite provides write methods for segment routing
type SRv6VPPWrite interface {
	// AddLocalSid adds local sid given by <sidAddr> and <localSID> into VPP
	AddLocalSid(sidAddr net.IP, localSID *srv6.LocalSID, swIfIndex ifaceidx.SwIfIndex) error
	// DeleteLocalSid delets local sid given by <sidAddr> in VPP
	DeleteLocalSid(sidAddr net.IP) error
	// SetEncapsSourceAddress sets for SRv6 in VPP the source address used for encapsulated packet
	SetEncapsSourceAddress(address string) error
	// AddPolicy adds SRv6 policy given by identified <bindingSid>,initial segment for policy <policySegment> and other policy settings in <policy>
	AddPolicy(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment) error
	// DeletePolicy deletes SRv6 policy given by binding SID <bindingSid>
	DeletePolicy(bindingSid net.IP) error
	// AddPolicySegment adds segment <policySegment> to SRv6 policy <policy> that has policy BSID <bindingSid>
	AddPolicySegment(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment) error
	// DeletePolicySegment removes segment <policySegment> (with segment index <segmentIndex>) from SRv6 policy <policy> that has policy BSID <bindingSid>
	DeletePolicySegment(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment, segmentIndex uint32) error
	// AddSteering sets in VPP steering into SRv6 policy.
	AddSteering(steering *srv6.Steering, swIfIndex ifaceidx.SwIfIndex) error
	// RemoveSteering removes in VPP steering into SRv6 policy.
	RemoveSteering(steering *srv6.Steering, swIfIndex ifaceidx.SwIfIndex) error
}

// SRv6VPPRead provides read methods for segment routing
type SRv6VPPRead interface {
	// TODO: implement dump methods
}

// SRv6VppHandler is accessor for SRv6-related vppcalls methods
type SRv6VppHandler struct {
	log          logging.Logger
	callsChannel govppapi.Channel
}

// NewSRv6VppHandler creates new instance of SRv6 vppcalls handler
func NewSRv6VppHandler(vppChan govppapi.Channel, log logging.Logger) *SRv6VppHandler {
	return &SRv6VppHandler{
		callsChannel: vppChan,
		log:          log,
	}
}
