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

package l3plugin

import (
	"fmt"
	"net"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/l3idx"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// ArpConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of L3 arp entries as modelled by the proto file "../model/l3/l3.proto" and stored
// in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1/arp". Updates received from the northbound API
// are compared with the VPP run-time configuration and differences are applied through the VPP binary API.
type ArpConfigurator struct {
	log logging.Logger

	// In-memory mappings
	ifIndexes ifaceidx.SwIfIndex
	// ARPIndexes is a list of ARP entries which are successfully configured on the VPP
	arpIndexes l3idx.ARPIndexRW
	// ARPCache is a list of ARP entries with are present in the ETCD, but not on VPP
	// due to missing interface
	arpCache l3idx.ARPIndexRW
	// ARPDeleted is a list of unsuccessfully deleted ARP entries. ARP entry cannot be removed
	// if the interface is missing (it runs into 'unnassigned' state). If the interface re-appears,
	// such an ARP have to be removed
	arpDeleted  l3idx.ARPIndexRW
	arpIndexSeq uint32

	// VPP channel
	vppChan govppapi.Channel
	// VPP API handler
	arpHandler vppcalls.ArpVppAPI
}

// Init initializes ARP configurator
func (c *ArpConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("l3-arp-conf")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.arpIndexes = l3idx.NewARPIndex(nametoidx.NewNameToIdx(c.log, "arp_indexes", nil))
	c.arpCache = l3idx.NewARPIndex(nametoidx.NewNameToIdx(c.log, "arp_cache", nil))
	c.arpDeleted = l3idx.NewARPIndex(nametoidx.NewNameToIdx(c.log, "arp_unnasigned", nil))
	c.arpIndexSeq = 1

	// VPP channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handler
	c.arpHandler = vppcalls.NewArpVppHandler(c.vppChan, c.ifIndexes, c.log)

	c.log.Info("VPP ARP configurator initialized")

	return nil
}

// Close GOVPP channel
func (c *ArpConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose VPP ARP configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *ArpConfigurator) clearMapping() {
	c.arpIndexes.Clear()
	c.arpCache.Clear()
	c.arpDeleted.Clear()
	c.log.Debugf("VPP ARP configurator mapping cleared")
}

// GetArpIndexes exposes arpIndexes mapping
func (c *ArpConfigurator) GetArpIndexes() l3idx.ARPIndexRW {
	return c.arpIndexes
}

// GetArpCache exposes list of cached ARP entries (present in ETCD but not in VPP)
func (c *ArpConfigurator) GetArpCache() l3idx.ARPIndexRW {
	return c.arpCache
}

// GetArpDeleted exposes arppDeleted mapping (unsuccessfully deleted ARP entries)
func (c *ArpConfigurator) GetArpDeleted() l3idx.ARPIndexRW {
	return c.arpDeleted
}

// Creates unique identifier which serves as a name in name to index mapping
func arpIdentifier(iface, ip, mac string) string {
	return fmt.Sprintf("arp-iface-%v-%v-%v", iface, ip, mac)
}

// AddArp processes the NB config and propagates it to bin api call
func (c *ArpConfigurator) AddArp(entry *l3.ArpTable_ArpEntry) error {
	if err := isValidARP(entry, c.log); err != nil {
		return err
	}

	arpID := arpIdentifier(entry.Interface, entry.PhysAddress, entry.IpAddress)

	// look for ARP in list of deleted ARPs
	_, _, exists := c.arpDeleted.UnregisterName(arpID)
	if exists {
		c.log.Debugf("ARP entry %v unregistered from (del) cache", arpID)
	}

	// verify interface presence
	ifIndex, _, exists := c.ifIndexes.LookupIdx(entry.Interface)
	if !exists {
		// Store ARP entry to cache
		c.arpCache.RegisterName(arpID, c.arpIndexSeq, entry)
		c.log.Debugf("ARP %S stored to cache, interface %s not found", arpID, entry.Interface)
		c.arpIndexSeq++
		return nil
	}

	// Transform arp data
	arp, err := transformArp(entry, ifIndex)
	if err != nil {
		return errors.Errorf("failed to transform ARP entry %s", arpID)
	}
	if arp == nil {
		return nil
	}

	// Create and register new arp entry
	if err = c.arpHandler.VppAddArp(arp); err != nil {
		return errors.Errorf("failed to configure VPP ARP %s: %v", arpID, err)
	}

	// Register configured ARP
	c.arpIndexes.RegisterName(arpID, c.arpIndexSeq, entry)
	c.arpIndexSeq++
	c.log.Debugf("ARP entry %s registered", arpID)

	c.log.Infof("ARP entry %s configured", arpID)
	return nil
}

// ChangeArp processes the NB config and propagates it to bin api call
func (c *ArpConfigurator) ChangeArp(entry *l3.ArpTable_ArpEntry, prevEntry *l3.ArpTable_ArpEntry) error {
	if err := c.DeleteArp(prevEntry); err != nil {
		return errors.Errorf("failed to delete ARP entry (MAC %s): %v", entry.PhysAddress, err)
	}
	if err := c.AddArp(entry); err != nil {
		return errors.Errorf("failed to add ARP entry (MAC %s): %v", entry.PhysAddress, err)
	}

	c.log.Infof("VPP ARP entry ( MAC %s) modified", entry.PhysAddress, *entry)
	return nil
}

// DeleteArp processes the NB config and propagates it to bin api call
func (c *ArpConfigurator) DeleteArp(entry *l3.ArpTable_ArpEntry) error {
	if err := isValidARP(entry, c.log); err != nil {
		// Note: such an ARP cannot be configured either, so it should not happen
		return err
	}

	// ARP entry identifier
	arpID := arpIdentifier(entry.Interface, entry.PhysAddress, entry.IpAddress)

	// Check if ARP entry is not just cached
	_, _, found := c.arpCache.LookupIdx(arpID)
	if found {
		c.arpCache.UnregisterName(arpID)
		c.log.Debugf("ARP entry %s found in cache, removed", arpID)
		// Cached ARP is not configured on the VPP, return
		return nil
	}

	// Check interface presence
	ifIndex, _, exists := c.ifIndexes.LookupIdx(entry.Interface)
	if !exists {
		// ARP entry cannot be removed without interface. Since the data are
		// no longer in the ETCD, agent need to remember the state and remove
		// entry when possible
		c.log.Debugf("Cannot remove ARP entry %s due to missing interface %s, will be removed when possible",
			arpID, entry.Interface)
		c.arpIndexes.UnregisterName(arpID)
		c.arpDeleted.RegisterName(arpID, c.arpIndexSeq, entry)
		c.log.Debugf("ARP entry %s removed from mapping and added to (del) cache", arpID)
		c.arpIndexSeq++

		return nil
	}

	// Transform arp data
	arp, err := transformArp(entry, ifIndex)
	if err != nil {
		return errors.Errorf("failed to transform ARP entry %s: %v", arpID, err)
	}
	if arp == nil {
		return nil
	}

	// Delete and un-register new arp
	if err = c.arpHandler.VppDelArp(arp); err != nil {
		return errors.Errorf("failed to delete VPP ARP %s: %v", arpID, err)
	}
	c.arpIndexes.UnregisterName(arpID)
	c.log.Debugf("ARP entry %v unregistered", arpID)

	c.log.Infof("ARP entry %v removed", arpID)

	return nil
}

// ResolveCreatedInterface handles case when new interface appears in the config
func (c *ArpConfigurator) ResolveCreatedInterface(ifName string) error {
	// find all entries which can be resolved
	entriesToAdd := c.arpCache.LookupNamesByInterface(ifName)
	entriesToRemove := c.arpDeleted.LookupNamesByInterface(ifName)

	// Configure all cached ARP entriesToAdd which can be configured
	for _, entry := range entriesToAdd {
		// ARP entry identifier. Every entry in cache was already validated
		arpID := arpIdentifier(entry.Interface, entry.PhysAddress, entry.IpAddress)
		if err := c.AddArp(entry); err != nil {
			return errors.Errorf("failed to add VPP ARP entry %s with registered interface %s: %v",
				arpID, ifName, err)
		}

		// remove from cache
		c.arpCache.UnregisterName(arpID)
		c.log.Debugf("Cached ARP %s was configured and unregistered from cache", arpID)
	}

	// Remove all entries which should not be configured
	for _, entry := range entriesToRemove {
		arpID := arpIdentifier(entry.Interface, entry.PhysAddress, entry.IpAddress)
		if err := c.DeleteArp(entry); err != nil {
			return errors.Errorf("failed to delete VPP ARP entry %s with registered interface %s: %v",
				arpID, ifName, err)
		}

		// remove from list of deleted
		c.arpDeleted.UnregisterName(arpID)
		c.log.Debugf("Cached ARP %s was removed and unregistered from (del) cache", arpID)
	}

	return nil
}

// ResolveDeletedInterface handles case when interface is removed from the config
func (c *ArpConfigurator) ResolveDeletedInterface(interfaceName string, interfaceIdx uint32) error {
	// Since the interface does not exist, all related ARP entries are 'un-assigned' on the VPP
	// but they cannot be removed using binary API. Nothing to do here.

	return nil
}

// Verify ARP entry contains all required fields
func isValidARP(arpInput *l3.ArpTable_ArpEntry, log logging.Logger) error {
	if arpInput == nil {
		log.Info("ARP input is empty")
		return errors.Errorf("ARP invalid: input is empty")
	}
	if arpInput.PhysAddress == "" {
		return errors.Errorf("ARP invalid: no MAC address provided")
	}
	if arpInput.Interface == "" {
		return errors.Errorf("ARP (MAC %s) invalid: no interface provided", arpInput.PhysAddress)
	}
	if arpInput.IpAddress == "" {
		return errors.Errorf("ARP (MAC %s) invalid: no IP address provided", arpInput.PhysAddress)
	}

	return nil
}

// transformArp converts raw entry data to ARP object
func transformArp(arpInput *l3.ArpTable_ArpEntry, ifIndex uint32) (*vppcalls.ArpEntry, error) {
	ipAddr := net.ParseIP(arpInput.IpAddress)
	arp := &vppcalls.ArpEntry{
		Interface:  ifIndex,
		IPAddress:  ipAddr,
		MacAddress: arpInput.PhysAddress,
		Static:     arpInput.Static,
	}
	return arp, nil
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *ArpConfigurator) LogError(err error) error {
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
