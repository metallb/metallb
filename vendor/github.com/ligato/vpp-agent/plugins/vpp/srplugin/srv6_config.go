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

package srplugin

import (
	"fmt"
	"net"
	"sort"
	"strings"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
	"github.com/ligato/vpp-agent/plugins/vpp/srplugin/cache"
	"github.com/ligato/vpp-agent/plugins/vpp/srplugin/vppcalls"
)

// TODO check all SID usages for comparisons that can fail due to upper/lower case mismatch (i.e. strings "A::E" and "a::e" are not equal but for our purposes it is the same SID and should be considered equal)

// SRv6Configurator runs in the background where it watches for any changes in the configuration of interfaces as
// modelled by the proto file "../model/srv6/srv6.proto" and stored in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1/srv6".
type SRv6Configurator struct {
	// injectable/public fields
	log         logging.Logger
	swIfIndexes ifaceidx.SwIfIndex // SwIfIndexes from default plugins

	// channels
	vppChan govppapi.Channel // channel to communicate with VPP

	// vpp api handler
	srHandler vppcalls.SRv6VppAPI

	// caches
	policyCache         *cache.PolicyCache        // Cache for SRv6 policies
	policySegmentsCache *cache.PolicySegmentCache // Cache for SRv6 policy segments
	steeringCache       *cache.SteeringCache      // Cache for SRv6 steering
	createdPolicies     map[string]struct{}       // Marker for created policies (key = bsid in string form)

	// indexes
	policyIndexSeq        *gaplessSequence
	policyIndexes         idxvpp.NameToIdxRW // Mapping between policy bsid and index inside VPP
	policySegmentIndexSeq *gaplessSequence
	policySegmentIndexes  idxvpp.NameToIdxRW // Mapping between policy segment name as defined in ETCD key and index inside VPP
}

// Init members
func (c *SRv6Configurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex,
	srHandler vppcalls.SRv6VppAPI) (err error) {
	// Logger
	c.log = logger.NewLogger("sr-plugin")

	// NewAPIChannel returns a new API channel for communication with VPP via govpp core.
	// It uses default buffer sizes for the request and reply Go channels.
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handler
	if srHandler != nil {
		c.srHandler = srHandler
	} else {
		c.srHandler = vppcalls.NewSRv6VppHandler(c.vppChan, c.log)
	}

	// Interface indexes
	c.swIfIndexes = swIfIndexes

	// Create caches
	c.policyCache = cache.NewPolicyCache(c.log)
	c.policySegmentsCache = cache.NewPolicySegmentCache(c.log)
	c.steeringCache = cache.NewSteeringCache(c.log)
	c.createdPolicies = make(map[string]struct{})

	// Create indexes
	c.policySegmentIndexSeq = newSequence()
	c.policySegmentIndexes = nametoidx.NewNameToIdx(c.log, "policy-segment-indexes", nil)
	c.policyIndexSeq = newSequence()
	c.policyIndexes = nametoidx.NewNameToIdx(c.log, "policy-indexes", nil)

	c.log.Info("Segment routing configurator initialized")

	return
}

// Close closes GOVPP channel
func (c *SRv6Configurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose segment routing configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *SRv6Configurator) clearMapping() {
	// Clear caches
	c.policyCache = cache.NewPolicyCache(c.log)
	c.policySegmentsCache = cache.NewPolicySegmentCache(c.log)
	c.steeringCache = cache.NewSteeringCache(c.log)
	c.createdPolicies = make(map[string]struct{})

	// Clear indexes
	c.policySegmentIndexSeq = newSequence()
	c.policySegmentIndexes.Clear()
	c.policyIndexSeq = newSequence()
	c.policyIndexes.Clear()

	c.log.Debugf("segment routing configurator mapping cleared")
}

// AddLocalSID adds new Local SID into VPP using VPP's binary api
func (c *SRv6Configurator) AddLocalSID(value *srv6.LocalSID) error {
	sid, err := ParseIPv6(value.GetSid())
	if err != nil {
		return errors.Errorf("failed to parse local sid %s, should be a valid ipv6 address: %v",
			value.GetSid(), err)
	}
	if err := c.srHandler.AddLocalSid(sid, value, c.swIfIndexes); err != nil {
		return errors.Errorf("failed to add local sid %s: %v", sid.String(), err)
	}
	c.log.Infof("Local sid %s added", value.GetSid())

	return nil
}

