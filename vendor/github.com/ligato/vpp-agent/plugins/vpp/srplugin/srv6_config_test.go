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
	"testing"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
	"github.com/ligato/vpp-agent/plugins/vpp/srplugin"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

// TODO add more tests: cover remove/modify for localsids/policies/policy segments/steering
// TODO add more tests: cover delayed configuration

var (
	sidA = sid("A::")
	sidB = sid("B::")
	sidC = sid("C::")
	sidD = sid("D::")
)

const (
	errorMessage = "this is test error"
	segmentName1 = "segmentName1"
	segmentName2 = "segmentName2"
	steeringName = "steeringName"
)

// TestAddLocalSID tests all cases where configurator's AddLocalSID is used (except of complicated cases involving multiple configurator methods)
func TestAddLocalSID(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name     string
		Verify   func(srv6.SID, *srv6.LocalSID, error, *SRv6Calls)
		FailIn   interface{}
		FailWith error
	}{
		{
			Name: "simple addition of local sid",
			Verify: func(sid srv6.SID, data *srv6.LocalSID, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				state := fakeVPPCalls.LocalSIDState()
				recordedData, exists := state[sid.String()]
				Expect(exists).To(BeTrue())
				Expect(recordedData).To(Equal(data))
			},
		},
		{
			Name:     "failure propagation from VPPCall's AddLocalSid",
			FailIn:   AddLocalSidFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(sid srv6.SID, data *srv6.LocalSID, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			sid := sidA
			data := localSID(sid)
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			err := configurator.AddLocalSID(data)
			td.Verify(sid, data, err, fakeVPPCalls)
		})
	}
}

// TestDeleteLocalSID tests all cases where configurator's DeleteLocalSID is used (except of complicated cases involving multiple configurator methods)
func TestDeleteLocalSID(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name     string
		Verify   func(error, *SRv6Calls)
		FailIn   interface{}
		FailWith error
	}{
		{
			Name: "simple deletion of local sid",
			Verify: func(err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.LocalSIDState()).To(BeEmpty())
			},
		},
		{
			Name:     "failure propagation from VPPCall's DeleteLocalSid",
			FailIn:   DeleteLocalSidFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			sid := sidA
			data := localSID(sid)
			configurator.AddLocalSID(data)
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested method and verify
			err := configurator.DeleteLocalSID(data)
			td.Verify(err, fakeVPPCalls)
		})
	}
}

