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

package ifplugin

import (
	"bytes"
	"net"
	"strings"

	"github.com/go-errors/errors"
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
	"github.com/ligato/vpp-agent/plugins/vpp/model/stn"
)

const ifTempName = "temp-if-name"

// Resync writes interfaces to the VPP. VPP interfaces are usually configured with tag, which corresponds with interface
// name (exceptions are physical devices, but their name is always equal to vpp internal name). Resync consists of
// following steps:
// 1. Dump all VPP interfaces
// 2. Every VPP interface looks for NB counterpart using tag (name). If found, it is calculated whether modification is
//    needed. Otherwise, the interface is only registered. If interface does not contain tag, it is stored for now and
//    resolved later. Tagged interfaces without NB config are removed.
// 3. Untagged interfaces are correlated heuristically (mac address, ip addresses). If correlation
//    is found, interface is modified if needed and registered.
// 4. All remaining NB interfaces are configured
func (c *InterfaceConfigurator) Resync(nbIfs []*intf.Interfaces_Interface) error {
	// Re-initialize cache
	if err := c.clearMapping(); err != nil {
		return err
	}
	c.afPacketConfigurator.clearMapping()

	var err error
	if c.memifScCache, err = c.ifHandler.DumpMemifSocketDetails(); err != nil {
		return errors.Errorf("interface resync error: failed to dump memif socket details: %v", err)
	}

	// Dump current state of the VPP interfaces
	vppIfs, err := c.ifHandler.DumpInterfaces()
	if err != nil {
		return errors.Errorf("interface resync error: failed to dump interfaces: %v", err)
	}

	// Cache for untagged interfaces. All un-named interfaces have to be correlated
	unnamedVppIfs := make(map[uint32]*intf.Interfaces_Interface)

	// Iterate over VPP interfaces and try to correlate NB config
	for vppIfIdx, vppIf := range vppIfs {
		if vppIfIdx == 0 {
			// Register local0 interface with zero index
			if err := c.registerInterface(vppIf.Meta.InternalName, vppIfIdx, vppIf.Interface); err != nil {
				return errors.Errorf("interface resync error: %v", err)
			}
			continue
		}
		// Register IPSec tunnel interfaces, but do not resync them (they are resynced in IPSec plugin)
		if vppIf.Interface.Type == intf.InterfaceType_IPSEC_TUNNEL {
			if err := c.registerInterface(vppIf.Meta.InternalName, vppIfIdx, vppIf.Interface); err != nil {
				return errors.Errorf("interface resync error: %v", err)
			}
			continue
		}
		if vppIf.Interface.Name == "" {
			// If interface has no name, it is stored as unnamed and resolved later
			c.log.Debugf("Interface resync: interface %v has no name (tag)", vppIfIdx)
			unnamedVppIfs[vppIfIdx] = vppIf.Interface
			continue
		}
		var correlated bool
		for _, nbIf := range nbIfs {
			if vppIf.Interface.Name == nbIf.Name {
				correlated = true
				// Register interface to mapping and VPP tag/index
				if err := c.registerInterface(vppIf.Interface.Name, vppIfIdx, nbIf); err != nil {
					return errors.Errorf("interface resync error: %v", err)
				}
				// Calculate whether modification is needed
				if c.isIfModified(nbIf, vppIf.Interface) {
					c.log.Debugf("Interface resync: modifying interface %v", vppIf.Interface.Name)
					if err = c.ModifyVPPInterface(nbIf, vppIf.Interface); err != nil {
						return errors.Errorf("interface resync error: failed to modify interface %s: %v",
							vppIf.Interface.Name, err)
					}
				} else {
					c.log.Debugf("Interface resync: %s registered without additional changes", vppIf.Interface.Name)
				}
				break
			}
		}
		if !correlated {
			// Register interface before removal (to keep state consistent)
			if err := c.registerInterface(vppIf.Interface.Name, vppIfIdx, vppIf.Interface); err != nil {
				return errors.Errorf("interface resync error: %v", err)
			}
			// VPP interface is obsolete and will be removed (un-configured if physical device)
			c.log.Debugf("RESYNC interfaces: removing obsolete interface %v", vppIf.Interface.Name)
			if err = c.deleteVPPInterface(vppIf.Interface, vppIfIdx); err != nil {
				return errors.Errorf("interface resync error: failed to remove interface %s: %v",
					vppIf.Interface.Name, err)
			}
		}
	}

	// Now resolve untagged interfaces
	for vppIfIdx, vppIf := range unnamedVppIfs {
		// Try to find NB config which is not registered and correlates with VPP interface
		var correlatedIf *intf.Interfaces_Interface
		for _, nbIf := range nbIfs {
			// Already registered interfaces cannot be correlated again
			_, _, found := c.swIfIndexes.LookupIdx(nbIf.Name)
			if found {
				continue
			}
			// Try to correlate heuristically
			correlatedIf = c.correlateInterface(vppIf, nbIf)
			if correlatedIf != nil {
				break
			}
		}

		if correlatedIf != nil {
			// Register interface
			if err := c.registerInterface(correlatedIf.Name, vppIfIdx, correlatedIf); err != nil {
				return errors.Errorf("interface resync error: %v", err)
			}
			// Calculate whether modification is needed
			if c.isIfModified(correlatedIf, vppIf) {
				c.log.Debugf("Interface resync: modifying correlated interface %v", vppIf.Name)
				if err = c.ModifyVPPInterface(correlatedIf, vppIf); err != nil {
					return errors.Errorf("interface resync error: failed to modify correlated interface %s: %v",
						vppIf.Name, err)
				}
			} else {
				c.log.Debugf("interface resync: correlated %v registered without additional changes", vppIf.Name)
			}
		} else {
			// Register interface  with temporary name (will be unregistered during removal)
			if err := c.registerInterface(ifTempName, vppIfIdx, vppIf); err != nil {
				return errors.Errorf("interface resync error: %v", err)
			}
			// VPP interface cannot be correlated and will be removed
			c.log.Debugf("Interface resync: removing interface %v", vppIf.Name)
			if err = c.deleteVPPInterface(vppIf, vppIfIdx); err != nil {
				return errors.Errorf("interface resync error: failed to remove interface %s: %v",
					vppIf.Name, err)
			}
		}
	}

	// Last step is to configure all new (not-yet-registered) interfaces
	for _, nbIf := range nbIfs {
		// If interface is registered, it was already processed
		_, _, found := c.swIfIndexes.LookupIdx(nbIf.Name)
		if !found {
			c.log.Debugf("Interface resync: configuring new interface %v", nbIf.Name)
			if err := c.ConfigureVPPInterface(nbIf); err != nil {
				return errors.Errorf("interface resync error: failed to configure interface %s: %v",
					nbIf.Name, err)
			}
		}
	}

	// update the interfaces state data in memory
	if err := c.propagateIfDetailsToStatus(); err != nil {
		return errors.Errorf("interface resync error: %v", err)
	}

	c.log.Info("Interface resync done")

	return nil
}