// DeleteLocalSID removes Local SID from VPP using VPP's binary api
func (c *SRv6Configurator) DeleteLocalSID(value *srv6.LocalSID) error {
	sid, err := ParseIPv6(value.GetSid())
	if err != nil {
		return errors.Errorf("failed to parse local sid %s, should be a valid ipv6 address: %v",
			value.GetSid(), err)
	}
	if err := c.srHandler.DeleteLocalSid(sid); err != nil {
		return errors.Errorf("failed to delete local sid %s: %v", sid.String(), err)
	}
	c.log.Infof("Local sid %s removed", value.GetSid())

	return nil
}

// ModifyLocalSID modifies Local SID from <prevValue> to <value> in VPP using VPP's binary api
func (c *SRv6Configurator) ModifyLocalSID(value *srv6.LocalSID, prevValue *srv6.LocalSID) error {
	err := c.DeleteLocalSID(prevValue)
	if err != nil {
		return errors.Errorf("local sid modify: failed to delete old version of Local SID %s: %v", prevValue.Sid, err)
	}
	err = c.AddLocalSID(value)
	if err != nil {
		return errors.Errorf("local sid modify: failed to apply new version of Local SID %s: %v", value.Sid, err)
	}
	c.log.Infof("Sid %s modified", value.GetSid())

	return nil
}

// AddPolicy adds new policy into VPP using VPP's binary api
func (c *SRv6Configurator) AddPolicy(policy *srv6.Policy) error {
	bsid, err := ParseIPv6(policy.GetBsid())
	if err != nil {
		return errors.Errorf("failed to parse bsid %s, should be a valid ipv6 address: %v",
			policy.GetBsid(), err)
	}
	c.policyCache.Put(bsid, policy)
	segments, segmentNames := c.policySegmentsCache.LookupByPolicy(bsid)
	if len(segments) == 0 {
		c.log.Debugf("addition of policy (%v) postponed until first policy segment is defined for it", bsid.String())
		return nil
	}

	c.addPolicyToIndexes(bsid)
	c.addSegmentToIndexes(bsid, segmentNames[0])
	err = c.srHandler.AddPolicy(bsid, policy, segments[0])
	if err != nil {
		return errors.Errorf("failed to write policy %s with first segment %s: %v",
			bsid.String(), segments[0].Segments, err)
	}
	c.createdPolicies[bsid.String()] = struct{}{} // write into Set that policy was successfully created
	if len(segments) > 1 {
		for i, segment := range segments[1:] {
			err = c.AddPolicySegment(segmentNames[i], segment)
			if err != nil {
				return errors.Errorf("faield to apply subsequent policy segment %s to policy %s: %v",
					segment.Segments, bsid.String(), err)
			}
		}
	}

	// adding policy dependent steerings
	idx, _, _ := c.policyIndexes.LookupIdx(bsid.String())
	steerings, steeringNames := c.lookupSteeringByPolicy(bsid, idx)
	for i, steering := range steerings {
		if err := c.AddSteering(steeringNames[i], steering); err != nil {
			return fmt.Errorf("failed to create steering %s by creating policy referenced by steering: %v",
				steeringNames[i], err)
		}
	}

	c.log.Infof("Policy with bsid %s added", policy.Bsid)

	return nil
}

func (c *SRv6Configurator) lookupSteeringByPolicy(bsid srv6.SID, index uint32) ([]*srv6.Steering, []string) {
	// union search by bsid and policy index
	steerings1, steeringNames1 := c.steeringCache.LookupByPolicyBSID(bsid)
	steerings2, steeringNames2 := c.steeringCache.LookupByPolicyIndex(index)
	steerings1 = append(steerings1, steerings2...)
	steeringNames1 = append(steeringNames1, steeringNames2...)
	return steerings1, steeringNames1
}