// TestModifyLocalSID tests all cases where configurator's ModifyLocalSID is used (except of complicated cases involving multiple configurator methods)
func TestModifyLocalSID(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name     string
		Verify   func(srv6.SID, *srv6.LocalSID, *srv6.LocalSID, error, *SRv6Calls)
		FailIn   interface{}
		FailWith error
	}{
		{
			Name: "simple modify of local sid",
			Verify: func(sid srv6.SID, data *srv6.LocalSID, prevData *srv6.LocalSID, err error, fakeVPPCalls *SRv6Calls) {
				state := fakeVPPCalls.LocalSIDState()
				recordedData, exists := state[sid.String()]
				Expect(exists).To(BeTrue())
				Expect(recordedData).To(Equal(data))
			},
		},
		{
			Name:     "failure propagation from VPPCall's AddLocalSid",
			FailIn:   AddLocalSidFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(sid srv6.SID, data *srv6.LocalSID, prevData *srv6.LocalSID, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
		{
			Name:     "failure propagation from VPPCall's DeleteLocalSid",
			FailIn:   DeleteLocalSidFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(sid srv6.SID, data *srv6.LocalSID, prevData *srv6.LocalSID, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup and teardown
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// data
			sid := sidA
			prevData := &srv6.LocalSID{
				Sid:        sid.String(),
				FibTableId: 0,
				BaseEndFunction: &srv6.LocalSID_End{
					Psp: true,
				},
			}
			data := &srv6.LocalSID{
				Sid:        sid.String(),
				FibTableId: 1,
				BaseEndFunction: &srv6.LocalSID_End{
					Psp: false,
				},
			}
			// state and failure setup
			configurator.AddLocalSID(prevData)
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested method and verify
			err := configurator.ModifyLocalSID(data, prevData)
			td.Verify(sid, data, prevData, err, fakeVPPCalls)
		})
	}
}

// TestAddPolicy tests all cases where configurator's AddPolicy and AddPolicySegment is used (except of complicated cases involving other configurator methods)
func TestAddPolicy(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name                              string
		VerifyAfterAddPolicy              func(*srv6.Policy, *srv6.PolicySegment, *srv6.PolicySegment, error, *SRv6Calls)
		VerifyAfterFirstAddPolicySegment  func(*srv6.Policy, *srv6.PolicySegment, *srv6.PolicySegment, error, *SRv6Calls)
		VerifyAfterSecondAddPolicySegment func(*srv6.Policy, *srv6.PolicySegment, *srv6.PolicySegment, error, *SRv6Calls)
		FailIn                            interface{}
		FailWith                          error
		SetPolicySegmentsFirst            bool
	}{
		{
			Name: "add policy and add 2 segment", // handling of first segment is special -> adding 2 segments
			VerifyAfterAddPolicy: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.PoliciesState()).To(BeEmpty())
			},
			VerifyAfterFirstAddPolicySegment: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				verifyOnePolicyWithSegments(fakeVPPCalls, policy, segment)
			},
			VerifyAfterSecondAddPolicySegment: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				verifyOnePolicyWithSegments(fakeVPPCalls, policy, segment, segment2)
			},
		},
		{
			Name: "add 2 segments to nonexisting policy and add policy", // handling of first segment is special -> adding 2 segments
			SetPolicySegmentsFirst: true,
			VerifyAfterFirstAddPolicySegment: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.PoliciesState()).To(HaveLen(0))
			},
			VerifyAfterSecondAddPolicySegment: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.PoliciesState()).To(HaveLen(0))
			},
			VerifyAfterAddPolicy: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				verifyOnePolicyWithSegments(fakeVPPCalls, policy, segment, segment2)
			},
		},
		{
			Name:     "failure propagation from VPPCall's AddPolicy",
			FailIn:   AddPolicyFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			VerifyAfterFirstAddPolicySegment: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
		{
			Name:                   "failure propagation from VPPCall's AddPolicySegment",
			FailIn:                 AddPolicySegmentFuncCall{},
			FailWith:               fmt.Errorf(errorMessage),
			SetPolicySegmentsFirst: true,
			VerifyAfterAddPolicy: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup and teardown
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// Data
			bsid := sidA
			policy := policy(bsid)
			segment := policySegment(bsid, 1, sidB, sidC, sidD)
			segment2 := policySegment(bsid, 1, sidA, sidB, sidC)
			// failure setup
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested methods and verification after each of them
			if td.SetPolicySegmentsFirst {
				err := configurator.AddPolicySegment(segmentName1, segment)
				if td.VerifyAfterFirstAddPolicySegment != nil {
					td.VerifyAfterFirstAddPolicySegment(policy, segment, segment2, err, fakeVPPCalls)
				}
				err = configurator.AddPolicySegment(segmentName2, segment2)
				if td.VerifyAfterSecondAddPolicySegment != nil {
					td.VerifyAfterSecondAddPolicySegment(policy, segment, segment2, err, fakeVPPCalls)
				}
				err = configurator.AddPolicy(policy)
				if td.VerifyAfterAddPolicy != nil {
					td.VerifyAfterAddPolicy(policy, segment, segment2, err, fakeVPPCalls)
				}
			} else {
				err := configurator.AddPolicy(policy)
				if td.VerifyAfterAddPolicy != nil {
					td.VerifyAfterAddPolicy(policy, segment, segment2, err, fakeVPPCalls)
				}
				err = configurator.AddPolicySegment(segmentName1, segment)
				if td.VerifyAfterFirstAddPolicySegment != nil {
					td.VerifyAfterFirstAddPolicySegment(policy, segment, segment2, err, fakeVPPCalls)
				}
				err = configurator.AddPolicySegment(segmentName2, segment2)
				if td.VerifyAfterSecondAddPolicySegment != nil {
					td.VerifyAfterSecondAddPolicySegment(policy, segment, segment2, err, fakeVPPCalls)
				}
			}
		})
	}
}

