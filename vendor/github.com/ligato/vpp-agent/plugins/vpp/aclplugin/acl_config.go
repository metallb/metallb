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

// Package aclplugin implements the ACL Plugin that handles management of VPP
// Access lists.
package aclplugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/aclplugin/aclidx"
	"github.com/ligato/vpp-agent/plugins/vpp/aclplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
)

// Interface attribute according to the configuration
const (
	INGRESS = "ingress"
	EGRESS  = "egress"
	L2      = "l2"
)

// ACLIfCacheEntry contains info about interface, aclID and whether it is MAC IP address. Used as a cache for missing
// interfaces while configuring ACL
type ACLIfCacheEntry struct {
	ifName string
	aclID  uint32
	ifAttr string
}

// ACLConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of ACLs as modelled by the proto file "../model/acl/acl.proto" and stored
// in ETCD under the key "/vnf-agent/{agent-label}/vpp/config/v1/acl/". Updates received from the northbound API
// are compared with the VPP run-time configuration and differences are applied through the VPP binary API.
type ACLConfigurator struct {
	log logging.Logger

	// In-memory mappings
	ifIndexes      ifaceidx.SwIfIndex
	l2AclIndexes   aclidx.ACLIndexRW
	l3l4AclIndexes aclidx.ACLIndexRW

	// Cache for ACL un-configured interfaces
	ifCache []*ACLIfCacheEntry

	// VPP channels
	vppChan     govppapi.Channel
	vppDumpChan govppapi.Channel

	// ACL VPP calls handler
	aclHandler vppcalls.ACLVppAPI
}

// Init goroutines, channels and mappings.
func (c *ACLConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("acl-plugin")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.l2AclIndexes = aclidx.NewACLIndex(nametoidx.NewNameToIdx(c.log, "acl_l2_indexes", nil))
	c.l3l4AclIndexes = aclidx.NewACLIndex(nametoidx.NewNameToIdx(c.log, "acl_l3_l4_indexes", nil))

	// VPP channels
	c.vppChan, err = goVppMux.NewAPIChannel()
	if err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}
	c.vppDumpChan, err = goVppMux.NewAPIChannel()
	if err != nil {
		return errors.Errorf("failed to create dump API channel: %v", err)
	}

	// ACL binary api handler
	c.aclHandler = vppcalls.NewACLVppHandler(c.vppChan, c.vppDumpChan)

	c.log.Infof("ACL configurator initialized")

	return nil
}

// Close GOVPP channel.
func (c *ACLConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan, c.vppDumpChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose interface configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *ACLConfigurator) clearMapping() {
	c.l2AclIndexes.Clear()
	c.l3l4AclIndexes.Clear()
}

// GetL2AclIfIndexes exposes l2 acl interface name-to-index mapping
func (c *ACLConfigurator) GetL2AclIfIndexes() aclidx.ACLIndexRW {
	return c.l2AclIndexes
}

// GetL3L4AclIfIndexes exposes l3/l4 acl interface name-to-index mapping
func (c *ACLConfigurator) GetL3L4AclIfIndexes() aclidx.ACLIndexRW {
	return c.l3l4AclIndexes
}