// VerifyVPPConfigPresence dumps VPP interface configuration on the vpp. If there are any interfaces configured (except
// the local0), it returns false (do not interrupt the resto of the resync), otherwise returns true
func (c *InterfaceConfigurator) VerifyVPPConfigPresence(nbIfaces []*intf.Interfaces_Interface) bool {
	// notify that the resync should be stopped
	var stop bool

	// Step 0: Dump actual state of the VPP
	vppIfaces, err := c.ifHandler.DumpInterfaces()
	if err != nil {
		// Do not return error here
		return stop
	}

	// The strategy is optimize-cold-start, so look over all dumped VPP interfaces and check for the configured ones
	// (leave out the local0). If there are any other interfaces, return true (resync will continue).
	// If not, return a false flag which cancels the VPP resync operation.
	c.log.Info("optimize-cold-start VPP resync strategy chosen, resolving...")
	if len(vppIfaces) == 0 {
		stop = true
		c.log.Infof("...VPP resync interrupted assuming there is no configuration on the VPP (no interface was found)")
		return stop
	}
	// if interface exists, try to find local0 interface (index 0)
	_, ok := vppIfaces[0]
	// in case local0 is the only interface on the vpp, stop the resync
	if len(vppIfaces) == 1 && ok {
		stop = true
		c.log.Infof("...VPP resync interrupted assuming there is no configuration on the VPP (only local0 was found)")
		return stop
	}
	// otherwise continue normally
	c.log.Infof("... VPP configuration found, continue with VPP resync")

	return stop
}

// ResyncSession writes BFD sessions to the empty VPP
func (c *BFDConfigurator) ResyncSession(nbSessions []*bfd.SingleHopBFD_Session) error {
	// Re-initialize cache
	c.clearMapping()

	// Dump all BFD vppSessions
	vppBfdSessions, err := c.bfdHandler.DumpBfdSessions()
	if err != nil {
		return errors.Errorf("BFD resync error: failed to dump BFD sessions: %v", err)
	}

	// Correlate existing BFD sessions from the VPP and NB config, configure new ones
	for _, nbSession := range nbSessions {
		// look for configured session
		var found bool
		for _, vppSession := range vppBfdSessions.Session {
			// compare fixed fields
			if nbSession.Interface == vppSession.Interface && nbSession.SourceAddress == vppSession.SourceAddress &&
				nbSession.DestinationAddress == vppSession.DestinationAddress {
				c.sessionsIndexes.RegisterName(nbSession.Interface, c.bfdIDSeq, nil)
				if err := c.ModifyBfdSession(vppSession, nbSession); err != nil {
					return errors.Errorf("BFD resync error: failed to modify BFD session %s: %v",
						nbSession.Interface, err)
				}
				found = true
			}
		}
		if !found {
			// configure new BFD session
			if err := c.ConfigureBfdSession(nbSession); err != nil {
				return errors.Errorf("BFD resync error: failed to create BFD session %s: %v", nbSession.Interface, err)
			}
		}
	}

	// Remove old sessions
	for _, vppSession := range vppBfdSessions.Session {
		// remove every not-yet-registered session
		_, _, found := c.sessionsIndexes.LookupIdx(vppSession.Interface)
		if !found {
			if err := c.DeleteBfdSession(vppSession); err != nil {
				return errors.Errorf("BFD resync error: failed to delete BFD session %s: %v", vppSession.Interface, err)
			}
		}
	}

	c.log.Info("BFD session resync done")

	return nil
}