// TestDeletePolicy tests all cases where configurator's DeletePolicy and DeletePolicySegment is used (except of complicated cases involving other configurator methods)
func TestDeletePolicy(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name                                 string
		VerifyAfterRemovePolicy              func(*srv6.Policy, *srv6.PolicySegment, *srv6.PolicySegment, error, *SRv6Calls)
		VerifyAfterFirstRemovePolicySegment  func(*srv6.Policy, *srv6.PolicySegment, *srv6.PolicySegment, error, *SRv6Calls)
		VerifyAfterSecondRemovePolicySegment func(*srv6.Policy, *srv6.PolicySegment, *srv6.PolicySegment, error, *SRv6Calls)
		FailIn                               interface{}
		FailWith                             error
		RemovePoliceSegment                  bool
	}{
		{
			Name: "remove policy (without removing segments)",
			VerifyAfterRemovePolicy: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.PoliciesState()).To(BeEmpty())
			},
		},
		{
			Name:                "remove segments and remove policy",
			RemovePoliceSegment: true,
			VerifyAfterFirstRemovePolicySegment: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.PoliciesState()).ToNot(BeEmpty())
			},
			VerifyAfterSecondRemovePolicySegment: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.PoliciesState()).ToNot(BeEmpty())
			},
			VerifyAfterRemovePolicy: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.PoliciesState()).To(BeEmpty())
			},
		},
		{
			Name:     "failure propagation from VPPCall's DeletePolicy",
			FailIn:   DeletePolicyFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			VerifyAfterRemovePolicy: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
		{
			Name:                "failure propagation from VPPCall's DeletePolicySegment",
			FailIn:              DeletePolicySegmentFuncCall{},
			FailWith:            fmt.Errorf(errorMessage),
			RemovePoliceSegment: true,
			VerifyAfterFirstRemovePolicySegment: func(policy *srv6.Policy, segment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup and teardown
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// Data
			bsid := sidA
			policy := policy(bsid)
			segment := policySegment(bsid, 1, sidB, sidC, sidD)
			segment2 := policySegment(bsid, 1, sidA, sidB, sidC)
			configurator.AddPolicy(policy)
			configurator.AddPolicySegment(segmentName1, segment)
			configurator.AddPolicySegment(segmentName2, segment2) // handling of first segment is special -> adding 2 segments
			// failure setup
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested methods and verification after each of them
			if td.RemovePoliceSegment {
				err := configurator.RemovePolicySegment(segmentName1, segment)
				if td.VerifyAfterFirstRemovePolicySegment != nil {
					td.VerifyAfterFirstRemovePolicySegment(policy, segment, segment2, err, fakeVPPCalls)
				}
				err = configurator.RemovePolicySegment(segmentName2, segment2)
				if td.VerifyAfterSecondRemovePolicySegment != nil {
					td.VerifyAfterSecondRemovePolicySegment(policy, segment, segment2, err, fakeVPPCalls)
				}
			}
			err := configurator.RemovePolicy(policy)
			if td.VerifyAfterRemovePolicy != nil {
				td.VerifyAfterRemovePolicy(policy, segment, segment2, err, fakeVPPCalls)
			}
		})
	}
}

