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

package ifplugin

import (
	"net"
	"strconv"
	"strings"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
)

// Mapping labels
const (
	dummyTag = "dummy-tag" // used for deletion where tag is not needed
)

// Default NAT virtual reassembly values
const (
	maxReassembly = 1024
	maxFragments  = 5
	timeout       = 2
)

// NatConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of NAT address pools and static entries with or without a load ballance,
// as modelled by the proto file "../common/model/nat/nat.proto"
// and stored in ETCD under the keys:
// - "/vnf-agent/{agent-label}/vpp/config/v1/nat/{vrf}/addrpool/" for NAT address pool
// - "/vnf-agent/{agent-label}/vpp/config/v1/nat/{vrf}/static/" for NAT static mapping
// - "/vnf-agent/{agent-label}/vpp/config/v1/nat/{vrf}/staticlb/" for NAT static mapping with
//   load balancer
// Updates received from the northbound API are compared with the VPP run-time configuration and differences
// are applied through the VPP binary API.
type NatConfigurator struct {
	log logging.Logger

	// Global config
	globalNAT *nat.Nat44Global

	// Mappings
	ifIndexes            ifaceidx.SwIfIndex
	sNatIndexes          idxvpp.NameToIdxRW // SNAT config indices
	sNatMappingIndexes   idxvpp.NameToIdxRW // SNAT indices for static mapping
	dNatIndexes          idxvpp.NameToIdxRW // DNAT indices
	dNatStMappingIndexes idxvpp.NameToIdxRW // DNAT indices for static mapping
	dNatIDMappingIndexes idxvpp.NameToIdxRW // DNAT indices for identity mapping
	natIndexSeq          uint32             // Nat name-to-idx mapping sequence
	natMappingTagSeq     uint32             // Static/identity mapping tag sequence

	// a map of missing interfaces which should be enabled for NAT (format ifName/data)
	notEnabledIfs map[string]*nat.Nat44Global_NatInterface
	// a map of NAT-enabled interfaces which should be disabled (format ifName/data)
	notDisabledIfs map[string]*nat.Nat44Global_NatInterface

	// VPP channels
	vppChan     govppapi.Channel
	vppDumpChan govppapi.Channel

	// VPP API handler
	natHandler vppcalls.NatVppAPI
}

// Init NAT configurator
func (c *NatConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, ifIndexes ifaceidx.SwIfIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("nat-conf")

	// Mappings
	c.ifIndexes = ifIndexes
	c.sNatIndexes = nametoidx.NewNameToIdx(c.log, "snat-indices", nil)
	c.sNatMappingIndexes = nametoidx.NewNameToIdx(c.log, "snat-mapping-indices", nil)
	c.dNatIndexes = nametoidx.NewNameToIdx(c.log, "dnat-indices", nil)
	c.dNatStMappingIndexes = nametoidx.NewNameToIdx(c.log, "dnat-st-mapping-indices", nil)
	c.dNatIDMappingIndexes = nametoidx.NewNameToIdx(c.log, "dnat-id-mapping-indices", nil)
	c.notEnabledIfs = make(map[string]*nat.Nat44Global_NatInterface)
	c.notDisabledIfs = make(map[string]*nat.Nat44Global_NatInterface)
	c.natIndexSeq, c.natMappingTagSeq = 1, 1

	// Init VPP API channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}
	if c.vppDumpChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create dump API channel: %v", err)
	}

	// VPP API handler
	c.natHandler = vppcalls.NewNatVppHandler(c.vppChan, c.vppDumpChan, c.ifIndexes, c.log)

	c.log.Info("NAT configurator initialized")

	return nil
}

// Close used resources
func (c *NatConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan, c.vppDumpChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose NAT configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *NatConfigurator) clearMapping() {
	c.sNatIndexes.Clear()
	c.sNatMappingIndexes.Clear()
	c.dNatIndexes.Clear()
	c.dNatStMappingIndexes.Clear()
	c.dNatIDMappingIndexes.Clear()
	c.notEnabledIfs = make(map[string]*nat.Nat44Global_NatInterface)
	c.notDisabledIfs = make(map[string]*nat.Nat44Global_NatInterface)

	c.log.Debugf("NAT configurator mapping cleared")
}

// GetGlobalNat makes current global nat accessible
func (c *NatConfigurator) GetGlobalNat() *nat.Nat44Global {
	return c.globalNAT
}

// IsInNotEnabledIfCache checks if interface is present in 'notEnabledIfs' cache
func (c *NatConfigurator) IsInNotEnabledIfCache(ifName string) bool {
	_, ok := c.notEnabledIfs[ifName]
	return ok
}

// IsInNotDisabledIfCache checks if interface is present in 'notDisabledIfs' cache
func (c *NatConfigurator) IsInNotDisabledIfCache(ifName string) bool {
	_, ok := c.notDisabledIfs[ifName]
	return ok
}

// IsDNatLabelRegistered checks if interface is present in 'notDisabledIfs' cache
func (c *NatConfigurator) IsDNatLabelRegistered(label string) bool {
	_, _, found := c.dNatIndexes.LookupIdx(label)
	return found
}

// IsDNatLabelStMappingRegistered checks if DNAT static mapping with provided id is registered
func (c *NatConfigurator) IsDNatLabelStMappingRegistered(id string) bool {
	_, _, found := c.dNatStMappingIndexes.LookupIdx(id)
	return found
}