// RemovePolicy removes policy from VPP using VPP's binary api
func (c *SRv6Configurator) RemovePolicy(policy *srv6.Policy) error {
	bsid, err := ParseIPv6(policy.GetBsid())
	if err != nil {
		return errors.Errorf("failed to parse bsid %s, should be a valid ipv6 address: %v",
			policy.GetBsid(), err)
	}
	// adding policy dependent steerings
	idx, _, _ := c.policyIndexes.LookupIdx(bsid.String())
	steerings, steeringNames := c.lookupSteeringByPolicy(bsid, idx)
	for i, steering := range steerings {
		if err := c.RemoveSteering(steeringNames[i], steering); err != nil {
			return errors.Errorf("failed to remove steering %s in process of removing policy referenced by steering: %v",
				steeringNames[i], err)
		}
	}

	c.policyCache.Delete(bsid)
	c.policyIndexes.UnregisterName(bsid.String())
	c.log.Debugf("policy %s removed from cache and mapping", policy.Bsid)
	delete(c.createdPolicies, bsid.String())
	_, segmentNames := c.policySegmentsCache.LookupByPolicy(bsid)
	for _, segmentName := range segmentNames {
		c.policySegmentsCache.Delete(bsid, segmentName)
		index, _, exists := c.policySegmentIndexes.UnregisterName(c.uniquePolicySegmentName(bsid, segmentName))
		c.log.Debugf("policy segment %s removed from cache and mapping", policy.Bsid)
		if exists {
			c.policySegmentIndexSeq.delete(index)
		}
	}
	if err := c.srHandler.DeletePolicy(bsid); err != nil { // expecting that policy delete will also delete policy segments in vpp
		return errors.Errorf("failed to delete policy %s: %v",
			bsid.String(), err)
	}

	return nil
}

// ModifyPolicy modifies policy in VPP using VPP's binary api
func (c *SRv6Configurator) ModifyPolicy(value *srv6.Policy, prevValue *srv6.Policy) error {
	bsid, err := ParseIPv6(value.GetBsid())
	if err != nil {
		return errors.Errorf("failed to parse modified bsid %s, should be a valid ipv6 address: %v",
			value.GetBsid(), err)
	}
	segments, segmentNames := c.policySegmentsCache.LookupByPolicy(bsid)
	err = c.RemovePolicy(prevValue)
	if err != nil {
		return err
	}
	err = c.AddPolicy(value)
	if err != nil {
		return err
	}
	for i, segment := range segments {
		err = c.AddPolicySegment(segmentNames[i], segment)
		if err != nil {
			return errors.Errorf("can't apply segment %s (%s) as part of policy modification: %v", segmentNames[i], segment.Segments, err)
		}
	}
	return nil
}

// AddPolicySegment adds policy segment <policySegment> with name <segmentName> into referenced policy in VPP using VPP's binary api.
func (c *SRv6Configurator) AddPolicySegment(segmentName string, policySegment *srv6.PolicySegment) error {
	bsid, err := ParseIPv6(policySegment.GetPolicyBsid())
	if err != nil {
		return errors.Errorf("failed to parse bsid %s, should be a valid ipv6 address: %v",
			policySegment.GetPolicyBsid(), err)
	}
	c.policySegmentsCache.Put(bsid, segmentName, policySegment)
	policy, exists := c.policyCache.GetValue(bsid)
	if !exists {
		c.log.Debugf("addition of policy segment (%v) postponed until policy with %s bsid is created",
			policySegment.GetSegments(), bsid.String())
		return nil
	}

	segments, _ := c.policySegmentsCache.LookupByPolicy(bsid)
	if len(segments) <= 1 {
		if _, alreadyCreated := c.createdPolicies[bsid.String()]; alreadyCreated {
			// last segment got deleted in etcd, but policy with last segment stays in VPP, and we want to add another segment
			// -> we must remove the old policy with last segment from the VPP to add it again with the new segment
			err := c.RemovePolicy(policy)
			if err != nil {
				return errors.Errorf("can't delete Policy (with previously deleted last policy segment) to recreated it with new policy segment: %v", err)
			}
			c.policySegmentsCache.Put(bsid, segmentName, policySegment) // got deleted in policy removal
		}
		return c.AddPolicy(policy)
	}
	// FIXME there is no API contract saying what happens to VPP indexes if addition fails (also different fail code can rollback or not rollback indexes) => no way how to handle this without being dependent on internal implementation inside VPP and that is just very fragile -> API should tell this but it doesn't!
	c.addSegmentToIndexes(bsid, segmentName)
	if err := c.srHandler.AddPolicySegment(bsid, policy, policySegment); err != nil {
		return errors.Errorf("failed to add policy segment %s: %v", bsid, err)
	}

	return nil
}