// ResyncAuthKey writes BFD keys to the empty VPP
func (c *BFDConfigurator) ResyncAuthKey(nbKeys []*bfd.SingleHopBFD_Key) error {
	// lookup BFD auth keys
	vppBfdKeys, err := c.bfdHandler.DumpBfdAuthKeys()
	if err != nil {
		return errors.Errorf("BFD resync error: failed to dump BFD authentication keys: %v", err)
	}

	// Correlate existing BFD keys from the VPP and NB config, configure new ones
	for _, nbKey := range nbKeys {
		// look for configured keys
		var found bool
		for _, vppKey := range vppBfdKeys.AuthKeys {
			// compare key ID
			if nbKey.Id == vppKey.Id {
				c.keysIndexes.RegisterName(AuthKeyIdentifier(nbKey.Id), c.bfdIDSeq, nil)
				if err := c.ModifyBfdAuthKey(vppKey, nbKey); err != nil {
					return errors.Errorf("BFD resync error: failed to modify BFD authentication key %s (ID %d): %v",
						nbKey.Name, nbKey.Id, err)
				}
				found = true
			}
		}
		if !found {
			// configure new BFD authentication key
			if err := c.ConfigureBfdAuthKey(nbKey); err != nil {
				return errors.Errorf("BFD resync error: failed to configure BFD authentication key %s (ID %d): %v",
					nbKey.Name, nbKey.Id, err)
			}
		}
	}

	// Remove old keys
	for _, vppKey := range vppBfdKeys.AuthKeys {
		// remove every not-yet-registered keys
		_, _, found := c.keysIndexes.LookupIdx(AuthKeyIdentifier(vppKey.Id))
		if !found {
			if err := c.DeleteBfdAuthKey(vppKey); err != nil {
				return errors.Errorf("BFD resync error: failed to delete BFD authentication key %s (ID %d): %v",
					vppKey.Name, vppKey.Id, err)
			}
		}
	}

	c.log.Info("BFD authentication key resync done")

	return nil
}

// ResyncEchoFunction writes BFD echo function to the empty VPP
func (c *BFDConfigurator) ResyncEchoFunction(echoFunctions []*bfd.SingleHopBFD_EchoFunction) error {
	if len(echoFunctions) == 0 {
		// Nothing to do here. Currently VPP does not support BFD echo dump so agent does not know
		// whether there is echo function already configured and cannot remove it
		return nil
	}
	// Only one config can be used to set an echo source. If there are multiple configurations,
	// use the first one
	if len(echoFunctions) > 1 {
		c.log.Warn("BFD resync: multiple configurations of BFD echo function found. Setting up %s as source",
			echoFunctions[0].EchoSourceInterface)
	}
	if err := c.ConfigureBfdEchoFunction(echoFunctions[0]); err != nil {
		return errors.Errorf("BFD resync error: failed to set interface %s as BFD echo source: %v",
			echoFunctions[0], err)
	}

	c.log.Info("BFD echo function resync done")

	return nil
}

// Resync writes stn rule to the the empty VPP
func (c *StnConfigurator) Resync(nbStnRules []*stn.STN_Rule) error {
	// Re-initialize cache
	c.clearMapping()

	// Dump existing STN Rules
	vppStnDetails, err := c.Dump()
	if err != nil {
		return err
	}

	// Correlate configuration, and remove obsolete rules STN rules
	var wasErr error
	for _, vppStnRule := range vppStnDetails.Rules {
		// Parse parameters
		var vppStnIP net.IP
		var vppStnIPStr string

		vppStnIfIdx, _, found := c.ifIndexes.LookupIdx(vppStnRule.Interface)
		if !found {
			// The rule is attached to non existing interface but it can be removed. If there is a similar
			// rule in NB config, it will be configured (or cached)
			if err := c.stnHandler.DelStnRule(vppStnIfIdx, &vppStnIP); err != nil {
				c.log.Error(err)
				wasErr = err
			}
			c.log.Debugf("RESYNC STN: rule IP: %v ifIdx: %v removed due to missing interface, will be reconfigured if needed",
				vppStnIPStr, vppStnIfIdx)
			continue
		}

		// Look for equal rule in NB configuration
		var match bool
		for _, nbStnRule := range nbStnRules {
			if nbStnRule.IpAddress == vppStnIPStr && nbStnRule.Interface == vppStnRule.Interface {
				// Register existing rule
				c.indexSTNRule(nbStnRule, false)
				match = true
			}
			c.log.Debugf("RESYNC STN: registered already existing rule %v", nbStnRule.RuleName)
		}

		// If STN rule does not exist, it is obsolete
		if !match {
			if err := c.stnHandler.DelStnRule(vppStnIfIdx, &vppStnIP); err != nil {
				c.log.Error(err)
				wasErr = err
			}
			c.log.Debugf("RESYNC STN: rule IP: %v ifName: %v removed as obsolete", vppStnIPStr, vppStnRule.Interface)
		}
	}

	// Configure missing rules
	for _, nbStnRule := range nbStnRules {
		identifier := StnIdentifier(nbStnRule.Interface)
		_, _, found := c.allIndexes.LookupIdx(identifier)
		if !found {
			if err := c.Add(nbStnRule); err != nil {
				c.log.Error(err)
				wasErr = err
			}
			c.log.Debugf("RESYNC STN: rule %v added", nbStnRule.RuleName)
		}
	}

	return wasErr
}