// IsDNatLabelIDMappingRegistered checks if DNAT identity mapping with provided id is registered
func (c *NatConfigurator) IsDNatLabelIDMappingRegistered(id string) bool {
	_, _, found := c.dNatIDMappingIndexes.LookupIdx(id)
	return found
}

// SetNatGlobalConfig configures common setup for all NAT use cases
func (c *NatConfigurator) SetNatGlobalConfig(config *nat.Nat44Global) error {
	// Store global NAT configuration (serves as cache)
	c.globalNAT = config

	// Forwarding
	if err := c.natHandler.SetNat44Forwarding(config.Forwarding); err != nil {
		return errors.Errorf("failed to set NAT44 forwarding to %t: %d", config.Forwarding, err)
	}

	// Inside/outside interfaces
	if len(config.NatInterfaces) > 0 {
		if err := c.enableNatInterfaces(config.NatInterfaces); err != nil {
			return err
		}
	}

	if err := c.addAddressPool(config.AddressPools); err != nil {
		return err
	}

	// Virtual reassembly IPv4
	if config.VirtualReassemblyIpv4 != nil {
		if err := c.natHandler.SetVirtualReassemblyIPv4(config.VirtualReassemblyIpv4); err != nil {
			return errors.Errorf("failed to set NAT virtual reassembly for IPv4: %v", err)
		}
	}
	// Virtual reassembly IPv6
	if config.VirtualReassemblyIpv6 != nil {
		if err := c.natHandler.SetVirtualReassemblyIPv6(config.VirtualReassemblyIpv6); err != nil {
			return errors.Errorf("failed to set NAT virtual reassembly for IPv6: %v", err)
		}
	}

	c.log.Info("Setting up NAT global config done")

	return nil
}

// ModifyNatGlobalConfig modifies common setup for all NAT use cases
func (c *NatConfigurator) ModifyNatGlobalConfig(oldConfig, newConfig *nat.Nat44Global) (err error) {
	// Replace global NAT config
	c.globalNAT = newConfig

	// Forwarding
	if oldConfig.Forwarding != newConfig.Forwarding {
		if err := c.natHandler.SetNat44Forwarding(newConfig.Forwarding); err != nil {
			return errors.Errorf("failed to set NAT44 forwarding to %t: %d", newConfig.Forwarding, err)
		}
	}

	// Inside/outside interfaces
	toSetIn, toSetOut, toUnsetIn, toUnsetOut := diffInterfaces(oldConfig.NatInterfaces, newConfig.NatInterfaces)
	if err := c.disableNatInterfaces(toUnsetIn); err != nil {
		return err
	}
	if err := c.disableNatInterfaces(toUnsetOut); err != nil {
		return err
	}
	if err := c.enableNatInterfaces(toSetIn); err != nil {
		return err
	}
	if err := c.enableNatInterfaces(toSetOut); err != nil {
		return err
	}

	// Address pool
	toAdd, toRemove := diffAddressPools(oldConfig.AddressPools, newConfig.AddressPools)
	if err := c.delAddressPool(toRemove); err != nil {
		return err
	}
	if err := c.addAddressPool(toAdd); err != nil {
		return err
	}

	// Virtual reassembly IPv4
	if toConfigure := isVirtualReassModified(oldConfig.VirtualReassemblyIpv4, newConfig.VirtualReassemblyIpv4); toConfigure != nil {
		if err := c.natHandler.SetVirtualReassemblyIPv4(toConfigure); err != nil {
			return errors.Errorf("failed to set NAT virtual reassembly for IPv4: %v", err)
		}
	}
	// Virtual reassembly IPv6
	if toConfigure := isVirtualReassModified(oldConfig.VirtualReassemblyIpv6, newConfig.VirtualReassemblyIpv6); toConfigure != nil {
		if err := c.natHandler.SetVirtualReassemblyIPv6(toConfigure); err != nil {
			return errors.Errorf("failed to set NAT virtual reassembly for IPv6: %v", err)
		}
	}

	c.log.Info("NAT global config modified")

	return nil
}

// DeleteNatGlobalConfig removes common setup for all NAT use cases
func (c *NatConfigurator) DeleteNatGlobalConfig(config *nat.Nat44Global) (err error) {
	// Remove global NAT config
	c.globalNAT = nil

	// Inside/outside interfaces
	if len(config.NatInterfaces) > 0 {
		if err := c.disableNatInterfaces(config.NatInterfaces); err != nil {
			return err
		}
	}

	// Address pools
	if len(config.AddressPools) > 0 {
		if err := c.delAddressPool(config.AddressPools); err != nil {
			return err
		}
	}

	// Reset virtual reassembly to default
	if err := c.natHandler.SetVirtualReassemblyIPv4(getDefaultVr()); err != nil {
		return errors.Errorf("failed to reset NAT virtual reassembly for IPv4 to default: %v", err)
	}
	if err := c.natHandler.SetVirtualReassemblyIPv6(getDefaultVr()); err != nil {
		return errors.Errorf("failed to reset NAT virtual reassembly for IPv4 to default: %v", err)
	}

	c.log.Info("NAT global config removed")

	return nil
}

// ConfigureSNat configures new SNAT setup
func (c *NatConfigurator) ConfigureSNat(sNat *nat.Nat44SNat_SNatConfig) error {
	c.log.Warn("SNAT CREATE not implemented")
	return nil
}