// TestModifyPolicy tests all cases where configurator's ModifyPolicy is used (except of complicated cases involving other configurator methods)
func TestModifyPolicy(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name     string
		Verify   func(*srv6.Policy, *srv6.Policy, *srv6.PolicySegment, error, *SRv6Calls)
		FailIn   interface{}
		FailWith error
	}{
		{
			Name: "policy attributes modification",
			Verify: func(policy *srv6.Policy, prevPolicy *srv6.Policy, segment *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				verifyOnePolicyWithSegments(fakeVPPCalls, policy, segment)
			},
		},
		{
			Name:     "failure propagation from VPPCall's AddPolicy",
			FailIn:   AddPolicyFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(policy *srv6.Policy, prevPolicy *srv6.Policy, segment *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
		{
			Name:     "failure propagation from VPPCall's DeletePolicy",
			FailIn:   DeletePolicyFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(policy *srv6.Policy, prevPolicy *srv6.Policy, segment *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup and teardown
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// Data
			bsid := sidA
			prevPolicy := &srv6.Policy{
				Bsid:             bsid.String(),
				FibTableId:       0,
				SprayBehaviour:   true,
				SrhEncapsulation: true,
			}
			policy := &srv6.Policy{
				Bsid:             bsid.String(),
				FibTableId:       1,
				SprayBehaviour:   false,
				SrhEncapsulation: false,
			}
			segment := policySegment(bsid, 1, sidB, sidC, sidD)
			configurator.AddPolicy(prevPolicy)
			configurator.AddPolicySegment(segmentName1, segment)
			// failure setup
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested methods and verification after each of them
			err := configurator.ModifyPolicy(policy, prevPolicy)
			if td.Verify != nil {
				td.Verify(policy, prevPolicy, segment, err, fakeVPPCalls)
			}
		})
	}
}

// TestModifyPolicySegment tests all cases where configurator's ModifyPolicySegment is used (except of complicated cases involving other configurator methods)
func TestModifyPolicySegment(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name           string
		Verify         func(*srv6.Policy, *srv6.PolicySegment, *srv6.PolicySegment, *srv6.PolicySegment, error, *SRv6Calls)
		FailIn         interface{}
		FailWith       error
		OnlyOneSegment bool
	}{
		{
			Name: "policy segment modification (non-last segment)", // last segment is handled differently
			Verify: func(policy *srv6.Policy, segment *srv6.PolicySegment, prevSegment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				verifyOnePolicyWithSegments(fakeVPPCalls, policy, segment2, segment)
			},
		},
		{
			Name:           "policy segment modification (last segment)", // last segment is handled differently
			OnlyOneSegment: true,
			Verify: func(policy *srv6.Policy, segment *srv6.PolicySegment, prevSegment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				verifyOnePolicyWithSegments(fakeVPPCalls, policy, segment)
			},
		},
		{
			Name:           "failure propagation from VPPCall's AddPolicy",
			OnlyOneSegment: true,
			FailIn:         AddPolicyFuncCall{},
			FailWith:       fmt.Errorf(errorMessage),
			Verify: func(policy *srv6.Policy, segment *srv6.PolicySegment, prevSegment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
		{
			Name:           "failure propagation from VPPCall's DeletePolicy",
			OnlyOneSegment: true,
			FailIn:         DeletePolicyFuncCall{},
			FailWith:       fmt.Errorf(errorMessage),
			Verify: func(policy *srv6.Policy, segment *srv6.PolicySegment, prevSegment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
		{
			Name:     "failure propagation from VPPCall's DeletePolicySegment",
			FailIn:   DeletePolicySegmentFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(policy *srv6.Policy, segment *srv6.PolicySegment, prevSegment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
		{
			Name:     "failure propagation from VPPCall's AddPolicySegment",
			FailIn:   AddPolicySegmentFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(policy *srv6.Policy, segment *srv6.PolicySegment, prevSegment *srv6.PolicySegment, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup and teardown
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// Data
			bsid := sidA
			policy := policy(bsid)
			prevSegment := policySegment(bsid, 0, sidA, sidB, sidC)
			segment := policySegment(bsid, 1, sidB, sidC, sidD)
			segment2 := policySegment(bsid, 2, sidC, sidD, sidA)
			configurator.AddPolicy(policy)
			configurator.AddPolicySegment(segmentName1, prevSegment)
			if !td.OnlyOneSegment {
				configurator.AddPolicySegment(segmentName2, segment2)
			}
			// failure setup
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested methods and verification after each of them
			err := configurator.ModifyPolicySegment(segmentName1, segment, prevSegment)
			if td.Verify != nil {
				td.Verify(policy, segment, prevSegment, segment2, err, fakeVPPCalls)
			}
		})
	}
}

// TestFillingAlreadyCreatedSegmentEmptyPolicy tests cases where policy is created, but cleaned off segments and
// new segment is added. This test is testing special case around last segment in policy.
func TestFillingAlreadyCreatedSegmentEmptyPolicy(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name     string
		Verify   func(*srv6.Policy, *srv6.PolicySegment, error, *SRv6Calls)
		FailIn   interface{}
		FailWith error
	}{
		{
			Name: "all segments removal and adding new onw", // last segment is handled differently
			Verify: func(policy *srv6.Policy, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				verifyOnePolicyWithSegments(fakeVPPCalls, policy, segment2)
			},
		},
		{
			Name:     "failure propagation from VPPCall's DeletePolicy",
			FailIn:   DeletePolicyFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(policy *srv6.Policy, segment2 *srv6.PolicySegment, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}
	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup and teardown
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// Data
			bsid := sidA
			policy := policy(bsid)
			segment := policySegment(bsid, 0, sidA, sidB, sidC)
			segment2 := policySegment(bsid, 1, sidB, sidC, sidD)
			// case building
			Expect(configurator.AddPolicy(policy)).To(BeNil())
			Expect(configurator.AddPolicySegment(segmentName1, segment)).To(BeNil())
			Expect(configurator.RemovePolicySegment(segmentName1, segment)).To(BeNil())
			// failure setup
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested methods and verification after each of them
			err := configurator.AddPolicySegment(segmentName2, segment2)
			td.Verify(policy, segment2, err, fakeVPPCalls)
		})
	}
}

// TestAddSteering tests all cases where configurator's AddSteering is used
func TestAddSteering(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name                   string
		VerifyAfterAddPolicy   func(*srv6.Steering, *SRv6Calls)
		VerifyAfterAddSteering func(*srv6.Steering, error, *SRv6Calls)
		FailIn                 interface{}
		FailWith               error
		ReferencePolicyByIndex bool
		CreatePolicyAfter      bool
		CustomSteeringData     *srv6.Steering
	}{
		{
			Name: "addition of steering (with already existing BSID-referenced policy)",
			VerifyAfterAddPolicy: func(steering *srv6.Steering, fakeVPPCalls *SRv6Calls) {
				Expect(fakeVPPCalls.SteeringState()).To(BeEmpty())
			},
			VerifyAfterAddSteering: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				state := fakeVPPCalls.SteeringState()
				_, exists := state[steering.PolicyBsid]
				Expect(exists).To(BeTrue())
			},
		},
		{
			Name:              "addition of steering (with BSID-referenced policy added later)",
			CreatePolicyAfter: true,
			VerifyAfterAddSteering: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.SteeringState()).To(BeEmpty())
			},
			VerifyAfterAddPolicy: func(steering *srv6.Steering, fakeVPPCalls *SRv6Calls) {
				state := fakeVPPCalls.SteeringState()
				_, exists := state[steering.PolicyBsid]
				Expect(exists).To(BeTrue())
			},
		},
		{
			Name: "addition of steering (with already existing Index-referenced policy)",
			ReferencePolicyByIndex: true,
			VerifyAfterAddPolicy: func(steering *srv6.Steering, fakeVPPCalls *SRv6Calls) {
				Expect(fakeVPPCalls.SteeringState()).To(BeEmpty())
			},
			VerifyAfterAddSteering: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				state := fakeVPPCalls.SteeringState()
				_, exists := state[steering.PolicyBsid]
				Expect(exists).To(BeTrue())
			},
		},
		{
			Name: "addition of steering (with Index-referenced policy added later)",
			ReferencePolicyByIndex: true,
			CreatePolicyAfter:      true,
			VerifyAfterAddSteering: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.SteeringState()).To(BeEmpty())
			},
			VerifyAfterAddPolicy: func(steering *srv6.Steering, fakeVPPCalls *SRv6Calls) {
				state := fakeVPPCalls.SteeringState()
				_, exists := state[steering.PolicyBsid]
				Expect(exists).To(BeTrue())
			},
		},
		{
			Name:               "invalid BSID as policy reference",
			CustomSteeringData: steeringWithPolicyBsidRef("XYZ"), // valid binding sid = valid IPv6
			VerifyAfterAddSteering: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
			},
		},
		{
			Name:     "failure propagation from VPPCall's AddSteering",
			FailIn:   AddSteeringFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			VerifyAfterAddSteering: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// data
			bsid := sidA
			policy := policy(bsid)
			segment := policySegment(bsid, 1, sidB, sidC, sidD)
			steering := steeringWithPolicyBsidRef(policy.Bsid)
			if td.ReferencePolicyByIndex {
				steering = steeringWithPolicyIndexRef(0)
			}
			if td.CustomSteeringData != nil {
				steering = td.CustomSteeringData
			}
			// failure setup
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// case building
			if td.CreatePolicyAfter {
				err := configurator.AddSteering(steeringName, steering)
				if td.VerifyAfterAddSteering != nil {
					td.VerifyAfterAddSteering(steering, err, fakeVPPCalls)
				}
				configurator.AddPolicy(policy)
				configurator.AddPolicySegment(segmentName1, segment)
				if td.VerifyAfterAddPolicy != nil {
					td.VerifyAfterAddPolicy(steering, fakeVPPCalls)
				}
			} else {
				configurator.AddPolicy(policy)
				configurator.AddPolicySegment(segmentName1, segment)
				if td.VerifyAfterAddPolicy != nil {
					td.VerifyAfterAddPolicy(steering, fakeVPPCalls)
				}
				err := configurator.AddSteering(steeringName, steering)
				if td.VerifyAfterAddSteering != nil {
					td.VerifyAfterAddSteering(steering, err, fakeVPPCalls)
				}
			}
		})
	}
}