// RemovePolicySegment removes policy segment <policySegment> with name <segmentName> from referenced policy in VPP using
// VPP's binary api. In case of last policy segment in policy, policy segment is not removed, because policy can't exists
// in VPP without policy segment. Instead it is postponed until policy removal or addition of another policy segment happen.
func (c *SRv6Configurator) RemovePolicySegment(segmentName string, policySegment *srv6.PolicySegment) error {
	bsid, err := ParseIPv6(policySegment.GetPolicyBsid())
	if err != nil {
		return errors.Errorf("failed to parse bsid %s, should be a valid ipv6 address: %v",
			policySegment.GetPolicyBsid(), err)
	}
	c.policySegmentsCache.Delete(bsid, segmentName)
	index, _, exists := c.policySegmentIndexes.UnregisterName(c.uniquePolicySegmentName(bsid, segmentName))

	siblings, _ := c.policySegmentsCache.LookupByPolicy(bsid) // sibling segments in the same policy
	if len(siblings) == 0 {                                   // last segment for policy
		c.log.Debugf("removal of policy segment (%v) postponed until policy with %s bsid is deleted", policySegment.GetSegments(), bsid.String())
		return nil
	}

	// removing not-last segment
	if !exists {
		return errors.Errorf("can't find index of policy segment %s in policy with bsid %s", policySegment.Segments, bsid)
	}
	policy, exists := c.policyCache.GetValue(bsid)
	if !exists {
		return errors.Errorf("can't find policy with bsid %s", bsid)
	}
	// FIXME there is no API contract saying what happens to VPP indexes if removal fails (also different fail code can rollback or not rollback indexes) => no way how to handle this without being dependent on internal implementation inside VPP and that is just very fragile -> API should tell this but it doesn't!
	c.policySegmentIndexSeq.delete(index)
	if err := c.srHandler.DeletePolicySegment(bsid, policy, policySegment, index); err != nil {
		return errors.Errorf("failed to delete policy segment %s: %v", bsid, err)
	}

	return nil
}

// ModifyPolicySegment modifies existing policy segment with name <segmentName> from <prevValue> to <value> in referenced policy.
func (c *SRv6Configurator) ModifyPolicySegment(segmentName string, value *srv6.PolicySegment, prevValue *srv6.PolicySegment) error {
	bsid, err := ParseIPv6(value.GetPolicyBsid())
	if err != nil {
		return errors.Errorf("failed to parse modified bsid %s, should be a valid ipv6 address: %v",
			value.GetPolicyBsid(), err)
	}
	segments, _ := c.policySegmentsCache.LookupByPolicy(bsid)
	if len(segments) <= 1 { // last segment in policy can't be removed without removing policy itself
		policy, exists := c.policyCache.GetValue(bsid)
		if !exists {
			return errors.Errorf("can't find Policy in cache when updating last policy segment in policy")
		}
		err := c.RemovePolicy(policy)
		if err != nil {
			return errors.Errorf("can't delete Policy as part of removing old version of last policy segment in policy: %v", err)
		}
		err = c.AddPolicy(policy)
		if err != nil {
			return errors.Errorf("can't add Policy as part of adding new version of last policy segment in policy: %v", err)
		}
		err = c.AddPolicySegment(segmentName, value)
		if err != nil {
			return errors.Errorf("can't apply new version of last Policy segment: %v", err)
		}
		return nil
	}
	err = c.RemovePolicySegment(segmentName, prevValue)
	if err != nil {
		return errors.Errorf("can't delete old version of Policy segment: %v", err)
	}
	err = c.AddPolicySegment(segmentName, value)
	if err != nil {
		return errors.Errorf("can't apply new version of Policy segment: %v", err)
	}
	return nil
}

func (c *SRv6Configurator) addSegmentToIndexes(bsid srv6.SID, segmentName string) {
	c.policySegmentIndexes.RegisterName(c.uniquePolicySegmentName(bsid, segmentName), c.policySegmentIndexSeq.nextID(), nil)
	c.log.Debugf("policy segment with bsid %s registered", bsid.String())
}

func (c *SRv6Configurator) addPolicyToIndexes(bsid srv6.SID) {
	c.policyIndexes.RegisterName(bsid.String(), c.policyIndexSeq.nextID(), nil)
	c.log.Debugf("policy with bsid %s registered", bsid.String())
}