// ModifySNat modifies existing SNAT setup
func (c *NatConfigurator) ModifySNat(oldSNat, newSNat *nat.Nat44SNat_SNatConfig) error {
	c.log.Warn("SNAT MODIFY not implemented")
	return nil
}

// DeleteSNat removes existing SNAT setup
func (c *NatConfigurator) DeleteSNat(sNat *nat.Nat44SNat_SNatConfig) error {
	c.log.Warn("SNAT DELETE not implemented")
	return nil
}

// ConfigureDNat configures new DNAT setup
func (c *NatConfigurator) ConfigureDNat(dNat *nat.Nat44DNat_DNatConfig) error {
	// Resolve static mapping
	if err := c.configureStaticMappings(dNat.Label, dNat.StMappings); err != nil {
		return err
	}

	// Resolve identity mapping
	if err := c.configureIdentityMappings(dNat.Label, dNat.IdMappings); err != nil {
		return err
	}

	// Register DNAT configuration
	c.dNatIndexes.RegisterName(dNat.Label, c.natIndexSeq, nil)
	c.natIndexSeq++
	c.log.Debugf("DNAT configuration registered (label: %v)", dNat.Label)

	c.log.Infof("DNAT %s configured", dNat.Label)

	return nil
}

// ModifyDNat modifies existing DNAT setup
func (c *NatConfigurator) ModifyDNat(oldDNat, newDNat *nat.Nat44DNat_DNatConfig) error {
	// Static mappings
	stmToAdd, stmToRemove := c.diffStatic(oldDNat.StMappings, newDNat.StMappings)

	if err := c.unconfigureStaticMappings(stmToRemove); err != nil {
		return err
	}

	if err := c.configureStaticMappings(newDNat.Label, stmToAdd); err != nil {
		return err
	}

	// Identity mappings
	idToAdd, idToRemove := c.diffIdentity(oldDNat.IdMappings, newDNat.IdMappings)

	if err := c.unconfigureIdentityMappings(idToRemove); err != nil {
		return err
	}

	if err := c.configureIdentityMappings(newDNat.Label, idToAdd); err != nil {
		return err
	}

	c.log.Infof("DNAT %s modification done", newDNat.Label)

	return nil
}

// DeleteDNat removes existing DNAT setup
func (c *NatConfigurator) DeleteDNat(dNat *nat.Nat44DNat_DNatConfig) error {
	// In delete case, vpp-agent attempts to reconstruct every static mapping entry and remove it from the VPP
	if err := c.unconfigureStaticMappings(dNat.StMappings); err != nil {
		return err
	}

	// Do the same also for identity apping
	if err := c.unconfigureIdentityMappings(dNat.IdMappings); err != nil {
		return err
	}

	// Unregister DNAT configuration
	c.dNatIndexes.UnregisterName(dNat.Label)
	c.log.Debugf("DNAT configuration unregistered (label: %v)", dNat.Label)

	c.log.Infof("DNAT %v removed", dNat.Label)

	return nil
}

// ResolveCreatedInterface looks for cache of interfaces which should be enabled or disabled
// for NAT
func (c *NatConfigurator) ResolveCreatedInterface(ifName string, ifIdx uint32) error {
	// Check for interfaces which should be enabled
	var enabledIf []*nat.Nat44Global_NatInterface
	for cachedName, data := range c.notEnabledIfs {
		if cachedName == ifName {
			delete(c.notEnabledIfs, cachedName)
			c.log.Debugf("interface %s removed from not-enabled-for-NAT cache", ifName)
			if err := c.enableNatInterfaces(append(enabledIf, data)); err != nil {
				return errors.Errorf("failed to enable cached interface %s for NAT: %v", ifName, err)
			}
		}
	}
	// Check for interfaces which could be disabled
	var disabledIf []*nat.Nat44Global_NatInterface
	for cachedName, data := range c.notDisabledIfs {
		if cachedName == ifName {
			delete(c.notDisabledIfs, cachedName)
			c.log.Debugf("interface %s removed from not-disabled-for-NAT cache", ifName)
			if err := c.disableNatInterfaces(append(disabledIf, data)); err != nil {
				return errors.Errorf("failed to disable cached interface %s for NAT: %v", ifName, err)
			}
		}
	}

	return nil
}

// ResolveDeletedInterface handles removed interface from NAT perspective
func (c *NatConfigurator) ResolveDeletedInterface(ifName string, ifIdx uint32) error {
	// Check global NAT for interfaces
	if c.globalNAT != nil {
		for _, natIf := range c.globalNAT.NatInterfaces {
			if natIf.Name == ifName {
				// This interface was removed and it is not possible to determine its state, so agent handles it as
				// not enabled
				c.notEnabledIfs[natIf.Name] = natIf
				c.log.Debugf("unregistered interface %s added to not-enabled-for-NAT cache", ifName)
				return nil
			}
		}
	}

	return nil
}

// DumpNatGlobal returns the current NAT44 global config
func (c *NatConfigurator) DumpNatGlobal() (*nat.Nat44Global, error) {
	return c.natHandler.Nat44GlobalConfigDump()
}

// DumpNatDNat returns the current NAT44 DNAT config
func (c *NatConfigurator) DumpNatDNat() (*nat.Nat44DNat, error) {
	return c.natHandler.Nat44DNatDump()
}

