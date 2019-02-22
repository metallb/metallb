//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package descriptor

import (
	"bytes"
	"net"

	"github.com/gogo/protobuf/proto"
	prototypes "github.com/gogo/protobuf/types"
	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/logging"
	"github.com/pkg/errors"

	acl "github.com/ligato/vpp-agent/api/models/vpp/acl"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/aclplugin/aclidx"
	"github.com/ligato/vpp-agent/plugins/vppv2/aclplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/aclplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
	ifdescriptor "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/descriptor"
)

const (
	// ACLDescriptorName is descriptor name
	ACLDescriptorName = "vpp-acl"
)

// ACLDescriptor is descriptor for ACL
type ACLDescriptor struct {
	// dependencies
	log        logging.Logger
	aclHandler vppcalls.ACLVppAPI

	// runtime
	ifPlugin ifplugin.API
}

// NewACLDescriptor is constructor for ACL descriptor
func NewACLDescriptor(aclHandler vppcalls.ACLVppAPI, ifPlugin ifplugin.API,
	logger logging.PluginLogger) *ACLDescriptor {
	return &ACLDescriptor{
		log:        logger.NewLogger("acl-descriptor"),
		ifPlugin:   ifPlugin,
		aclHandler: aclHandler,
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *ACLDescriptor) GetDescriptor() *adapter.ACLDescriptor {
	return &adapter.ACLDescriptor{
		Name:            ACLDescriptorName,
		NBKeyPrefix:     acl.ModelACL.KeyPrefix(),
		ValueTypeName:   acl.ModelACL.ProtoName(),
		KeySelector:     acl.ModelACL.IsKeyValid,
		KeyLabel:        acl.ModelACL.StripKeyPrefix,
		ValueComparator: d.EquivalentACLs,
		WithMetadata:    true,
		MetadataMapFactory: func() idxmap.NamedMappingRW {
			return aclidx.NewACLIndex(d.log, "vpp-acl-index")
		},
		Add:                d.Add,
		Delete:             d.Delete,
		Modify:             d.Modify,
		ModifyWithRecreate: d.ModifyWithRecreate,
		DerivedValues:      d.DerivedValues,
		Dump:               d.Dump,
		DumpDependencies:   []string{ifdescriptor.InterfaceDescriptorName},
	}
}

// EquivalentACLs compares two ACLs
func (d *ACLDescriptor) EquivalentACLs(key string, oldACL, newACL *acl.ACL) bool {
	// check if ACL name changed
	if oldACL.Name != newACL.Name {
		return false
	}

	// check if rules changed (order matters)
	if len(oldACL.Rules) != len(newACL.Rules) {
		return false
	}
	for i := 0; i < len(oldACL.Rules); i++ {
		if !d.equivalentACLRules(oldACL.Rules[i], newACL.Rules[i]) {
			return false
		}
	}

	return true
}

// validateRules provided in ACL. Every rule has to contain actions and matches.
// Current limitation: L2 and L3/4 have to be split to different ACLs and
// there cannot be L2 rules and L3/4 rules in the same ACL.
func (d *ACLDescriptor) validateRules(aclName string, rules []*acl.ACL_Rule) ([]*acl.ACL_Rule, bool) {
	var validL3L4Rules []*acl.ACL_Rule
	var validL2Rules []*acl.ACL_Rule

	for _, rule := range rules {
		if rule.GetIpRule() != nil {
			validL3L4Rules = append(validL3L4Rules, rule)
		}
		if rule.GetMacipRule() != nil {
			validL2Rules = append(validL2Rules, rule)
		}
	}
	if len(validL3L4Rules) > 0 && len(validL2Rules) > 0 {
		d.log.Warnf("ACL %s contains L2 rules and L3/L4 rules as well. This case is not supported, only L3/L4 rules will be resolved",
			aclName)
		return validL3L4Rules, false
	} else if len(validL3L4Rules) > 0 {
		return validL3L4Rules, false
	} else {
		return validL2Rules, true
	}
}

// Add configures ACL
func (d *ACLDescriptor) Add(key string, acl *acl.ACL) (metadata *aclidx.ACLMetadata, err error) {
	if len(acl.Rules) == 0 {
		return nil, errors.Errorf("failed to configure ACL %s, no rules to set", acl.Name)
	}

	rules, isL2MacIP := d.validateRules(acl.Name, acl.Rules)

	// Configure ACL rules.
	var vppACLIndex uint32
	if isL2MacIP {
		vppACLIndex, err = d.aclHandler.AddMACIPACL(rules, acl.Name)
		if err != nil {
			return nil, errors.Errorf("failed to add MACIP ACL %s: %v", acl.Name, err)
		}
	} else {
		vppACLIndex, err = d.aclHandler.AddACL(rules, acl.Name)
		if err != nil {
			return nil, errors.Errorf("failed to add IP ACL %s: %v", acl.Name, err)
		}
	}

	metadata = &aclidx.ACLMetadata{
		Index: vppACLIndex,
		L2:    isL2MacIP,
	}
	return metadata, nil
}

// Delete deletes ACL
func (d *ACLDescriptor) Delete(key string, acl *acl.ACL, metadata *aclidx.ACLMetadata) error {
	if metadata.L2 {
		// Remove ACL L2.
		err := d.aclHandler.DeleteMACIPACL(metadata.Index)
		if err != nil {
			return errors.Errorf("failed to delete MACIP ACL %s: %v", acl.Name, err)
		}
	} else {
		// Remove ACL L3/L4.
		err := d.aclHandler.DeleteACL(metadata.Index)
		if err != nil {
			return errors.Errorf("failed to delete IP ACL %s: %v", acl.Name, err)
		}
	}
	return nil
}

// Modify modifies ACL
func (d *ACLDescriptor) Modify(key string, oldACL, newACL *acl.ACL, oldMetadata *aclidx.ACLMetadata) (newMetadata *aclidx.ACLMetadata, err error) {
	// Validate rules.
	rules, isL2MacIP := d.validateRules(newACL.Name, newACL.Rules)

	if isL2MacIP {
		// L2 ACL
		err := d.aclHandler.ModifyMACIPACL(oldMetadata.Index, rules, newACL.Name)
		if err != nil {
			return nil, errors.Errorf("failed to modify MACIP ACL %s: %v", newACL.Name, err)
		}
	} else {
		// L3/L4 ACL can be modified directly.
		err := d.aclHandler.ModifyACL(oldMetadata.Index, rules, newACL.Name)
		if err != nil {
			return nil, errors.Errorf("failed to modify IP ACL %s: %v", newACL.Name, err)
		}
	}

	newMetadata = &aclidx.ACLMetadata{
		Index: oldMetadata.Index,
		L2:    isL2MacIP,
	}
	return newMetadata, nil
}

// ModifyWithRecreate checks if modification requires recreation
func (d *ACLDescriptor) ModifyWithRecreate(key string, oldACL, newACL *acl.ACL, metadata *aclidx.ACLMetadata) bool {
	var hasL2 bool
	for _, rule := range oldACL.Rules {
		if rule.GetMacipRule() != nil {
			hasL2 = true
		} else if rule.GetIpRule() != nil && hasL2 {
			return true
		}
	}
	return false
}

// DerivedValues returns list of derived values for ACL.
func (d *ACLDescriptor) DerivedValues(key string, value *acl.ACL) (derived []api.KeyValuePair) {
	for _, ifName := range value.GetInterfaces().GetIngress() {
		derived = append(derived, api.KeyValuePair{
			Key:   acl.ToInterfaceKey(value.Name, ifName, acl.IngressFlow),
			Value: &prototypes.Empty{},
		})
	}
	for _, ifName := range value.GetInterfaces().GetEgress() {
		derived = append(derived, api.KeyValuePair{
			Key:   acl.ToInterfaceKey(value.Name, ifName, acl.EgressFlow),
			Value: &prototypes.Empty{},
		})
	}
	return derived
}

// Dump returns list of dumped ACLs with metadata
func (d *ACLDescriptor) Dump(correlate []adapter.ACLKVWithMetadata) (
	dump []adapter.ACLKVWithMetadata, err error,
) {
	// Retrieve VPP configuration.
	ipACLs, err := d.aclHandler.DumpACL()
	if err != nil {
		return nil, errors.Errorf("failed to dump IP ACLs: %v", err)
	}
	macipACLs, err := d.aclHandler.DumpMACIPACL()
	if err != nil {
		return nil, errors.Errorf("failed to dump MAC IP ACLs: %v", err)
	}

	for _, ipACL := range ipACLs {
		dump = append(dump, adapter.ACLKVWithMetadata{
			Key:   acl.Key(ipACL.ACL.Name),
			Value: ipACL.ACL,
			Metadata: &aclidx.ACLMetadata{
				Index: ipACL.Meta.Index,
			},
			Origin: api.FromNB,
		})
	}
	for _, macipACL := range macipACLs {
		dump = append(dump, adapter.ACLKVWithMetadata{
			Key:   acl.Key(macipACL.ACL.Name),
			Value: macipACL.ACL,
			Metadata: &aclidx.ACLMetadata{
				Index: macipACL.Meta.Index,
				L2:    true,
			},
			Origin: api.FromNB,
		})
	}

	return
}

// equivalentACLRules compares two ACL rules, handling the cases of unspecified
// source/destination networks.
func (d *ACLDescriptor) equivalentACLRules(rule1, rule2 *acl.ACL_Rule) bool {
	// Action
	if rule1.Action != rule2.Action {
		return false
	}

	// MAC IP Rule
	if !proto.Equal(rule1.MacipRule, rule2.MacipRule) {
		return false
	}

	// IP Rule
	ipRule1 := rule1.GetIpRule()
	ipRule2 := rule2.GetIpRule()
	if !proto.Equal(ipRule1.GetIcmp(), ipRule2.GetIcmp()) ||
		!proto.Equal(ipRule1.GetTcp(), ipRule2.GetTcp()) ||
		!proto.Equal(ipRule1.GetUdp(), ipRule2.GetUdp()) {
		return false
	}
	if !d.equivalentIPRuleNetworks(ipRule1.GetIp().GetSourceNetwork(), ipRule2.GetIp().GetSourceNetwork()) {
		return false
	}
	if !d.equivalentIPRuleNetworks(ipRule1.GetIp().GetDestinationNetwork(), ipRule2.GetIp().GetDestinationNetwork()) {
		return false
	}
	return true
}

// equivalentIPRuleNetworks compares two IP networks, taking into account the fact
// that empty string is equivalent to address with all zeroes.
func (d *ACLDescriptor) equivalentIPRuleNetworks(net1, net2 string) bool {
	var (
		ip1, ip2       net.IP
		ipNet1, ipNet2 *net.IPNet
		err1, err2     error
	)
	if net1 != "" {
		ip1, ipNet1, err1 = net.ParseCIDR(net1)
	}
	if net2 != "" {
		ip2, ipNet2, err2 = net.ParseCIDR(net2)
	}
	if err1 != nil || err2 != nil {
		return net1 == net2
	}
	if ipNet1 == nil {
		return ipNet2 == nil || ip2.IsUnspecified()
	}
	if ipNet2 == nil {
		return ipNet1 == nil || ip1.IsUnspecified()
	}
	return ip1.Equal(ip2) && bytes.Equal(ipNet1.Mask, ipNet2.Mask)
}