// ConfigureACL creates access list with provided rules and sets this list to every relevant interface.
func (c *ACLConfigurator) ConfigureACL(acl *acl.AccessLists_Acl) error {
	if len(acl.Rules) == 0 {
		return errors.Errorf("failed to configure ACL %s, no rules to set", acl.AclName)
	}

	rules, isL2MacIP := c.validateRules(acl.AclName, acl.Rules)
	// Configure ACL rules.
	var vppACLIndex uint32
	var err error
	if isL2MacIP {
		vppACLIndex, err = c.aclHandler.AddMacIPACL(rules, acl.AclName)
		if err != nil {
			return errors.Errorf("failed to add MAC IP ACL %s: %v", acl.AclName, err)
		}
		// Index used for L2 registration is ACLIndex + 1 (ACL indexes start from 0).
		agentACLIndex := vppACLIndex + 1
		c.l2AclIndexes.RegisterName(acl.AclName, agentACLIndex, acl)
		c.log.Debugf("ACL %s registered to L2 mapping", acl.AclName)
	} else {
		vppACLIndex, err = c.aclHandler.AddIPACL(rules, acl.AclName)
		if err != nil {
			return errors.Errorf("failed to add IP ACL %s: %v", acl.AclName, err)
		}
		// Index used for L3L4 registration is aclIndex + 1 (ACL indexes start from 0).
		agentACLIndex := vppACLIndex + 1
		c.l3l4AclIndexes.RegisterName(acl.AclName, agentACLIndex, acl)
		c.log.Debugf("ACL %s registered to L3/L4 mapping", acl.AclName)
	}

	// Set ACL to interfaces.
	if ifaces := acl.GetInterfaces(); ifaces != nil {
		if isL2MacIP {
			aclIfIndices := c.getOrCacheInterfaces(acl.Interfaces.Ingress, vppACLIndex, L2)
			err := c.aclHandler.SetMacIPACLToInterface(vppACLIndex, aclIfIndices)
			if err != nil {
				return errors.Errorf("failed to set MAC IP ACL %s to interface(s) %v: %v",
					acl.AclName, acl.Interfaces.Ingress, err)
			}
		} else {
			aclIfInIndices := c.getOrCacheInterfaces(acl.Interfaces.Ingress, vppACLIndex, INGRESS)
			err = c.aclHandler.SetACLToInterfacesAsIngress(vppACLIndex, aclIfInIndices)
			if err != nil {
				return errors.Errorf("failed to set IP ACL %s to interface(s) %v as ingress: %v",
					acl.AclName, acl.Interfaces.Ingress, err)
			}
			aclIfEgIndices := c.getOrCacheInterfaces(acl.Interfaces.Egress, vppACLIndex, EGRESS)
			err = c.aclHandler.SetACLToInterfacesAsEgress(vppACLIndex, aclIfEgIndices)
			if err != nil {
				return errors.Errorf("failed to set IP ACL %s to interface(s) %v as egress: %v",
					acl.AclName, acl.Interfaces.Ingress, err)
			}
		}
	}

	c.log.Infof("ACL %s configured with ID %d", acl.AclName, vppACLIndex)

	return nil
}

// ModifyACL modifies previously created access list. L2 access list is removed and recreated,
// L3/L4 access list is modified directly. List of interfaces is refreshed as well.
func (c *ACLConfigurator) ModifyACL(oldACL, newACL *acl.AccessLists_Acl) error {
	if newACL.Rules != nil {
		// Validate rules.
		rules, isL2MacIP := c.validateRules(newACL.AclName, newACL.Rules)
		var vppACLIndex uint32
		if isL2MacIP {
			agentACLIndex, _, found := c.l2AclIndexes.LookupIdx(oldACL.AclName)
			if !found {
				return errors.Errorf("cannot modify IP MAC ACL %s, index not found in the mapping", oldACL.AclName)
			}
			// Index used in VPP = index used in mapping - 1
			vppACLIndex = agentACLIndex - 1
		} else {
			agentACLIndex, _, found := c.l3l4AclIndexes.LookupIdx(oldACL.AclName)
			if !found {
				return errors.Errorf("cannot modify IP ACL %s, index not found in the mapping", oldACL.AclName)
			}
			vppACLIndex = agentACLIndex - 1
		}
		if isL2MacIP {
			// L2 ACL
			err := c.aclHandler.ModifyMACIPACL(vppACLIndex, rules, newACL.AclName)
			if err != nil {
				return errors.Errorf("failed to modify MAC IP ACL %s: %v", newACL.AclName, err)
			}
			// There is no need to update index because modified ACL keeps the old one.
		} else {
			// L3/L4 ACL can be modified directly.
			err := c.aclHandler.ModifyIPACL(vppACLIndex, rules, newACL.AclName)
			if err != nil {
				return errors.Errorf("failed to modify IP ACL %s: %v", newACL.AclName, err)
			}
			// There is no need to update index because modified ACL keeps the old one.
		}

		// Update interfaces.
		if isL2MacIP {
			// Remove L2 ACL from old interfaces.
			if oldACL.Interfaces != nil {
				err := c.aclHandler.RemoveMacIPIngressACLFromInterfaces(vppACLIndex, c.getInterfaces(oldACL.Interfaces.Ingress))
				if err != nil {
					return errors.Errorf("failed to remove MAC IP ingress interfaces from ACL %s: %v",
						oldACL.AclName, err)
				}
			}
			// Put L2 ACL to new interfaces.
			if newACL.Interfaces != nil {
				aclMacInterfaces := c.getOrCacheInterfaces(newACL.Interfaces.Ingress, vppACLIndex, L2)
				err := c.aclHandler.SetMacIPACLToInterface(vppACLIndex, aclMacInterfaces)
				if err != nil {
					return errors.Errorf("failed to set MAC IP ingress interfaces to ACL %s: %v",
						newACL.AclName, err)
				}
			}
		} else {
			aclOldInInterfaces := c.getInterfaces(oldACL.Interfaces.Ingress)
			aclOldEgInterfaces := c.getInterfaces(oldACL.Interfaces.Egress)
			aclNewInInterfaces := c.getOrCacheInterfaces(newACL.Interfaces.Ingress, vppACLIndex, INGRESS)
			aclNewEgInterfaces := c.getOrCacheInterfaces(newACL.Interfaces.Egress, vppACLIndex, EGRESS)
			addedInInterfaces, removedInInterfaces := diffInterfaces(aclOldInInterfaces, aclNewInInterfaces)
			addedEgInterfaces, removedEgInterfaces := diffInterfaces(aclOldEgInterfaces, aclNewEgInterfaces)

			if len(removedInInterfaces) > 0 {
				err := c.aclHandler.RemoveIPIngressACLFromInterfaces(vppACLIndex, removedInInterfaces)
				if err != nil {
					return errors.Errorf("failed to remove IP ingress interfaces from ACL %s: %v",
						oldACL.AclName, err)
				}
			}
			if len(removedEgInterfaces) > 0 {
				err := c.aclHandler.RemoveIPEgressACLFromInterfaces(vppACLIndex, removedEgInterfaces)
				if err != nil {
					return errors.Errorf("failed to remove IP egress interfaces from ACL %s: %v",
						oldACL.AclName, err)
				}
			}
			if len(addedInInterfaces) > 0 {
				err := c.aclHandler.SetACLToInterfacesAsIngress(vppACLIndex, addedInInterfaces)
				if err != nil {
					return errors.Errorf("failed to set IP ingress interfaces to ACL %s: %v",
						newACL.AclName, err)
				}
			}
			if len(addedEgInterfaces) > 0 {
				err := c.aclHandler.SetACLToInterfacesAsEgress(vppACLIndex, addedEgInterfaces)
				if err != nil {
					return errors.Errorf("failed to add IP egress interfaces to ACL %s: %v",
						oldACL.AclName, err)
				}
			}
		}
	}

	c.log.Info("ACL %s modified", newACL.AclName)

	return nil
}