// enables set of interfaces as inside/outside in NAT
func (c *NatConfigurator) enableNatInterfaces(natInterfaces []*nat.Nat44Global_NatInterface) error {
	for _, natInterface := range natInterfaces {
		ifIdx, _, found := c.ifIndexes.LookupIdx(natInterface.Name)
		if !found {
			c.notEnabledIfs[natInterface.Name] = natInterface // cache interface
			c.log.Debugf("Interface %s missing, cannot enable it for NAT yet, cached", natInterface.Name)
		} else {
			if natInterface.OutputFeature {
				// enable nat interface and output feature
				if err := c.natHandler.EnableNat44InterfaceOutput(ifIdx, natInterface.IsInside); err != nil {
					return errors.Errorf("failed to enable interface %s for NAT44 as output feature: %v",
						natInterface.Name, err)
				}
			} else {
				// enable interface only
				if err := c.natHandler.EnableNat44Interface(ifIdx, natInterface.IsInside); err != nil {
					return errors.Errorf("failed to enable interface %s for NAT44: %v", natInterface.Name, err)
				}
			}
		}
	}

	return nil
}

// disables set of interfaces in NAT
func (c *NatConfigurator) disableNatInterfaces(natInterfaces []*nat.Nat44Global_NatInterface) error {
	for _, natInterface := range natInterfaces {
		// Check if interface is not in the cache
		for ifName := range c.notEnabledIfs {
			if ifName == natInterface.Name {
				delete(c.notEnabledIfs, ifName)
			}
		}
		// Check if interface exists
		ifIdx, _, found := c.ifIndexes.LookupIdx(natInterface.Name)
		if !found {
			c.notDisabledIfs[natInterface.Name] = natInterface // cache interface
			c.log.Debugf("Interface %s missing, cannot disable it for NAT yet, cached", natInterface.Name)
		} else {
			if natInterface.OutputFeature {
				// disable nat interface and output feature
				if err := c.natHandler.DisableNat44InterfaceOutput(ifIdx, natInterface.IsInside); err != nil {
					return errors.Errorf("failed to disable NAT44 interface %s as output feature: %v",
						natInterface.Name, err)
				}
			} else {
				// disable interface
				if err := c.natHandler.DisableNat44Interface(ifIdx, natInterface.IsInside); err != nil {
					return errors.Errorf("failed to disable NAT44 interface %s: %v", natInterface.Name, err)
				}
			}
		}
	}

	return nil
}

// Configures NAT address pool. If an address pool cannot is invalid and cannot be configured, it is skipped.
func (c *NatConfigurator) addAddressPool(addressPools []*nat.Nat44Global_AddressPool) (err error) {
	for _, addressPool := range addressPools {
		if addressPool.FirstSrcAddress == "" && addressPool.LastSrcAddress == "" {
			return errors.Errorf("failed to add address pool: invalid config, no IP address provided")
		}
		var firstIP []byte
		var lastIP []byte
		if addressPool.FirstSrcAddress != "" {
			firstIP = net.ParseIP(addressPool.FirstSrcAddress).To4()
			if firstIP == nil {
				return errors.Errorf("failed to add address pool: unable to parse IP address %s",
					addressPool.FirstSrcAddress)
			}
		}
		if addressPool.LastSrcAddress != "" {
			lastIP = net.ParseIP(addressPool.LastSrcAddress).To4()
			if lastIP == nil {
				return errors.Errorf("failed to add address pool: unable to parse IP address %s",
					addressPool.LastSrcAddress)
			}
		}
		// Both fields have to be set, at least at the same value if only one of them is set
		if firstIP == nil {
			firstIP = lastIP
		} else if lastIP == nil {
			lastIP = firstIP // Matthew 20:16
		}
		if err = c.natHandler.AddNat44AddressPool(firstIP, lastIP, addressPool.VrfId, addressPool.TwiceNat); err != nil {
			return errors.Errorf("failed to add NAT44 address pool %s - %s: %v", firstIP, lastIP, err)
		}
	}

	return nil
}

// Removes NAT address pool. Invalid address pool configuration is skipped with warning, configurator assumes that
// such a data could not be configured to the vpp.
func (c *NatConfigurator) delAddressPool(addressPools []*nat.Nat44Global_AddressPool) error {
	for _, addressPool := range addressPools {
		if addressPool.FirstSrcAddress == "" && addressPool.LastSrcAddress == "" {
			// No address pool to remove
			continue
		}
		var firstIP []byte
		var lastIP []byte
		if addressPool.FirstSrcAddress != "" {
			firstIP = net.ParseIP(addressPool.FirstSrcAddress).To4()
			if firstIP == nil {
				// Do not return error here
				c.log.Warnf("First NAT44 address pool IP %s cannot be parsed and removed, skipping",
					addressPool.FirstSrcAddress)
				continue
			}
		}
		if addressPool.LastSrcAddress != "" {
			lastIP = net.ParseIP(addressPool.LastSrcAddress).To4()
			if lastIP == nil {
				// Do not return error here
				c.log.Warnf("Last NAT44 address pool IP %s cannot be parsed and removed, skipping",
					addressPool.LastSrcAddress)
				continue
			}
		}
		// Both fields have to be set, at least at the same value if only one of them is set
		if firstIP == nil {
			firstIP = lastIP
		} else if lastIP == nil {
			lastIP = firstIP
		}

		// remove address pool
		if err := c.natHandler.DelNat44AddressPool(firstIP, lastIP, addressPool.VrfId, addressPool.TwiceNat); err != nil {
			errors.Errorf("failed to delete NAT44 address pool %s - %s: %v", firstIP, lastIP, err)
		}
	}

	return nil
}

