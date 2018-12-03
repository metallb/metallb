// Copyright (c) 2018 Bell Canada, Pantheon Technologies and/or its affiliates.
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

package srplugin_test

import (
	"fmt"
	"net"
	"strings"

	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
)

// SRv6Calls is fake implementation of vppcalls.SRv6Calls used for testing purposes
type SRv6Calls struct {
	failCall        interface{}
	failError       error
	localSidState   map[string]*srv6.LocalSID
	policiesState   map[string]*PolicyState
	nextPolicyIndex uint32
	policiesIdxs    map[uint32]string   // index to bsid
	steeringState   map[string]struct{} // set of steerings
}

// PolicyState is holder for one policy and its segments
type PolicyState struct {
	policy   *srv6.Policy
	segments []*srv6.PolicySegment
}

// NewSRv6Calls creates new SRv6Calls (fake implementation of vppcalls.SRv6Calls)
func NewSRv6Calls() *SRv6Calls {
	return &SRv6Calls{
		localSidState: make(map[string]*srv6.LocalSID, 0),
		policiesState: make(map[string]*PolicyState, 0),
		policiesIdxs:  make(map[uint32]string, 0),
		steeringState: make(map[string]struct{}, 0),
	}
}

// FailIn makes SRv6Calls fake to fail in given function call <funcCall> with given error <err>
func (fake *SRv6Calls) FailIn(funcCall interface{}, err error) {
	fake.failCall = funcCall
	fake.failError = err
}

// LocalSIDState provides current state of local sids as changed by calls to SRv6Calls fake
func (fake *SRv6Calls) LocalSIDState() map[string]*srv6.LocalSID {
	return fake.localSidState
}

// PoliciesState provides current state of policies as changed by calls to SRv6Calls fake
func (fake *SRv6Calls) PoliciesState() map[string]*PolicyState {
	return fake.policiesState
}

// SteeringState provides current state of steerings as changed by calls to SRv6Calls fake
func (fake *SRv6Calls) SteeringState() map[string]struct{} {
	return fake.steeringState
}

// Policy provides policy reference from PolicyState
func (ps *PolicyState) Policy() *srv6.Policy {
	return ps.policy
}

// Segments provides segments reference from PolicyState
func (ps *PolicyState) Segments() []*srv6.PolicySegment {
	return ps.segments
}

// AddLocalSidFuncCall is reference to function AddLocalSid and it used for as failure target reference for failure cases
type AddLocalSidFuncCall struct{}

// DeleteLocalSidFuncCall is reference to function DeleteLocalSid and it used for as failure target reference for failure cases
type DeleteLocalSidFuncCall struct{}

// SetEncapsSourceAddressFuncCall is reference to function SetEncapsSourceAddress and it used for as failure target reference for failure cases
type SetEncapsSourceAddressFuncCall struct{}

// AddPolicyFuncCall is reference to function AddPolicy and it used for as failure target reference for failure cases
type AddPolicyFuncCall struct{}

// DeletePolicyFuncCall is reference to function DeletePolicy and it used for as failure target reference for failure cases
type DeletePolicyFuncCall struct{}

// AddPolicySegmentFuncCall is reference to function AddPolicySegment and it used for as failure target reference for failure cases
type AddPolicySegmentFuncCall struct{}

// DeletePolicySegmentFuncCall is reference to function DeletePolicySegment and it used for as failure target reference for failure cases
type DeletePolicySegmentFuncCall struct{}

// AddSteeringFuncCall is reference to function AddSteering and it used for as failure target reference for failure cases
type AddSteeringFuncCall struct{}

// RemoveSteeringFuncCall is reference to function RemoveSteering and it used for as failure target reference for failure cases
type RemoveSteeringFuncCall struct{}

// AddLocalSid adds local sid given by <sidAddr> and <localSID> into VPP
func (fake *SRv6Calls) AddLocalSid(sidAddr net.IP, localSID *srv6.LocalSID, swIfIndex ifaceidx.SwIfIndex) error {
	if _, ok := fake.failCall.(AddLocalSidFuncCall); ok {
		return fake.failError
	}
	fake.localSidState[sidAddr.String()] = localSID
	return nil
}

// DeleteLocalSid delets local sid given by <sidAddr> in VPP
func (fake *SRv6Calls) DeleteLocalSid(sidAddr net.IP) error {
	if _, ok := fake.failCall.(DeleteLocalSidFuncCall); ok {
		return fake.failError
	}
	delete(fake.localSidState, sidAddr.String())
	return nil
}