// TestRemoveSteering tests all cases where configurator's RemoveSteering is used (except of complicated cases involving multiple configurator methods)
func TestRemoveSteering(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name     string
		Verify   func(error, *SRv6Calls)
		FailIn   interface{}
		FailWith error
	}{
		{
			Name: "simple steering removal",
			Verify: func(err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				Expect(fakeVPPCalls.SteeringState()).To(BeEmpty())
			},
		},
		{
			Name:     "failure propagation from VPPCall's RemoveSteering",
			FailIn:   RemoveSteeringFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// data
			bsid := sidA
			policy := policy(bsid)
			segment := policySegment(bsid, 1, sidB, sidC, sidD)
			steering := steeringWithPolicyBsidRef(policy.Bsid)
			// case building
			configurator.AddPolicy(policy)
			configurator.AddPolicySegment(segmentName1, segment)
			configurator.AddSteering(steeringName, steering)
			// failure setup
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested method and verify
			err := configurator.RemoveSteering(steeringName, steering)
			td.Verify(err, fakeVPPCalls)
		})
	}
}

// TestModifySteering tests all cases where configurator's ModifySteering is used (except of complicated cases involving multiple configurator methods)
func TestModifySteering(t *testing.T) {
	// Prepare different cases
	cases := []struct {
		Name     string
		Verify   func(*srv6.Steering, error, *SRv6Calls)
		FailIn   interface{}
		FailWith error
	}{
		{
			Name: "simple modification of steering",
			Verify: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).To(BeNil())
				state := fakeVPPCalls.SteeringState()
				_, exists := state[steering.PolicyBsid]
				Expect(exists).To(BeTrue())
			},
		},
		{
			Name:     "failure propagation from VPPCall's AddSteering",
			FailIn:   AddSteeringFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
		{
			Name:     "failure propagation from VPPCall's RemoveSteering",
			FailIn:   RemoveSteeringFuncCall{},
			FailWith: fmt.Errorf(errorMessage),
			Verify: func(steering *srv6.Steering, err error, fakeVPPCalls *SRv6Calls) {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			},
		},
	}

	// Run all cases
	for _, td := range cases {
		t.Run(td.Name, func(t *testing.T) {
			// setup and teardown
			configurator, fakeVPPCalls, connection := srv6TestSetup(t)
			defer srv6TestTeardown(connection, configurator)
			// data
			bsid := sidA
			policy := policy(bsid)
			segment := policySegment(bsid, 1, sidB, sidC, sidD)
			prevData := &srv6.Steering{
				PolicyBsid: bsid.String(),
				L3Traffic: &srv6.Steering_L3Traffic{
					FibTableId:    0,
					PrefixAddress: "A::",
				},
			}
			data := &srv6.Steering{
				PolicyBsid: bsid.String(),
				L3Traffic: &srv6.Steering_L3Traffic{
					FibTableId:    1,
					PrefixAddress: "B::",
				},
			}
			// case building
			configurator.AddPolicy(policy)
			configurator.AddPolicySegment(segmentName1, segment)
			configurator.AddSteering(steeringName, prevData)
			// failure setup
			if td.FailIn != nil {
				fakeVPPCalls.FailIn(td.FailIn, td.FailWith)
			}
			// run tested method and verify
			err := configurator.ModifySteering(steeringName, data, prevData)
			td.Verify(data, err, fakeVPPCalls)
		})
	}
}