// configures a list of static mappings for provided label
func (c *NatConfigurator) configureStaticMappings(label string, mappings []*nat.Nat44DNat_DNatConfig_StaticMapping) error {
	for _, mappingEntry := range mappings {
		localIPList := mappingEntry.LocalIps
		if len(localIPList) == 0 {
			return errors.Errorf("cannot configure DNAT static mapping %s: no local address provided", label)
		} else if len(localIPList) == 1 {
			// Case without load balance (one local address)
			if err := c.handleStaticMapping(mappingEntry, label, true); err != nil {
				return err
			}
		} else {
			// Case with load balance (more local addresses)
			if err := c.handleStaticMappingLb(mappingEntry, label, true); err != nil {
				return err
			}
		}
		// Register DNAT static mapping
		mappingIdentifier := GetStMappingIdentifier(mappingEntry)
		c.dNatStMappingIndexes.RegisterName(mappingIdentifier, c.natIndexSeq, nil)
		c.natIndexSeq++
		c.log.Debugf("DNAT static (lb) mapping registered (ID: %s, Tag: %s)", mappingIdentifier, label)
	}

	return nil
}

// removes static mappings from configuration with provided label
func (c *NatConfigurator) unconfigureStaticMappings(mappings []*nat.Nat44DNat_DNatConfig_StaticMapping) error {
	for mappingIdx, mappingEntry := range mappings {
		localIPList := mappingEntry.LocalIps
		if len(localIPList) == 0 {
			c.log.Warnf("DNAT mapping %s has not local IPs, cannot remove it", mappingIdx)
			continue
		} else if len(localIPList) == 1 {
			// Case without load balance (one local address)
			if err := c.handleStaticMapping(mappingEntry, dummyTag, false); err != nil {
				return err
			}
		} else {
			// Case with load balance (more local addresses)
			if err := c.handleStaticMappingLb(mappingEntry, dummyTag, false); err != nil {
				return err
			}
		}
		// Unregister DNAT mapping
		mappingIdentifier := GetStMappingIdentifier(mappingEntry)
		c.dNatStMappingIndexes.UnregisterName(mappingIdentifier)
		c.log.Debugf("DNAT lb-mapping unregistered (ID %v)", mappingIdentifier)
	}

	return nil
}

// configures single static mapping entry with load balancer
func (c *NatConfigurator) handleStaticMappingLb(staticMappingLb *nat.Nat44DNat_DNatConfig_StaticMapping, tag string, add bool) (err error) {
	// Validate tag
	if tag == dummyTag && add {
		c.log.Warn("Static mapping will be configured with generic tag")
	}
	// Parse external IP address
	exIPAddrByte := net.ParseIP(staticMappingLb.ExternalIp).To4()
	if exIPAddrByte == nil {
		return errors.Errorf("cannot configure DNAT lb static mapping: unable to parse external IP %s", exIPAddrByte.String())
	}

	// In this case, external port is required
	if staticMappingLb.ExternalPort == 0 {
		return errors.Errorf("cannot configure DNAT lb static mapping: external port is not set")
	}

	// Address mapping with load balancer
	ctx := &vppcalls.StaticMappingLbContext{
		Tag:          tag,
		ExternalIP:   exIPAddrByte,
		ExternalPort: uint16(staticMappingLb.ExternalPort),
		Protocol:     getProtocol(staticMappingLb.Protocol, c.log),
		LocalIPs:     getLocalIPs(staticMappingLb.LocalIps, c.log),
		TwiceNat:     staticMappingLb.TwiceNat == nat.TwiceNatMode_ENABLED,
		SelfTwiceNat: staticMappingLb.TwiceNat == nat.TwiceNatMode_SELF,
	}

	if len(ctx.LocalIPs) == 0 {
		return errors.Errorf("cannot configure DNAT mapping: no local IP was successfully parsed")
	}

	if add {
		if err := c.natHandler.AddNat44StaticMappingLb(ctx); err != nil {
			return errors.Errorf("failed to add NAT44 lb static mapping: %v", err)
		}
	} else {
		if err := c.natHandler.DelNat44StaticMappingLb(ctx); err != nil {
			return errors.Errorf("failed to delete NAT44 static mapping: %v", err)
		}
	}
	return nil
}