// ResyncNatGlobal writes NAT address pool config to the the empty VPP
func (c *NatConfigurator) ResyncNatGlobal(nbGlobal *nat.Nat44Global) error {
	// Re-initialize cache
	c.clearMapping()

	vppNatGlobal, err := c.natHandler.Nat44GlobalConfigDump()
	if err != nil {
		return errors.Errorf("NAT resync error: failed to dump NAT44 global config: %v", err)
	}

	// Modify will made all the diffs needed (nothing if content is equal)
	if err := c.ModifyNatGlobalConfig(vppNatGlobal, nbGlobal); err != nil {
		return err
	}

	c.log.Info("NAT global config resync done.")

	return nil
}

// ResyncSNat writes NAT static mapping config to the the empty VPP
func (c *NatConfigurator) ResyncSNat(sNatConf []*nat.Nat44SNat_SNatConfig) error {
	// todo SNAT not implemented yet, nothing to resync
	return nil
}

// ResyncDNat writes NAT static mapping config to the the empty VPP
func (c *NatConfigurator) ResyncDNat(nbDNatConfig []*nat.Nat44DNat_DNatConfig) error {
	c.log.Debug("RESYNC DNAT config.")

	vppDNatCfg, err := c.natHandler.Nat44DNatDump()
	if err != nil {
		return errors.Errorf("NAT resync error: failed to dump NAT44 DNAT: %v", err)
	}
	if len(vppDNatCfg.DnatConfigs) == 0 {
		return nil
	}

	// Correlate with existing config
	for _, nbDNat := range nbDNatConfig {
		for _, vppDNat := range vppDNatCfg.DnatConfigs {
			if nbDNat.Label != vppDNat.Label {
				continue
			}
			// Compare all VPP mappings with the NB, register existing ones
			c.resolveMappings(nbDNat, &vppDNat.StMappings, &vppDNat.IdMappings)
			// Configure all missing DNAT mappings
			for _, nbMapping := range nbDNat.StMappings {
				mappingIdentifier := GetStMappingIdentifier(nbMapping)
				_, _, found := c.dNatStMappingIndexes.LookupIdx(mappingIdentifier)
				if !found {
					// Configure missing mapping
					if len(nbMapping.LocalIps) == 1 {
						if err := c.handleStaticMapping(nbMapping, "", true); err != nil {
							return err
						}
					} else {
						if err := c.handleStaticMappingLb(nbMapping, "", true); err != nil {
							return err
						}
					}
					// Register new DNAT mapping
					c.dNatStMappingIndexes.RegisterName(mappingIdentifier, c.natIndexSeq, nil)
					c.natIndexSeq++
					c.log.Debugf("NAT44 resync: new (lb)static mapping %s registered", mappingIdentifier)
				}
			}
			// Configure all missing DNAT identity mappings
			for _, nbIDMapping := range nbDNat.IdMappings {
				mappingIdentifier := GetIDMappingIdentifier(nbIDMapping)
				_, _, found := c.dNatIDMappingIndexes.LookupIdx(mappingIdentifier)
				if !found {
					// Configure missing mapping
					if err := c.handleIdentityMapping(nbIDMapping, "", true); err != nil {
						return err
					}

					// Register new DNAT mapping
					c.dNatIDMappingIndexes.RegisterName(mappingIdentifier, c.natIndexSeq, nil)
					c.natIndexSeq++
					c.log.Debugf("NAT44 resync: new identity mapping %s registered", mappingIdentifier)
				}
			}
			// Remove obsolete mappings from DNAT
			for _, vppMapping := range vppDNat.StMappings {
				// Static mapping
				if len(vppMapping.LocalIps) == 1 {
					if err := c.handleStaticMapping(vppMapping, "", false); err != nil {
						return err
					}
				} else {
					// Lb-static mapping
					if err := c.handleStaticMappingLb(vppMapping, "", false); err != nil {
						return err
					}
				}
			}
			for _, vppIDMapping := range vppDNat.IdMappings {
				// Identity mapping
				if err := c.handleIdentityMapping(vppIDMapping, "", false); err != nil {
					return err
				}
			}
			// At this point, the DNAT is completely configured and can be registered
			c.dNatIndexes.RegisterName(nbDNat.Label, c.natIndexSeq, nil)
			c.natIndexSeq++
			c.log.Debugf("NAT44 resync: DNAT %s synced", nbDNat.Label)
		}
	}

	// Remove obsolete DNAT configurations which are not registered
	for _, vppDNat := range vppDNatCfg.DnatConfigs {
		_, _, found := c.dNatIndexes.LookupIdx(vppDNat.Label)
		if !found {
			if err := c.DeleteDNat(vppDNat); err != nil {
				return err
			}
		}
	}

	c.log.Info("DNAT resync done.")

	return nil
}