/* Srv6 Test Setup */

func srv6TestSetup(t *testing.T) (*srplugin.SRv6Configurator, *SRv6Calls, *core.Connection) {
	RegisterTestingT(t)
	// connection
	ctx := &vppcallmock.TestCtx{
		MockVpp: mock.NewVppAdapter(),
	}
	connection, err := core.Connect(ctx.MockVpp)
	Expect(err).ShouldNot(HaveOccurred())
	// Logger
	log := logging.ForPlugin("test-log")
	log.SetLevel(logging.DebugLevel)
	// Interface index from default plugins
	swIndex := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "sw_if_indexes", ifaceidx.IndexMetadata))
	// Configurator
	fakeVPPCalls := NewSRv6Calls()
	configurator := &srplugin.SRv6Configurator{}
	err = configurator.Init(log, connection, swIndex, fakeVPPCalls)
	Expect(err).To(BeNil())

	return configurator, fakeVPPCalls, connection
}

/* Srv6 Test Teardown */

func srv6TestTeardown(connection *core.Connection, plugin *srplugin.SRv6Configurator) {
	connection.Disconnect()
	err := plugin.Close()
	Expect(err).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

func verifyOnePolicyWithSegments(fakeVPPCalls *SRv6Calls, policy *srv6.Policy, segments ...*srv6.PolicySegment) {
	policiesState := fakeVPPCalls.PoliciesState()
	Expect(policiesState).To(HaveLen(1))
	policyState, exists := policiesState[policy.Bsid]
	Expect(exists).To(BeTrue())
	Expect(policyState.Policy()).To(Equal(policy))
	Expect(policyState.Segments()).To(HaveLen(len(segments)))
	intersection := 0
	for _, actualSegment := range policyState.Segments() {
		for _, expectedSegment := range segments {
			if actualSegment == expectedSegment {
				intersection++
			}
		}
	}
	Expect(intersection).To(BeEquivalentTo(len(segments)), "policy have exactly the same segments as expected")
}

func sid(str string) srv6.SID {
	bsid, err := srplugin.ParseIPv6(str)
	if err != nil {
		panic(fmt.Sprintf("can't parse %q into SRv6 BSID (IPv6 address)", str))
	}
	return bsid
}

func localSID(sid srv6.SID) *srv6.LocalSID {
	return &srv6.LocalSID{
		Sid:        sid.String(),
		FibTableId: 0,
		BaseEndFunction: &srv6.LocalSID_End{
			Psp: true,
		},
	}
}

func policy(bsid srv6.SID) *srv6.Policy {
	return &srv6.Policy{
		Bsid:             bsid.String(),
		FibTableId:       0,
		SprayBehaviour:   true,
		SrhEncapsulation: true,
	}
}

func policySegment(policyBsid srv6.SID, weight uint32, sids ...srv6.SID) *srv6.PolicySegment {
	segments := make([]string, len(sids))
	for i, sid := range sids {
		segments[i] = sid.String()
	}

	return &srv6.PolicySegment{
		PolicyBsid: policyBsid.String(),
		Weight:     weight,
		Segments:   segments,
	}
}

func steeringWithPolicyBsidRef(bsid string) *srv6.Steering {
	return steeringRef(bsid, 0)
}

func steeringWithPolicyIndexRef(index uint32) *srv6.Steering {
	return steeringRef("", index)
}

func steeringRef(bsid string, index uint32) *srv6.Steering {
	return &srv6.Steering{
		PolicyBsid:  bsid,
		PolicyIndex: index,
		L3Traffic: &srv6.Steering_L3Traffic{
			FibTableId:    0,
			PrefixAddress: "A::",
		},
	}
}