// handler for single static mapping entry
func (c *NatConfigurator) handleStaticMapping(staticMapping *nat.Nat44DNat_DNatConfig_StaticMapping, tag string, add bool) (err error) {
	var ifIdx uint32 = 0xffffffff // default value - means no external interface is set
	var exIPAddr net.IP

	// Validate tag
	if tag == dummyTag && add {
		c.log.Warn("Static mapping will be configured with generic tag")
	}

	// Parse local IP address and port
	lcIPAddr := net.ParseIP(staticMapping.LocalIps[0].LocalIp).To4()
	lcPort := staticMapping.LocalIps[0].LocalPort
	lcVrf := staticMapping.LocalIps[0].VrfId
	if lcIPAddr == nil {
		return errors.Errorf("cannot configure DNAT static mapping: unable to parse local IP %s", lcIPAddr.String())
	}

	// Check external interface (prioritized over external IP)
	if staticMapping.ExternalInterface != "" {
		// Check external interface
		var found bool
		ifIdx, _, found = c.ifIndexes.LookupIdx(staticMapping.ExternalInterface)
		if !found {
			return errors.Errorf("cannot configure DNAT static mapping: required external interface %s is missing",
				staticMapping.ExternalInterface)
		}
	} else {
		// Parse external IP address
		exIPAddr = net.ParseIP(staticMapping.ExternalIp).To4()
		if exIPAddr == nil {
			return errors.Errorf("cannot configure DNAT static mapping: unable to parse external IP %s", exIPAddr.String())
		}
	}

	// Resolve mapping (address only or address and port)
	var addrOnly bool
	if lcPort == 0 || staticMapping.ExternalPort == 0 {
		addrOnly = true
	}

	// Address mapping with load balancer
	ctx := &vppcalls.StaticMappingContext{
		Tag:           tag,
		AddressOnly:   addrOnly,
		LocalIP:       lcIPAddr,
		LocalPort:     uint16(lcPort),
		ExternalIP:    exIPAddr,
		ExternalPort:  uint16(staticMapping.ExternalPort),
		ExternalIfIdx: ifIdx,
		Protocol:      getProtocol(staticMapping.Protocol, c.log),
		Vrf:           lcVrf,
		TwiceNat:      staticMapping.TwiceNat == nat.TwiceNatMode_ENABLED,
		SelfTwiceNat:  staticMapping.TwiceNat == nat.TwiceNatMode_SELF,
	}

	if add {
		if err := c.natHandler.AddNat44StaticMapping(ctx); err != nil {
			return errors.Errorf("failed to add NAT44 static mapping: %v", err)
		}
	} else {
		if err := c.natHandler.DelNat44StaticMapping(ctx); err != nil {
			return errors.Errorf("failed to delete NAT44 static mapping: %v", err)
		}
	}
	return nil
}

// configures a list of identity mappings with label
func (c *NatConfigurator) configureIdentityMappings(label string, mappings []*nat.Nat44DNat_DNatConfig_IdentityMapping) error {
	for _, idMapping := range mappings {
		if idMapping.IpAddress == "" && idMapping.AddressedInterface == "" {
			return errors.Errorf("cannot configure DNAT %s identity mapping: no IP address or interface provided", label)
		}
		// Case without load balance (one local address)
		if err := c.handleIdentityMapping(idMapping, label, true); err != nil {
			return err
		}

		// Register DNAT identity mapping
		mappingIdentifier := GetIDMappingIdentifier(idMapping)
		c.dNatIDMappingIndexes.RegisterName(mappingIdentifier, c.natIndexSeq, nil)
		c.natIndexSeq++
		c.log.Debugf("DNAT identity mapping registered (ID: %s, Tag: %s)", mappingIdentifier, label)
	}

	return nil
}

// removes identity mappings from configuration with provided label
func (c *NatConfigurator) unconfigureIdentityMappings(mappings []*nat.Nat44DNat_DNatConfig_IdentityMapping) error {
	var wasErr error
	for mappingIdx, idMapping := range mappings {
		if idMapping.IpAddress == "" && idMapping.AddressedInterface == "" {
			return errors.Errorf("cannot configure DNAT identity mapping %d: no IP address or interface provided",
				mappingIdx)
		}
		if err := c.handleIdentityMapping(idMapping, dummyTag, false); err != nil {
			return err
		}

		// Unregister DNAT identity mapping
		mappingIdentifier := GetIDMappingIdentifier(idMapping)
		c.dNatIDMappingIndexes.UnregisterName(mappingIdentifier)
		c.natIndexSeq++
		c.log.Debugf("DNAT identity mapping unregistered (ID: %v)", mappingIdentifier)
	}

	return wasErr
}

// handler for single identity mapping entry
func (c *NatConfigurator) handleIdentityMapping(idMapping *nat.Nat44DNat_DNatConfig_IdentityMapping, tag string, isAdd bool) (err error) {
	// Verify interface if exists
	var ifIdx uint32
	if idMapping.AddressedInterface != "" {
		var found bool
		ifIdx, _, found = c.ifIndexes.LookupIdx(idMapping.AddressedInterface)
		if !found {
			// TODO: use cache to configure later
			return errors.Errorf("failed to configure identity mapping: provided interface %s does not exist",
				idMapping.AddressedInterface)
		}
	}

	// Identity mapping (common fields)
	ctx := &vppcalls.IdentityMappingContext{
		Tag:      tag,
		Protocol: getProtocol(idMapping.Protocol, c.log),
		Port:     uint16(idMapping.Port),
		IfIdx:    ifIdx,
		Vrf:      idMapping.VrfId,
	}

	if ctx.IfIdx == 0 {
		// Case with IP (optionally port). Verify and parse input IP/port
		parsedIP := net.ParseIP(idMapping.IpAddress).To4()
		if parsedIP == nil {
			return errors.Errorf("failed to configure identity mapping: unable to parse IP address %s", idMapping.IpAddress)
		}
		// Add IP address to context
		ctx.IPAddress = parsedIP
	}

	// Configure/remove identity mapping
	if isAdd {
		if err := c.natHandler.AddNat44IdentityMapping(ctx); err != nil {
			return errors.Errorf("failed to add NAT44 identity mapping: %v", err)
		}
	} else {
		if err := c.natHandler.DelNat44IdentityMapping(ctx); err != nil {
			return errors.Errorf("failed to remove NAT44 identity mapping: %v", err)
		}
	}
	return nil
}