// SetEncapsSourceAddress sets for SRv6 in VPP the source address used for encapsulated packet
func (fake *SRv6Calls) SetEncapsSourceAddress(address string) error {
	if _, ok := fake.failCall.(SetEncapsSourceAddressFuncCall); ok {
		return fake.failError
	}
	return nil
}

// AddPolicy adds SRv6 policy given by identified <bindingSid>,initial segment for policy <policySegment> and other policy settings in <policy>
func (fake *SRv6Calls) AddPolicy(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment) error {
	if _, ok := fake.failCall.(AddPolicyFuncCall); ok {
		return fake.failError
	}
	fake.policiesState[bindingSid.String()] = &PolicyState{
		policy:   policy,
		segments: []*srv6.PolicySegment{policySegment},
	}
	fake.policiesIdxs[fake.nextPolicyIndex] = bindingSid.String()
	fake.nextPolicyIndex++
	return nil
}

// DeletePolicy deletes SRv6 policy given by binding SID <bindingSid>
func (fake *SRv6Calls) DeletePolicy(bindingSid net.IP) error {
	if _, ok := fake.failCall.(DeletePolicyFuncCall); ok {
		return fake.failError
	}
	delete(fake.policiesState, bindingSid.String())
	for key, value := range fake.policiesIdxs {
		if value == bindingSid.String() {
			delete(fake.policiesIdxs, key)
		}
	}
	return nil
}

// AddPolicySegment adds segment <policySegment> to SRv6 policy <policy> that has policy BSID <bindingSid>
func (fake *SRv6Calls) AddPolicySegment(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment) error {
	if _, ok := fake.failCall.(AddPolicySegmentFuncCall); ok {
		return fake.failError
	}

	policyState, exists := fake.policiesState[bindingSid.String()]
	if !exists {
		return fmt.Errorf("policy with binding sid %v doesn't exist", bindingSid)
	}
	policyState.segments = append(policyState.segments, policySegment)
	return nil
}

// DeletePolicySegment removes segment <policySegment> (with segment index <segmentIndex>) from SRv6 policy <policy> that has policy BSID <bindingSid>
func (fake *SRv6Calls) DeletePolicySegment(bindingSid net.IP, policy *srv6.Policy, policySegment *srv6.PolicySegment,
	segmentIndex uint32) error {
	if _, ok := fake.failCall.(DeletePolicySegmentFuncCall); ok {
		return fake.failError
	}
	// TODO not checking segmentIndex (segment index is guessed by non-fake implementation and this is due to bad VPP API that doesn't return index when segment is returned)
	policyState, exists := fake.policiesState[bindingSid.String()]
	if !exists {
		return fmt.Errorf("policy with binding sid %v doesn't exist", bindingSid)
	}
	var err error
	policyState.segments, err = removeSegment(policyState.segments, policySegment)
	return err
}

func removeSegment(segments []*srv6.PolicySegment, segment *srv6.PolicySegment) ([]*srv6.PolicySegment, error) {
	for i, cur := range segments {
		if segment == cur {
			return append(segments[:i], segments[i+1:]...), nil
		}
	}
	return nil, fmt.Errorf("can't find policy segment %v in policy segments %v", segment, segments)
}

// AddSteering sets in VPP steering into SRv6 policy.
func (fake *SRv6Calls) AddSteering(steering *srv6.Steering, swIfIndex ifaceidx.SwIfIndex) error {
	if _, ok := fake.failCall.(AddSteeringFuncCall); ok {
		return fake.failError
	}
	bsidStr := steering.PolicyBsid
	if len(strings.Trim(steering.PolicyBsid, " ")) == 0 { // policy defined by index
		var exists bool
		bsidStr, exists = fake.policiesIdxs[steering.PolicyIndex]
		if !exists {
			return fmt.Errorf("can't find policy for index %v (adding steering proces)", steering.PolicyIndex)
		}
	}
	if _, exists := fake.policiesState[bsidStr]; !exists {
		return fmt.Errorf("can't find policy for bsid %v (adding steering proces)", bsidStr)
	}
	fake.steeringState[steering.PolicyBsid] = struct{}{}
	return nil
}

// RemoveSteering removes in VPP steering into SRv6 policy.
func (fake *SRv6Calls) RemoveSteering(steering *srv6.Steering, swIfIndex ifaceidx.SwIfIndex) error {
	if _, ok := fake.failCall.(RemoveSteeringFuncCall); ok {
		return fake.failError
	}
	delete(fake.steeringState, steering.PolicyBsid)
	return nil
}