// DeleteACL removes existing ACL. To detach ACL from interfaces, list of interfaces has to be provided.
func (c *ACLConfigurator) DeleteACL(acl *acl.AccessLists_Acl) (err error) {
	// Get ACL index. Keep in mind that all ACL Indices were incremented by 1.
	agentL2AclIndex, _, l2AclFound := c.l2AclIndexes.LookupIdx(acl.AclName)
	agentL3L4AclIndex, _, l3l4AclFound := c.l3l4AclIndexes.LookupIdx(acl.AclName)
	if !l2AclFound && !l3l4AclFound {
		return errors.Errorf("cannot remove ACL %s, index not found in the mapping", acl.AclName)
	}
	if l2AclFound {
		// Remove interfaces from L2 ACL.
		vppACLIndex := agentL2AclIndex - 1
		if acl.Interfaces != nil {
			err := c.aclHandler.RemoveMacIPIngressACLFromInterfaces(vppACLIndex, c.getInterfaces(acl.Interfaces.Ingress))
			if err != nil {
				return errors.Errorf("failed to remove MAC IP interfaces from ACL %s: %v",
					acl.AclName, err)
			}
		}
		// Remove ACL L2.
		err := c.aclHandler.DeleteMacIPACL(vppACLIndex)
		if err != nil {
			return errors.Errorf("failed to remove MAC IP ACL %s: %v", acl.AclName, err)
		}
		// Unregister.
		c.l2AclIndexes.UnregisterName(acl.AclName)
		c.log.Debugf("ACL %s unregistered from L2 mapping", acl.AclName)
	}
	if l3l4AclFound {
		// Remove interfaces.
		vppACLIndex := agentL3L4AclIndex - 1
		if acl.Interfaces != nil {
			err = c.aclHandler.RemoveIPIngressACLFromInterfaces(vppACLIndex, c.getInterfaces(acl.Interfaces.Ingress))
			if err != nil {
				return errors.Errorf("failed to remove IP ingress interfaces from ACL %s: %v",
					acl.AclName, err)
			}

			err = c.aclHandler.RemoveIPEgressACLFromInterfaces(vppACLIndex, c.getInterfaces(acl.Interfaces.Egress))
			if err != nil {
				return errors.Errorf("failed to remove IP egress interfaces from ACL %s: %v",
					acl.AclName, err)
			}
		}
		// Remove ACL L3/L4.
		err := c.aclHandler.DeleteIPACL(vppACLIndex)
		if err != nil {
			return errors.Errorf("failed to remove IP ACL %s: %v", acl.AclName, err)
		}
		// Unregister.
		c.l3l4AclIndexes.UnregisterName(acl.AclName)
		c.log.Debugf("ACL %s unregistered from L3/L4 mapping", acl.AclName)
	}

	c.log.Infof("ACL %s removed", acl.AclName)

	return err
}