// looks for new and obsolete IN interfaces
func diffInterfaces(oldIfs, newIfs []*nat.Nat44Global_NatInterface) (toSetIn, toSetOut, toUnsetIn, toUnsetOut []*nat.Nat44Global_NatInterface) {
	// Find new interfaces
	for _, newIf := range newIfs {
		var found bool
		for _, oldIf := range oldIfs {
			if newIf.Name == oldIf.Name && newIf.IsInside == oldIf.IsInside && newIf.OutputFeature == oldIf.OutputFeature {
				found = true
				break
			}
		}
		if !found {
			if newIf.IsInside {
				toSetIn = append(toSetIn, newIf)
			} else {
				toSetOut = append(toSetOut, newIf)
			}
		}
	}
	// Find obsolete interfaces
	for _, oldIf := range oldIfs {
		var found bool
		for _, newIf := range newIfs {
			if oldIf.Name == newIf.Name && oldIf.IsInside == newIf.IsInside && oldIf.OutputFeature == newIf.OutputFeature {
				found = true
				break
			}
		}
		if !found {
			if oldIf.IsInside {
				toUnsetIn = append(toUnsetIn, oldIf)
			} else {
				toUnsetOut = append(toUnsetOut, oldIf)
			}
		}
	}

	return
}

// looks for new and obsolete address pools
func diffAddressPools(oldAPs, newAPs []*nat.Nat44Global_AddressPool) (toAdd, toRemove []*nat.Nat44Global_AddressPool) {
	// Find new address pools
	for _, newAp := range newAPs {
		// If new address pool is a range, add it
		if newAp.LastSrcAddress != "" {
			toAdd = append(toAdd, newAp)
			continue
		}
		// Otherwise try to find the same address pool
		var found bool
		for _, oldAp := range oldAPs {
			// Skip address pools
			if oldAp.LastSrcAddress != "" {
				continue
			}
			if newAp.FirstSrcAddress == oldAp.FirstSrcAddress && newAp.TwiceNat == oldAp.TwiceNat && newAp.VrfId == oldAp.VrfId {
				found = true
			}
		}
		if !found {
			toAdd = append(toAdd, newAp)
		}
	}
	// Find obsolete address pools
	for _, oldAp := range oldAPs {
		// If new address pool is a range, remove it
		if oldAp.LastSrcAddress != "" {
			toRemove = append(toRemove, oldAp)
			continue
		}
		// Otherwise try to find the same address pool
		var found bool
		for _, newAp := range newAPs {
			// Skip address pools (they are already added)
			if oldAp.LastSrcAddress != "" {
				continue
			}
			if oldAp.FirstSrcAddress == newAp.FirstSrcAddress && oldAp.TwiceNat == newAp.TwiceNat && oldAp.VrfId == newAp.VrfId {
				found = true
			}
		}
		if !found {
			toRemove = append(toRemove, oldAp)
		}
	}

	return
}

// returns a list of static mappings to add/remove
func (c *NatConfigurator) diffStatic(oldMappings, newMappings []*nat.Nat44DNat_DNatConfig_StaticMapping) (toAdd, toRemove []*nat.Nat44DNat_DNatConfig_StaticMapping) {
	// Find missing mappings
	for _, newMap := range newMappings {
		var found bool
		for _, oldMap := range oldMappings {
			// VRF, protocol and twice map
			if newMap.Protocol != oldMap.Protocol || newMap.TwiceNat != oldMap.TwiceNat {
				continue
			}
			// External interface, IP and port
			if newMap.ExternalInterface != oldMap.ExternalInterface || newMap.ExternalIp != oldMap.ExternalIp ||
				newMap.ExternalPort != oldMap.ExternalPort {
				continue
			}
			// Local IPs
			if !c.compareLocalIPs(oldMap.LocalIps, newMap.LocalIps) {
				continue
			}
			found = true
		}
		if !found {
			toAdd = append(toAdd, newMap)
		}
	}
	// Find obsolete mappings
	for _, oldMap := range oldMappings {
		var found bool
		for _, newMap := range newMappings {
			// VRF, protocol and twice map
			if newMap.Protocol != oldMap.Protocol || newMap.TwiceNat != oldMap.TwiceNat {
				continue
			}
			// External interface, IP and port
			if newMap.ExternalInterface != oldMap.ExternalInterface || newMap.ExternalIp != oldMap.ExternalIp ||
				newMap.ExternalPort != oldMap.ExternalPort {
				continue
			}
			// Local IPs
			if !c.compareLocalIPs(oldMap.LocalIps, newMap.LocalIps) {
				continue
			}
			found = true
		}
		if !found {
			toRemove = append(toRemove, oldMap)
		}
	}

	return
}