func (c *SRv6Configurator) uniquePolicySegmentName(bsid srv6.SID, segmentName string) string {
	return bsid.String() + srv6.EtcdKeyPathDelimiter + segmentName
}

// AddSteering adds new steering into VPP using VPP's binary api
func (c *SRv6Configurator) AddSteering(name string, steering *srv6.Steering) error {
	c.steeringCache.Put(name, steering)
	bsidStr := steering.PolicyBsid
	if len(strings.Trim(steering.PolicyBsid, " ")) == 0 { // policy defined by index
		var exists bool
		bsidStr, _, exists = c.policyIndexes.LookupName(steering.PolicyIndex)
		if !exists {
			c.log.Debugf("addition of steering (index %d) postponed until referenced policy is defined", steering.PolicyIndex)
			return nil
		}
	}
	// policy defined by BSID (or index defined converted to BSID defined)
	bsid, err := ParseIPv6(bsidStr)
	if err != nil {
		return errors.Errorf("failed to parse modified bsid %s, should be a valid ipv6 address: %v",
			steering.GetPolicyBsid(), err)
	}
	if _, exists := c.policyCache.GetValue(bsid); !exists {
		c.log.Debugf("addition of steering (bsid %s) postponed until referenced policy is defined", name)
		return nil
	}

	if err := c.srHandler.AddSteering(steering, c.swIfIndexes); err != nil {
		return errors.Errorf("failed to add steering %s: %v", name, err)
	}

	return nil
}

// RemoveSteering removes steering from VPP using VPP's binary api
func (c *SRv6Configurator) RemoveSteering(name string, steering *srv6.Steering) error {
	c.steeringCache.Delete(name)
	c.log.Debugf("steering %s removed from cache", name)
	if err := c.srHandler.RemoveSteering(steering, c.swIfIndexes); err != nil {
		return errors.Errorf("failed to remove steering %s: %v", name, err)
	}

	return nil
}

// ModifySteering modifies existing steering in VPP using VPP's binary api
func (c *SRv6Configurator) ModifySteering(name string, value *srv6.Steering, prevValue *srv6.Steering) error {
	err := c.RemoveSteering(name, prevValue)
	if err != nil {
		return errors.Errorf("modify steering: failed to remove old value %s: %v", name, err)
	}
	err = c.AddSteering(name, value)
	if err != nil {
		return errors.Errorf("modify steering: failed to apply new value %s: %v", name, err)
	}
	return nil
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *SRv6Configurator) LogError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *errors.Error:
		c.log.WithField("logger", c.log).Errorf(string(err.Error() + "\n" + string(err.(*errors.Error).Stack())))
	default:
		c.log.Error(err)
	}
	return err
}

// ParseIPv6 parses string <str> to IPv6 address (including IPv4 address converted to IPv6 address)
func ParseIPv6(str string) (net.IP, error) {
	ip := net.ParseIP(str)
	if ip == nil {
		return nil, errors.Errorf(" %q is not ip address", str)
	}
	ipv6 := ip.To16()
	if ipv6 == nil {
		return nil, errors.Errorf(" %q is not ipv6 address", str)
	}
	return ipv6, nil
}

// gaplessSequence emulates sequence indexes grabbing for Policy segments inside VPP // FIXME this is poor VPP API, correct way is tha API should tell as choosen index at Policy segment creation
type gaplessSequence struct {
	nextfree []uint32
}

func newSequence() *gaplessSequence {
	return &gaplessSequence{
		nextfree: []uint32{0},
	}
}

func (seq *gaplessSequence) nextID() uint32 {
	if len(seq.nextfree) == 1 { // no gaps in sequence
		result := seq.nextfree[0]
		seq.nextfree[0]++
		return result
	}
	// use first gap and then remove it from free IDs list
	result := seq.nextfree[0]
	seq.nextfree = seq.nextfree[1:]
	return result
}

func (seq *gaplessSequence) delete(id uint32) {
	if id >= seq.nextfree[len(seq.nextfree)-1] {
		return // nothing to do because it is not sequenced yet
	}
	// add gap and move it to proper place (gaps with lower id should be used first by finding next ID)
	seq.nextfree = append(seq.nextfree, id)
	sort.Slice(seq.nextfree, func(i, j int) bool { return seq.nextfree[i] < seq.nextfree[j] })
}