// DumpIPACL returns all configured IP ACLs in proto format
func (c *ACLConfigurator) DumpIPACL() (acls []*acl.AccessLists_Acl, err error) {
	aclsWithIndex, err := c.aclHandler.DumpIPACL(c.ifIndexes)
	if err != nil {
		return nil, errors.Errorf("failed to dump IP ACLs: %v", err)
	}
	for _, aclWithIndex := range aclsWithIndex {
		acls = append(acls, aclWithIndex.ACL)
	}
	return acls, nil
}

// DumpMACIPACL returns all configured MACIP ACLs in proto format
func (c *ACLConfigurator) DumpMACIPACL() (acls []*acl.AccessLists_Acl, err error) {
	aclsWithIndex, err := c.aclHandler.DumpMACIPACL(c.ifIndexes)
	if err != nil {
		c.log.Error(err)
		return nil, errors.Errorf("failed to dump MAC IP ACLs: %v", err)
	}
	for _, aclWithIndex := range aclsWithIndex {
		acls = append(acls, aclWithIndex.ACL)
	}
	return acls, nil
}

// Returns a list of existing ACL interfaces
func (c *ACLConfigurator) getInterfaces(interfaces []string) (configurableIfs []uint32) {
	for _, name := range interfaces {
		ifIdx, _, found := c.ifIndexes.LookupIdx(name)
		if !found {
			continue
		}
		configurableIfs = append(configurableIfs, ifIdx)
	}
	return configurableIfs
}

// diffInterfaces returns a difference between two lists of interfaces
func diffInterfaces(oldInterfaces, newInterfaces []uint32) (added, removed []uint32) {
	intfMap := make(map[uint32]struct{})
	for _, intf := range oldInterfaces {
		intfMap[intf] = struct{}{}
	}
	for _, intf := range newInterfaces {
		if _, has := intfMap[intf]; !has {
			added = append(added, intf)
		} else {
			delete(intfMap, intf)
		}
	}
	for intf := range intfMap {
		removed = append(removed, intf)
	}
	return added, removed
}

// ResolveCreatedInterface configures new interface for every ACL found in cache
func (c *ACLConfigurator) ResolveCreatedInterface(ifName string, ifIdx uint32) error {
	// Iterate over cache in order to find out where the interface is used
	for entryIdx, aclCacheEntry := range c.ifCache {
		if aclCacheEntry.ifName == ifName {
			switch aclCacheEntry.ifAttr {
			case L2:
				if err := c.aclHandler.SetMacIPACLToInterface(aclCacheEntry.aclID, []uint32{ifIdx}); err != nil {
					return errors.Errorf("failed to set MAC IP ACL %v to interface %v: %v", aclCacheEntry.aclID, ifName, err)
				}
			case INGRESS:
				if err := c.aclHandler.SetACLToInterfacesAsIngress(aclCacheEntry.aclID, []uint32{ifIdx}); err != nil {
					return errors.Errorf("failed to set ACL %v as ingress to interface %v: %v", aclCacheEntry.aclID, ifName, err)
				}
			case EGRESS:
				if err := c.aclHandler.SetACLToInterfacesAsEgress(aclCacheEntry.aclID, []uint32{ifIdx}); err != nil {
					return errors.Errorf("failed to set ACL %v as egress to interface %v: %v", aclCacheEntry.aclID, ifName, err)
				}
			default:
				return errors.Errorf("ACL %v for interface %v is set as %q and not as L2, ingress or egress", aclCacheEntry.aclID, ifName, aclCacheEntry.ifAttr)
			}
			// Remove from cache
			c.log.Debugf("New interface %s (%s) configured for ACL %d, removed from cache",
				ifName, aclCacheEntry.ifAttr, aclCacheEntry.aclID)
			c.ifCache = append(c.ifCache[:entryIdx], c.ifCache[entryIdx+1:]...)
		}
	}

	return nil
}