// Looks for the same mapping in the VPP, register existing ones
func (c *NatConfigurator) resolveMappings(nbDNatConfig *nat.Nat44DNat_DNatConfig,
	vppMappings *[]*nat.Nat44DNat_DNatConfig_StaticMapping, vppIDMappings *[]*nat.Nat44DNat_DNatConfig_IdentityMapping) {
	// Iterate over static mappings in NB DNAT config
	for _, nbMapping := range nbDNatConfig.StMappings {
		if len(nbMapping.LocalIps) > 1 {
			// Load balanced
		MappingCompare:
			for vppIndex, vppLbMapping := range *vppMappings {
				// Compare VRF/SNAT fields
				if nbMapping.TwiceNat != vppLbMapping.TwiceNat {
					continue
				}
				// Compare external IP/Port
				if nbMapping.ExternalIp != vppLbMapping.ExternalIp || nbMapping.ExternalPort != vppLbMapping.ExternalPort {
					continue
				}
				// Compare protocol
				if nbMapping.Protocol != vppLbMapping.Protocol {
					continue
				}
				// Compare Local IP/Port and probability addresses
				if len(nbMapping.LocalIps) != len(vppLbMapping.LocalIps) {
					continue
				}
				for _, nbLocal := range nbMapping.LocalIps {
					var found bool
					for _, vppLocal := range vppLbMapping.LocalIps {
						if proto.Equal(nbLocal, vppLocal) {
							found = true
						}
					}
					if !found {
						continue MappingCompare
					}
				}
				// At this point, the NB mapping matched the VPP one, so register it
				mappingIdentifier := GetStMappingIdentifier(nbMapping)
				c.dNatStMappingIndexes.RegisterName(mappingIdentifier, c.natIndexSeq, nil)
				c.natIndexSeq++

				// Remove registered entry from vpp mapping (configurator knows which mappings were registered)
				dMappings := *vppMappings
				*vppMappings = append(dMappings[:vppIndex], dMappings[vppIndex+1:]...)
				c.log.Debugf("NAT44 resync: lb-mapping %v already configured", mappingIdentifier)
			}
		} else {
			// No load balancer
			for vppIndex, vppMapping := range *vppMappings {
				// Compare VRF/SNAT fields
				if nbMapping.TwiceNat != vppMapping.TwiceNat {
					continue
				}
				// Compare external IP/Port and interface
				if nbMapping.ExternalIp != vppMapping.ExternalIp || nbMapping.ExternalPort != vppMapping.ExternalPort {
					continue
				}
				// Compare external interface
				if nbMapping.ExternalInterface != vppMapping.ExternalInterface {
					continue
				}
				// Compare protocol
				if nbMapping.Protocol != vppMapping.Protocol {
					continue
				}
				// Compare Local IP/Port and probability addresses (there is only one local IP address in this case)
				if len(nbMapping.LocalIps) != 1 || len(vppMapping.LocalIps) != 1 {
					c.log.Warnf("NAT44 resync: mapping without load balancer contains more than 1 local IP address")
					continue
				}
				nbLocal := nbMapping.LocalIps[0]
				vppLocal := vppMapping.LocalIps[0]
				if !proto.Equal(nbLocal, vppLocal) {
					continue
				}

				// At this point, the NB mapping matched the VPP one, so register it
				mappingIdentifier := GetStMappingIdentifier(nbMapping)
				c.dNatStMappingIndexes.RegisterName(mappingIdentifier, c.natIndexSeq, nil)
				c.natIndexSeq++

				// Remove registered entry from vpp mapping (so configurator knows which mappings were registered)
				dMappings := *vppMappings
				*vppMappings = append(dMappings[:vppIndex], dMappings[vppIndex+1:]...)
				c.log.Debugf("NAT44 resync: mapping %v already configured", mappingIdentifier)
			}
		}
	}
	// Iterate over identity mappings in NB DNAT config
	for _, nbIDMapping := range nbDNatConfig.IdMappings {
		for vppIDIndex, vppIDMapping := range *vppIDMappings {
			// Compare VRF and address interface
			if nbIDMapping.VrfId != vppIDMapping.VrfId || nbIDMapping.AddressedInterface != vppIDMapping.AddressedInterface {
				continue
			}
			// Compare IP and port values
			if nbIDMapping.IpAddress != vppIDMapping.IpAddress || nbIDMapping.Port != vppIDMapping.Port {
				continue
			}
			// Compare protocol
			if nbIDMapping.Protocol != vppIDMapping.Protocol {
				continue
			}

			// At this point, the NB mapping matched the VPP one, so register it
			mappingIdentifier := GetIDMappingIdentifier(nbIDMapping)
			c.dNatIDMappingIndexes.RegisterName(mappingIdentifier, c.natIndexSeq, nil)
			c.natIndexSeq++

			// Remove registered entry from vpp mapping (configurator knows which mappings were registered)
			dIDMappings := *vppIDMappings
			*vppIDMappings = append(dIDMappings[:vppIDIndex], dIDMappings[vppIDIndex+1:]...)
			c.log.Debugf("NAT44 resync: identity mapping %v already configured", mappingIdentifier)
		}
	}
}