// returns a list of identity mappings to add/remove
func (c *NatConfigurator) diffIdentity(oldMappings, newMappings []*nat.Nat44DNat_DNatConfig_IdentityMapping) (toAdd, toRemove []*nat.Nat44DNat_DNatConfig_IdentityMapping) {
	// Find missing mappings
	for _, newMap := range newMappings {
		var found bool
		for _, oldMap := range oldMappings {
			// VRF and protocol
			if newMap.VrfId != oldMap.VrfId || newMap.Protocol != oldMap.Protocol {
				continue
			}
			// Addressed interface, IP address and port
			if newMap.AddressedInterface != oldMap.AddressedInterface || newMap.IpAddress != oldMap.IpAddress ||
				newMap.Port != oldMap.Port {
				continue
			}
			found = true
		}
		if !found {
			toAdd = append(toAdd, newMap)
		}
	}
	// Find obsolete mappings
	for _, oldMap := range oldMappings {
		var found bool
		for _, newMap := range newMappings {
			// VRF and protocol
			if newMap.VrfId != oldMap.VrfId || newMap.Protocol != oldMap.Protocol {
				continue
			}
			// Addressed interface, IP address and port
			if newMap.AddressedInterface != oldMap.AddressedInterface || newMap.IpAddress != oldMap.IpAddress ||
				newMap.Port != oldMap.Port {
				continue
			}
			found = true
		}
		if !found {
			toRemove = append(toRemove, oldMap)
		}
	}

	return
}

// diffVirtualReassembly compares virtual reassembly from old config, returns virtual reassembly to be configured, or nil
// if no changes are needed
func isVirtualReassModified(oldReass, newReass *nat.Nat44Global_VirtualReassembly) *nat.Nat44Global_VirtualReassembly {
	// If new value is set while the old value does not exist, or it is different, return new value to configure
	if newReass != nil && (oldReass == nil || !proto.Equal(oldReass, newReass)) {
		return newReass
	}
	// If old value was set but new is not, return default
	if oldReass != nil && newReass == nil {
		return getDefaultVr()
	}
	return nil
}

// compares two lists of Local IP addresses, returns true if lists are equal, false otherwise
func (c *NatConfigurator) compareLocalIPs(oldIPs, newIPs []*nat.Nat44DNat_DNatConfig_StaticMapping_LocalIP) bool {
	if len(oldIPs) != len(newIPs) {
		return false
	}
	for _, newIP := range newIPs {
		var found bool
		for _, oldIP := range oldIPs {
			if newIP.VrfId == oldIP.VrfId && newIP.LocalIp == oldIP.LocalIp && newIP.LocalPort == oldIP.LocalPort && newIP.Probability == oldIP.Probability {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	// Do not need to compare old vs. new if length is the same
	return true
}

// returns a list of validated local IP addresses with port and probability value
func getLocalIPs(ipPorts []*nat.Nat44DNat_DNatConfig_StaticMapping_LocalIP, log logging.Logger) (locals []*vppcalls.LocalLbAddress) {
	for _, ipPort := range ipPorts {
		if ipPort.LocalPort == 0 {
			log.Error("cannot set local IP/Port to mapping: port is missing")
			continue
		}

		localIP := net.ParseIP(ipPort.LocalIp).To4()
		if localIP == nil {
			log.Errorf("cannot set local IP/Port to mapping: unable to parse local IP %v", ipPort.LocalIp)
			continue
		}

		locals = append(locals, &vppcalls.LocalLbAddress{
			Vrf:         ipPort.VrfId,
			LocalIP:     localIP,
			LocalPort:   uint16(ipPort.LocalPort),
			Probability: uint8(ipPort.Probability),
		})
	}

	return
}

// returns num representation of provided protocol value
func getProtocol(protocol nat.Protocol, log logging.Logger) uint8 {
	switch protocol {
	case nat.Protocol_TCP:
		return vppcalls.TCP
	case nat.Protocol_UDP:
		return vppcalls.UDP
	case nat.Protocol_ICMP:
		return vppcalls.ICMP
	default:
		log.Warnf("Unknown protocol %v, defaulting to TCP", protocol)
		return vppcalls.TCP
	}
}

// GetStMappingIdentifier returns unique ID of the mapping
func GetStMappingIdentifier(mapping *nat.Nat44DNat_DNatConfig_StaticMapping) string {
	extIP := strings.Replace(mapping.ExternalIp, ".", "", -1)
	extIP = strings.Replace(extIP, "/", "", -1)
	locIP := strings.Replace(mapping.LocalIps[0].LocalIp, ".", "", -1)
	locIP = strings.Replace(locIP, "/", "", -1)
	return extIP + locIP + strconv.Itoa(int(mapping.LocalIps[0].VrfId))
}

// GetIDMappingIdentifier returns unique ID of the mapping
func GetIDMappingIdentifier(mapping *nat.Nat44DNat_DNatConfig_IdentityMapping) string {
	extIP := strings.Replace(mapping.IpAddress, ".", "", -1)
	extIP = strings.Replace(extIP, "/", "", -1)
	if mapping.AddressedInterface == "" {
		return extIP + "-noif-" + strconv.Itoa(int(mapping.VrfId))
	}
	return extIP + "-" + mapping.AddressedInterface + "-" + strconv.Itoa(int(mapping.VrfId))
}

// getDefaultVr returns default nat virtual reassembly configuration.
func getDefaultVr() *nat.Nat44Global_VirtualReassembly {
	return &nat.Nat44Global_VirtualReassembly{
		MaxReass: maxReassembly,
		MaxFrag:  maxFragments,
		Timeout:  timeout,
		DropFrag: false,
	}
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *NatConfigurator) LogError(err error) error {
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