// ResolveDeletedInterface puts removed interface to cache, including acl index. Note: it's not needed to remove ACL
// from interface manually, VPP handles it itself and such an behavior would cause errors (ACLs cannot be dumped
// from non-existing interface)
func (c *ACLConfigurator) ResolveDeletedInterface(ifName string, ifIdx uint32) error {
	// L3/L4 ingress/egress ACLs
	for _, aclName := range c.l3l4AclIndexes.GetMapping().ListNames() {
		aclIdx, aclData, found := c.l3l4AclIndexes.LookupIdx(aclName)
		if !found {
			return errors.Errorf("cannot resolve ACL %s for interface %s, not found in the mapping", aclName, ifName)
		}
		vppACLIdx := aclIdx - 1
		if ifaces := aclData.GetInterfaces(); ifaces != nil {
			// Look over ingress interfaces
			for _, iface := range ifaces.Ingress {
				if iface == ifName {
					c.ifCache = append(c.ifCache, &ACLIfCacheEntry{
						ifName: ifName,
						aclID:  vppACLIdx,
						ifAttr: INGRESS,
					})
				}
			}
			// Look over egress interfaces
			for _, iface := range ifaces.Egress {
				if iface == ifName {
					c.ifCache = append(c.ifCache, &ACLIfCacheEntry{
						ifName: ifName,
						aclID:  vppACLIdx,
						ifAttr: EGRESS,
					})
				}
			}
		}
	}
	// L2 ACLs
	for _, aclName := range c.l2AclIndexes.GetMapping().ListNames() {
		aclIdx, aclData, found := c.l2AclIndexes.LookupIdx(aclName)
		if !found {
			return errors.Errorf("cannot resolve ACL %s for interface %s, not found in the mapping", aclName, ifName)
		}
		vppACLIdx := aclIdx - 1
		if ifaces := aclData.GetInterfaces(); ifaces != nil {
			// Look over ingress interfaces
			for _, ingressIf := range ifaces.Ingress {
				if ingressIf == ifName {
					c.ifCache = append(c.ifCache, &ACLIfCacheEntry{
						ifName: ifName,
						aclID:  vppACLIdx,
						ifAttr: L2,
					})
				}
			}
		}
	}

	return nil
}

// Returns a list of interfaces configurable on the ACL. If interface is missing, put it to the cache. It will be
// configured when available
func (c *ACLConfigurator) getOrCacheInterfaces(interfaces []string, acl uint32, attr string) (configurableIfs []uint32) {
	for _, name := range interfaces {
		ifIdx, _, found := c.ifIndexes.LookupIdx(name)
		if !found {
			// Put interface to cache
			c.ifCache = append(c.ifCache, &ACLIfCacheEntry{
				ifName: name,
				aclID:  acl,
				ifAttr: attr,
			})
			c.log.Debugf("Interface %s (%s) not found for ACL %s, moving to cache", name, attr, acl)
			continue
		}
		configurableIfs = append(configurableIfs, ifIdx)
	}
	return configurableIfs
}

// Validate rules provided in ACL. Every rule has to contain actions and matches.
// Current limitation: L2 and L3/4 have to be split to different ACLs and
// there cannot be L2 rules and L3/4 rules in the same ACL.
func (c *ACLConfigurator) validateRules(aclName string, rules []*acl.AccessLists_Acl_Rule) ([]*acl.AccessLists_Acl_Rule, bool) {
	var validL3L4Rules []*acl.AccessLists_Acl_Rule
	var validL2Rules []*acl.AccessLists_Acl_Rule

	for index, rule := range rules {
		if rule.GetMatch() == nil {
			c.log.Warnf("invalid ACL %s: rule %d does not contain match", aclName, index)
			continue
		}
		if rule.GetMatch().GetIpRule() != nil {
			validL3L4Rules = append(validL3L4Rules, rule)
		}
		if rule.GetMatch().GetMacipRule() != nil {
			validL2Rules = append(validL2Rules, rule)
		}
	}
	if len(validL3L4Rules) > 0 && len(validL2Rules) > 0 {
		c.log.Warnf("ACL %s contains L2 rules and L3/L4 rules as well. This case is not supported, only L3/L4 rules will be resolved",
			aclName)
		return validL3L4Rules, false
	} else if len(validL3L4Rules) > 0 {
		return validL3L4Rules, false
	} else {
		return validL2Rules, true
	}
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *ACLConfigurator) LogError(err error) error {
	if err == nil {
		return nil
	}
	c.log.WithField("logger", c.log).Errorf(string(err.Error() + "\n" + string(err.(*errors.Error).Stack())))
	return err
}