// Correlate interfaces according to MAC address, interface addresses
func (c *InterfaceConfigurator) correlateInterface(vppIf, nbIf *intf.Interfaces_Interface) *intf.Interfaces_Interface {
	// Correlate MAC address
	if nbIf.PhysAddress != "" {
		if nbIf.PhysAddress == vppIf.PhysAddress {
			return nbIf
		}
	}
	// Correlate IP addresses
	if len(nbIf.IpAddresses) == len(vppIf.IpAddresses) {
		ipMatch := true

	ipComparison:
		for _, nbIP := range nbIf.IpAddresses {
			var ipFound bool
			for _, vppIP := range vppIf.IpAddresses {
				pNbIP, nbIPNet, err := net.ParseCIDR(nbIP)
				if err != nil {
					c.log.Error(err)
					continue
				}
				pVppIP, vppIPNet, err := net.ParseCIDR(vppIP)
				if err != nil {
					c.log.Error(err)
					continue
				}
				if nbIPNet.Mask.String() == vppIPNet.Mask.String() && bytes.Compare(pNbIP, pVppIP) == 0 {
					ipFound = true
					break
				}
			}
			if !ipFound {
				// Break comparison if there is mismatch
				ipMatch = false
				break ipComparison
			}
		}

		if ipMatch {
			return nbIf
		}
	}
	// todo correlate also unnumbered interfaces if possible

	// Otherwise there is no match between interfaces
	return nil
}

// Compares two interfaces. If there is any difference, returns true, false otherwise
func (c *InterfaceConfigurator) isIfModified(nbIf, vppIf *intf.Interfaces_Interface) bool {
	c.log.Debugf("Interface RESYNC comparison started for interface %s", nbIf.Name)
	// Type
	if nbIf.Type != vppIf.Type {
		c.log.Debugf("Interface RESYNC comparison: type changed (NB: %v, VPP: %v)",
			nbIf.Type, vppIf.Type)
		return true
	}
	// Enabled
	if nbIf.Enabled != vppIf.Enabled {
		c.log.Debugf("Interface RESYNC comparison: enabled state changed (NB: %t, VPP: %t)",
			nbIf.Enabled, vppIf.Enabled)
		return true
	}
	// VRF
	if nbIf.Vrf != vppIf.Vrf {
		c.log.Debugf("Interface RESYNC comparison: VRF changed (NB: %d, VPP: %d)",
			nbIf.Vrf, vppIf.Vrf)
		return true
	}
	// Container IP address
	if nbIf.ContainerIpAddress != vppIf.ContainerIpAddress {
		c.log.Debugf("Interface RESYNC comparison: container IP changed (NB: %s, VPP: %s)",
			nbIf.ContainerIpAddress, vppIf.ContainerIpAddress)
		return true
	}
	// DHCP setup
	if nbIf.SetDhcpClient != vppIf.SetDhcpClient {
		c.log.Debugf("Interface RESYNC comparison: DHCP setup changed (NB: %t, VPP: %t)",
			nbIf.SetDhcpClient, vppIf.SetDhcpClient)
		return true
	}
	// MTU value (not valid for VxLAN)
	if nbIf.Mtu != vppIf.Mtu && nbIf.Type != intf.InterfaceType_VXLAN_TUNNEL {
		c.log.Debugf("Interface RESYNC comparison: MTU changed (NB: %d, VPP: %d)",
			nbIf.Mtu, vppIf.Mtu)
		return true
	}
	// MAC address (compare only if it is set in the NB configuration)
	nbMac := strings.ToUpper(nbIf.PhysAddress)
	vppMac := strings.ToUpper(vppIf.PhysAddress)
	if nbMac != "" && nbMac != vppMac {
		c.log.Debugf("Interface RESYNC comparison: Physical address changed (NB: %s, VPP: %s)", nbMac, vppMac)
		return true
	}

	// Check if NB or SB interface is unnumbered
	if nbIf.Unnumbered != nil && nbIf.Unnumbered.IsUnnumbered {
		if vppIf.Unnumbered == nil || (vppIf.Unnumbered != nil && !vppIf.Unnumbered.IsUnnumbered) {
			c.log.Debugf("Interface RESYNC comparison: NB %s is unnumbered, VPP %s is not", nbMac, vppMac)
			return true
		}
	} else {
		// NB interface is not unnumbered, check VPP
		if vppIf.Unnumbered != nil && vppIf.Unnumbered.IsUnnumbered {
			c.log.Debugf("Interface RESYNC comparison: NB %s is not unnumbered, but VPP %s is", nbMac, vppMac)
			return true
		}
	}
	// If both of them are, check IP-interface. Otherwise, check IP addresses
	if nbIf.Unnumbered != nil && nbIf.Unnumbered.IsUnnumbered {
		if nbIf.Unnumbered.InterfaceWithIp != vppIf.Unnumbered.InterfaceWithIp {
			c.log.Debugf("Interface RESYNC comparison: IP-unnumbered interface is different (NB: %s, VPP: %s)",
				nbIf.Unnumbered.InterfaceWithIp, vppIf.Unnumbered.InterfaceWithIp)
			return true
		}
		vppIf.IpAddresses = nil
	} else {
		// Remove IPv6 link local addresses (default values)
		for ipIdx, ipAddress := range vppIf.IpAddresses {
			if strings.HasPrefix(ipAddress, "fe80") {
				vppIf.IpAddresses = append(vppIf.IpAddresses[:ipIdx], vppIf.IpAddresses[ipIdx+1:]...)
			}
		}
		// Compare IP address count
		if len(nbIf.IpAddresses) != len(vppIf.IpAddresses) {
			c.log.Debugf("Interface RESYNC comparison: IP address count changed (NB: %d, VPP: %d)",
				len(nbIf.IpAddresses), len(vppIf.IpAddresses))
			return true
		}
		// Compare every single IP address. If equal, every address should have identical counterpart
		for _, nbIP := range nbIf.IpAddresses {
			var ipFound bool
			for _, vppIP := range vppIf.IpAddresses {
				pNbIP, nbIPNet, err := net.ParseCIDR(nbIP)
				if err != nil {
					c.log.Error(err)
					continue
				}
				pVppIP, vppIPNet, err := net.ParseCIDR(vppIP)
				if err != nil {
					c.log.Error(err)
					continue
				}
				if nbIPNet.Mask.String() == vppIPNet.Mask.String() && bytes.Compare(pNbIP, pVppIP) == 0 {
					ipFound = true
					break
				}
			}
			if !ipFound {
				c.log.Debugf("Interface RESYNC comparison: VPP interface %s does not contain IP %s", nbIf.Name, nbIP)
				return true
			}
		}
	}
	// RxMode settings
	if nbIf.RxModeSettings == nil && vppIf.RxModeSettings != nil || nbIf.RxModeSettings != nil && vppIf.RxModeSettings == nil {
		c.log.Debugf("Interface RESYNC comparison: RxModeSettings changed (NB: %v, VPP: %v)",
			nbIf.RxModeSettings, vppIf.RxModeSettings)
		return true
	}
	if nbIf.RxModeSettings != nil && vppIf.RxModeSettings != nil {
		// RxMode
		if nbIf.RxModeSettings.RxMode != vppIf.RxModeSettings.RxMode {
			c.log.Debugf("Interface RESYNC comparison: RxMode changed (NB: %v, VPP: %v)",
				nbIf.RxModeSettings.RxMode, vppIf.RxModeSettings.RxMode)
			return true

		}
		// QueueID
		if nbIf.RxModeSettings.QueueId != vppIf.RxModeSettings.QueueId {
			c.log.Debugf("Interface RESYNC comparison: QueueID changed (NB: %d, VPP: %d)",
				nbIf.RxModeSettings.QueueId, vppIf.RxModeSettings.QueueId)
			return true

		}
		// QueueIDValid
		if nbIf.RxModeSettings.QueueIdValid != vppIf.RxModeSettings.QueueIdValid {
			c.log.Debugf("Interface RESYNC comparison: QueueIDValid changed (NB: %d, VPP: %d)",
				nbIf.RxModeSettings.QueueIdValid, vppIf.RxModeSettings.QueueIdValid)
			return true

		}
	}

	switch nbIf.Type {
	case intf.InterfaceType_AF_PACKET_INTERFACE:
		if nbIf.Afpacket == nil && vppIf.Afpacket != nil || nbIf.Afpacket != nil && vppIf.Afpacket == nil {
			c.log.Debugf("Interface RESYNC comparison: AF-packet setup changed (NB: %v, VPP: %v)",
				nbIf.Afpacket, vppIf.Afpacket)
			return true
		}
		if nbIf.Afpacket != nil && vppIf.Afpacket != nil {
			// AF-packet host name
			if nbIf.Afpacket.HostIfName != vppIf.Afpacket.HostIfName {
				c.log.Debugf("Interface RESYNC comparison: AF-packet host name changed (NB: %s, VPP: %s)",
					nbIf.Afpacket.HostIfName, vppIf.Afpacket.HostIfName)
				return true
			}
		}
	case intf.InterfaceType_MEMORY_INTERFACE:
		if nbIf.Memif == nil && vppIf.Memif != nil || nbIf.Memif != nil && vppIf.Memif == nil {
			c.log.Debugf("Interface RESYNC comparison: memif setup changed (NB: %v, VPP: %v)",
				nbIf.Memif, vppIf.Memif)
			return true
		}
		if nbIf.Memif != nil && vppIf.Memif != nil {
			// Memif ID
			if nbIf.Memif.Id != vppIf.Memif.Id {
				c.log.Debugf("Interface RESYNC comparison: memif ID changed (NB: %d, VPP: %d)",
					nbIf.Memif.Id, vppIf.Memif.Id)
				return true
			}

			// Memif socket
			if nbIf.Memif.SocketFilename != vppIf.Memif.SocketFilename {
				c.log.Debugf("Interface RESYNC comparison: memif socket filename changed (NB: %s, VPP: %s)",
					nbIf.Memif.SocketFilename, vppIf.Memif.SocketFilename)
				return true
			}
			// Master
			if nbIf.Memif.Master != vppIf.Memif.Master {
				c.log.Debugf("Interface RESYNC comparison: memif master setup changed (NB: %t, VPP: %t)",
					nbIf.Memif.Master, vppIf.Memif.Master)
				return true
			}
			// Mode
			if nbIf.Memif.Mode != vppIf.Memif.Mode {
				c.log.Debugf("Interface RESYNC comparison: memif mode setup changed (NB: %v, VPP: %v)",
					nbIf.Memif.Mode, vppIf.Memif.Mode)
				return true
			}
			// Rx queues
			if nbIf.Memif.RxQueues != vppIf.Memif.RxQueues {
				c.log.Debugf("Interface RESYNC comparison: RxQueues changed (NB: %d, VPP: %d)",
					nbIf.Memif.RxQueues, vppIf.Memif.RxQueues)
				return true
			}
			// Tx queues
			if nbIf.Memif.TxQueues != vppIf.Memif.TxQueues {
				c.log.Debugf("Interface RESYNC comparison: TxQueues changed (NB: %d, VPP: %d)",
					nbIf.Memif.TxQueues, vppIf.Memif.TxQueues)
				return true
			}
			// todo secret, buffer size and ring size is not compared. VPP always returns 0 for buffer size
			// and 1 for ring size. Secret cannot be dumped at all.
		}
	case intf.InterfaceType_TAP_INTERFACE:
		if nbIf.Tap == nil && vppIf.Tap != nil || nbIf.Tap != nil && vppIf.Tap == nil {
			c.log.Debugf("Interface RESYNC comparison: tap setup changed (NB: %v, VPP: %v)",
				nbIf.Tap, vppIf.Tap)
			return true
		}
		if nbIf.Tap != nil && vppIf.Tap != nil {
			// Tap version
			if nbIf.Tap.Version == 2 && nbIf.Tap.Version != vppIf.Tap.Version {
				c.log.Debugf("Interface RESYNC comparison: tap version changed (NB: %d, VPP: %d)",
					nbIf.Tap.Version, vppIf.Tap.Version)
				return true
			}
			// Namespace and host name
			if nbIf.Tap.Namespace != vppIf.Tap.Namespace {
				c.log.Debugf("Interface RESYNC comparison: tap namespace changed (NB: %s, VPP: %s)",
					nbIf.Tap.Namespace, vppIf.Tap.Namespace)
				return true
			}
			// Namespace and host name
			if nbIf.Tap.HostIfName != vppIf.Tap.HostIfName {
				c.log.Debugf("Interface RESYNC comparison: tap host name changed (NB: %s, VPP: %s)",
					nbIf.Tap.HostIfName, vppIf.Tap.HostIfName)
				return true
			}
			// Rx ring size
			if nbIf.Tap.RxRingSize != nbIf.Tap.RxRingSize {
				c.log.Debugf("Interface RESYNC comparison: tap Rx ring size changed (NB: %d, VPP: %d)",
					nbIf.Tap.RxRingSize, vppIf.Tap.RxRingSize)
				return true
			}
			// Tx ring size
			if nbIf.Tap.TxRingSize != nbIf.Tap.TxRingSize {
				c.log.Debugf("Interface RESYNC comparison: tap Tx ring size changed (NB: %d, VPP: %d)",
					nbIf.Tap.TxRingSize, vppIf.Tap.TxRingSize)
				return true
			}
		}
	case intf.InterfaceType_VXLAN_TUNNEL:
		if nbIf.Vxlan == nil && vppIf.Vxlan != nil || nbIf.Vxlan != nil && vppIf.Vxlan == nil {
			c.log.Debugf("Interface RESYNC comparison: VxLAN setup changed (NB: %v, VPP: %v)",
				nbIf.Vxlan, vppIf.Vxlan)
			return true
		}
		if nbIf.Vxlan != nil && vppIf.Vxlan != nil {
			// VxLAN Vni
			if nbIf.Vxlan.Vni != vppIf.Vxlan.Vni {
				c.log.Debugf("Interface RESYNC comparison: VxLAN Vni changed (NB: %d, VPP: %d)",
					nbIf.Vxlan.Vni, vppIf.Vxlan.Vni)
				return true
			}
			// VxLAN Src Address
			if nbIf.Vxlan.SrcAddress != vppIf.Vxlan.SrcAddress {
				c.log.Debugf("Interface RESYNC comparison: VxLAN src address changed (NB: %s, VPP: %s)",
					nbIf.Vxlan.SrcAddress, vppIf.Vxlan.SrcAddress)
				return true
			}
			// VxLAN Dst Address
			if nbIf.Vxlan.DstAddress != vppIf.Vxlan.DstAddress {
				c.log.Debugf("Interface RESYNC comparison: VxLAN dst address changed (NB: %s, VPP: %s)",
					nbIf.Vxlan.DstAddress, vppIf.Vxlan.DstAddress)
				return true
			}
			// VxLAN Multicast
			if nbIf.Vxlan.Multicast != vppIf.Vxlan.Multicast {
				c.log.Debugf("Interface RESYNC comparison: VxLAN multicast address changed (NB: %s, VPP: %s)",
					nbIf.Vxlan.Multicast, vppIf.Vxlan.Multicast)
				return true
			}
		}
	}

	// At last, return false if interfaces are equal
	return false
}

// Register interface to mapping and add tag/index to the VPP
func (c *InterfaceConfigurator) registerInterface(ifName string, ifIdx uint32, ifData *intf.Interfaces_Interface) error {
	c.swIfIndexes.RegisterName(ifName, ifIdx, ifData)
	if err := c.ifHandler.SetInterfaceTag(ifName, ifIdx); err != nil {
		return errors.Errorf("error while adding interface tag %s, index %d: %v", ifName, ifIdx, err)
	}
	// Add AF-packet type interface to local cache
	if ifData.Type == intf.InterfaceType_AF_PACKET_INTERFACE {
		if c.linux != nil && c.afPacketConfigurator != nil && ifData.Afpacket != nil {
			// Interface is already present on the VPP so it cannot be marked as pending.
			c.afPacketConfigurator.addToCache(ifData, false)
		}
	}
	c.log.Debugf("RESYNC interfaces: registered interface %s (index %d)", ifName, ifIdx)
	return nil
}
