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

	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin/l3idx"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
	"github.com/vishvananda/netlink"
)

const (
	noIfaceIdxFilter = 0
	noFamilyFilter   = 0
)

// LinuxArpConfigurator watches for any changes in the configuration of static ARPs as modelled by the proto file
// "model/l3/l3.proto" and stored in ETCD under the key "/vnf-agent/{vnf-agent}/linux/config/v1/arp".
// Updates received from the northbound API are compared with the Linux network configuration and differences
// are applied through the Netlink AP
type LinuxArpConfigurator struct {
	log logging.Logger

	// Mappings
	ifIndexes  ifaceidx.LinuxIfIndexRW
	arpIndexes l3idx.LinuxARPIndexRW
	arpIfCache map[string]*ArpToInterface // Cache for non-configurable ARPs due to missing interface
	arpIdxSeq  uint32

	// Linux namespace/calls handler
	l3Handler linuxcalls.NetlinkAPI
	nsHandler nsplugin.NamespaceAPI
}

// ArpToInterface is an object which stores ARP-to-interface pairs used in cache.
// Field 'isAdd' marks whether the entry should be added or removed
type ArpToInterface struct {
	arp    *l3.LinuxStaticArpEntries_ArpEntry
	ifName string
	isAdd  bool
}

// GetArpIndexes returns arp in-memory indexes
func (c *LinuxArpConfigurator) GetArpIndexes() l3idx.LinuxARPIndexRW {
	return c.arpIndexes
}

// GetArpInterfaceCache returns internal non-configurable interface cache, mainly for testing purpose
func (c *LinuxArpConfigurator) GetArpInterfaceCache() map[string]*ArpToInterface {
	return c.arpIfCache
}

// Init initializes ARP configurator and starts goroutines
func (c *LinuxArpConfigurator) Init(logger logging.PluginLogger, l3Handler linuxcalls.NetlinkAPI, nsHandler nsplugin.NamespaceAPI,
	arpIndexes l3idx.LinuxARPIndexRW, ifIndexes ifaceidx.LinuxIfIndexRW) error {
	// Logger
	c.log = logger.NewLogger("arp-conf")

	// In-memory mappings
	c.ifIndexes = ifIndexes
	c.arpIndexes = arpIndexes
	c.arpIfCache = make(map[string]*ArpToInterface)
	c.arpIdxSeq = 1

	// L3 and namespace handler
	c.l3Handler = l3Handler
	c.nsHandler = nsHandler

	c.log.Debug("Linux ARP configurator initialized")

	return nil
}

// Close closes all goroutines started during Init
func (c *LinuxArpConfigurator) Close() error {
	return nil
}

// ConfigureLinuxStaticArpEntry reacts to a new northbound Linux ARP entry config by creating and configuring
// the entry in the host network stack through Netlink API.
func (c *LinuxArpConfigurator) ConfigureLinuxStaticArpEntry(arpEntry *l3.LinuxStaticArpEntries_ArpEntry) error {
	var err error
	// Prepare ARP entry object
	neigh := &netlink.Neigh{}

	// Find interface
	_, ifData, found := c.ifIndexes.LookupIdx(arpEntry.Interface)
	if !found || ifData == nil {
		c.log.Debugf("cannot create ARP entry %s due to missing interface %s (found: %v, data: %v), cached",
			arpEntry.Name, arpEntry.Interface, found, ifData)
		c.arpIfCache[arpEntry.Name] = &ArpToInterface{
			arp:    arpEntry,
			ifName: arpEntry.Interface,
			isAdd:  true,
		}
		return nil
	}

	neigh.LinkIndex = int(ifData.Index)

	// Set IP address
	ipAddr := net.ParseIP(arpEntry.IpAddr)
	if ipAddr == nil {
		return fmt.Errorf("cannot create ARP entry %v, unable to parse IP address %v", arpEntry.Name, arpEntry.IpAddr)
	}
	neigh.IP = ipAddr

	// Set MAC address
	var mac net.HardwareAddr
	if mac, err = net.ParseMAC(arpEntry.HwAddress); err != nil {
		return errors.Errorf("failed to create linux ARP entry %s, unable to parse MAC address %s, error: %v",
			arpEntry.Name, arpEntry.HwAddress, err)
	}
	neigh.HardwareAddr = mac

	// Set ARP entry state
	neigh.State = arpStateParser(arpEntry.State)

	// Set ip family
	neigh.Family = getIPFamily(arpEntry.IpFamily)

	// Prepare namespace of related interface
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	arpNs := c.nsHandler.ArpNsToGeneric(arpEntry.Namespace)

	// ARP entry has to be created in the same namespace as the interface
	revertNs, err := c.nsHandler.SwitchNamespace(arpNs, nsMgmtCtx)
	if err != nil {
		return errors.Errorf("failed to switch namespace: %v", err)
	}
	defer revertNs()

	// Create a new ARP entry in interface namespace
	err = c.l3Handler.AddArpEntry(arpEntry.Name, neigh)
	if err != nil {
		return errors.Errorf("failed to add linux ARP entry %s: %v", arpEntry.Name, err)
	}

	// Register created ARP entry
	c.arpIndexes.RegisterName(ArpIdentifier(neigh), c.arpIdxSeq, arpEntry)
	c.arpIdxSeq++
	c.log.Debugf("ARP entry %v registered as %v", arpEntry.Name, ArpIdentifier(neigh))

	c.log.Infof("Linux ARP entry %s configured", arpEntry.Name)

	return nil
}

// ModifyLinuxStaticArpEntry applies changes in the NB configuration of a Linux ARP through Netlink API.
func (c *LinuxArpConfigurator) ModifyLinuxStaticArpEntry(newArpEntry *l3.LinuxStaticArpEntries_ArpEntry, oldArpEntry *l3.LinuxStaticArpEntries_ArpEntry) (err error) {
	// If the namespace of the new ARP entry was changed, the old entry needs to be removed and the new one created
	// in the new namespace
	// If interface or IP address was changed, the old entry needs to be removed and recreated as well. In such a case,
	// ModifyArpEntry (analogy to 'ip neigh replace') would create a new entry instead of modifying the existing one
	callReplace := true

	oldArpNs := c.nsHandler.ArpNsToGeneric(oldArpEntry.Namespace)
	newArpNs := c.nsHandler.ArpNsToGeneric(newArpEntry.Namespace)
	result := oldArpNs.CompareNamespaces(newArpNs)
	if result != 0 || oldArpEntry.Interface != newArpEntry.Interface || oldArpEntry.IpAddr != newArpEntry.IpAddr {
		callReplace = false
	}

	// Remove old entry and configure a new one, then return
	if !callReplace {
		if err := c.DeleteLinuxStaticArpEntry(oldArpEntry); err != nil {
			return errors.Errorf("linux ARP modify: failed to remove old entry %s: %v", oldArpEntry.Name, err)
		}
		if err := c.ConfigureLinuxStaticArpEntry(newArpEntry); err != nil {
			return errors.Errorf("linux ARP modify: failed to add new entry %s: %v", oldArpEntry.Name, err)
		}
		return nil
	}

	// Create modified ARP entry object
	neigh := &netlink.Neigh{}

	// Find interface
	_, ifData, found := c.ifIndexes.LookupIdx(newArpEntry.Interface)
	if !found || ifData == nil {
		c.log.Debugf("cannot create ARP entry %s due to missing interface %s, cached",
			newArpEntry.Name, newArpEntry.Interface, found, ifData)
		return nil
	}
	neigh.LinkIndex = int(ifData.Index)

	// Set IP address
	ipAddr := net.ParseIP(newArpEntry.IpAddr)
	if ipAddr == nil {
		return errors.Errorf("cannot create ARP entry %s, unable to parse IP address %s", newArpEntry.Name, newArpEntry.IpAddr)
	}
	neigh.IP = ipAddr

	// Set MAC address
	var mac net.HardwareAddr
	if mac, err = net.ParseMAC(newArpEntry.HwAddress); err != nil {
		return errors.Errorf("cannot create ARP entry %s, unable to parse MAC address %s: %v", newArpEntry.Name,
			newArpEntry.HwAddress, err)
	}
	neigh.HardwareAddr = mac

	// Set ARP entry state
	neigh.State = arpStateParser(newArpEntry.State)

	// Set ip family
	neigh.Family = getIPFamily(newArpEntry.IpFamily)

	// Prepare namespace of related interface
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	arpNs := c.nsHandler.ArpNsToGeneric(newArpEntry.Namespace)

	// ARP entry has to be created in the same namespace as the interface
	revertNs, err := c.nsHandler.SwitchNamespace(arpNs, nsMgmtCtx)
	if err != nil {
		return errors.Errorf("failed to switch namespace: %v", err)
	}
	defer revertNs()

	err = c.l3Handler.SetArpEntry(newArpEntry.Name, neigh)
	if err != nil {
		return errors.Errorf("modifying arp entry %q failed: %v (%+v)", newArpEntry.Name, err, neigh)
	}

	c.log.Infof("Linux ARP entry %s modified", newArpEntry.Name)

	return nil
}

// DeleteLinuxStaticArpEntry reacts to a removed NB configuration of a Linux ARP entry.
func (c *LinuxArpConfigurator) DeleteLinuxStaticArpEntry(arpEntry *l3.LinuxStaticArpEntries_ArpEntry) (err error) {
	// Prepare ARP entry object
	neigh := &netlink.Neigh{}

	// Find interface
	_, ifData, foundIface := c.ifIndexes.LookupIdx(arpEntry.Interface)
	if !foundIface || ifData == nil {
		c.log.Debugf("cannot remove ARP entry %v due to missing interface %v, cached", arpEntry.Name, arpEntry.Interface)
		c.arpIfCache[arpEntry.Name] = &ArpToInterface{
			arp:    arpEntry,
			ifName: arpEntry.Interface,
		}
		return nil
	}
	neigh.LinkIndex = int(ifData.Index)

	// Set IP address
	ipAddr := net.ParseIP(arpEntry.IpAddr)
	if ipAddr == nil {
		return errors.Errorf("cannot create ARP entry %s, unable to parse IP address %s", arpEntry.Name, arpEntry.IpAddr)
	}
	neigh.IP = ipAddr

	// Prepare namespace of related interface
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	arpNs := c.nsHandler.ArpNsToGeneric(arpEntry.Namespace)

	// ARP entry has to be removed from the same namespace as the interface
	revertNs, err := c.nsHandler.SwitchNamespace(arpNs, nsMgmtCtx)
	if err != nil {
		return errors.Errorf("failed to switch namespace: %v", err)
	}
	defer revertNs()

	// Read all ARP entries configured for interface
	entries, err := c.l3Handler.GetArpEntries(int(ifData.Index), noFamilyFilter)
	if err != nil {
		return errors.Errorf("failed to read ARP entries for index %d: %v", ifData.Index, err)
	}

	// Look for ARP to remove. If it already does not exist, return
	var found bool
	for _, entry := range entries {
		if compareARPLinkIdxAndIP(&entry, neigh) {
			found = true
			break
		}
	}
	if !found {
		c.log.Debugf("ARP entry with IP %v and link index %v not configured, skipped", neigh.IP.String(), neigh.LinkIndex)
		return nil
	}

	// Remove the ARP entry from the interface namespace
	err = c.l3Handler.DelArpEntry(arpEntry.Name, neigh)
	if err != nil {
		return errors.Errorf("failed to remove linux ARP entry %s: %v", arpEntry.Name, err)
	}

	_, _, found = c.arpIndexes.UnregisterName(ArpIdentifier(neigh))
	if found {
		c.log.Debugf("Linux ARP entry  %s unregistered", arpEntry.Name)
	}

	c.log.Infof("Linux ARP entry %s removed", arpEntry.Name)

	return nil
}

// LookupLinuxArpEntries reads all ARP entries from all interfaces and registers them if needed
func (c *LinuxArpConfigurator) LookupLinuxArpEntries() error {
	c.log.Debugf("Browsing Linux ARP entries")

	// Set interface index and family to 0 reads all arp entries from all of the interfaces
	entries, err := c.l3Handler.GetArpEntries(noIfaceIdxFilter, noFamilyFilter)
	if err != nil {
		return errors.Errorf("failed to read linux ARP entries: %v", err)
	}

	for _, entry := range entries {
		_, arp, found := c.arpIndexes.LookupIdx(ArpIdentifier(&entry))
		if !found {
			var ifName string
			if arp == nil || arp.Namespace == nil {
				ifName, _, _ = c.ifIndexes.LookupNameByNamespace(uint32(entry.LinkIndex), ifaceidx.DefNs)
			} else {
				ifName, _, _ = c.ifIndexes.LookupNameByNamespace(uint32(entry.LinkIndex), arp.Namespace.Name)
			}
			c.arpIndexes.RegisterName(ArpIdentifier(&entry), c.arpIdxSeq, &l3.LinuxStaticArpEntries_ArpEntry{
				// Register fields required to reconstruct ARP identifier
				Interface: ifName,
				IpAddr:    entry.IP.String(),
				HwAddress: entry.HardwareAddr.String(),
			})
			c.arpIdxSeq++
			c.log.Debugf("ARP entry registered as %s", ArpIdentifier(&entry))
		}
	}

	return nil
}

// ResolveCreatedInterface resolves a new linux interface from ARP perspective
func (c *LinuxArpConfigurator) ResolveCreatedInterface(ifName string, ifIdx uint32) error {
	// Look for ARP entries where the interface is used
	for arpName, arpIfPair := range c.arpIfCache {
		if arpIfPair.ifName == ifName && arpIfPair.isAdd {
			if err := c.ConfigureLinuxStaticArpEntry(arpIfPair.arp); err != nil {
				return errors.Errorf("failed to configure linux ARP %s with registered interface %s: %v",
					arpIfPair.arp.Name, ifName, err)
			}
			delete(c.arpIfCache, arpName)
		} else if arpIfPair.ifName == ifName && !arpIfPair.isAdd {
			c.log.Debugf("Cached ARP %v for interface %v removed", arpName, ifName)
			if err := c.DeleteLinuxStaticArpEntry(arpIfPair.arp); err != nil {
				return errors.Errorf("failed to remove linux ARP %s with registered interface %s: %v",
					arpIfPair.arp.Name, ifName, err)
			}
			delete(c.arpIfCache, arpName)
		}
		c.log.Debugf("Linux ARP %s removed from cache", arpName)
	}

	return nil
}

// ResolveDeletedInterface resolves removed linux interface from ARP perspective
func (c *LinuxArpConfigurator) ResolveDeletedInterface(ifName string, ifIdx uint32) error {
	// Read cache at first and remove obsolete entries
	for arpName, arpToIface := range c.arpIfCache {
		if arpToIface.ifName == ifName && !arpToIface.isAdd {
			delete(c.arpIfCache, arpName)
		}
	}

	// Read mapping of ARP entries and find all using removed interface
	for _, arpName := range c.arpIndexes.GetMapping().ListNames() {
		_, arp, found := c.arpIndexes.LookupIdx(arpName)
		if !found {
			// Should not happend but better to log it
			return errors.Errorf("failed to resolve unregistered interface for ARP: entry %s not found", arpName)
		}
		if arp == nil {
			return errors.Errorf("failed to resolve unregistered interface for ARP: no data available")
		}
		if arp.Interface == ifName {
			// Unregister
			ip := net.ParseIP(arp.IpAddr)
			if ip == nil {
				return errors.Errorf("failed to resolve unregistered interface for ARP %s: invalid IP address %s",
					arpName, arp.IpAddr)
			}
			mac, err := net.ParseMAC(arp.HwAddress)
			if err != nil {
				return errors.Errorf("failed to resolve unregistered interface for ARP %s: invalid MAC address %s",
					arpName, arp.HwAddress)
			}
			c.arpIndexes.UnregisterName(ArpIdentifier(&netlink.Neigh{
				LinkIndex:    int(ifIdx),
				IP:           ip,
				HardwareAddr: mac,
			}))
			// Cache
			c.arpIfCache[arpName] = &ArpToInterface{
				arp:    arp,
				ifName: ifName,
				isAdd:  true,
			}
			c.log.Debugf("Linux ARP entry %s unregistered and removed from cache", arpName)
		}
	}

	return nil
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *LinuxArpConfigurator) LogError(err error) error {
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

// ArpIdentifier generates unique ARP ID used in mapping
func ArpIdentifier(arp *netlink.Neigh) string {
	return fmt.Sprintf("iface%v-%v-%v", arp.LinkIndex, arp.IP.String(), arp.HardwareAddr)
}

// arpStateParser returns representation of neighbor unreachability detection index as defined in netlink
func arpStateParser(stateType *l3.LinuxStaticArpEntries_ArpEntry_NudState) int {
	// if state is not set, set it to permanent as default
	if stateType == nil {
		return netlink.NUD_PERMANENT
	}
	state := stateType.Type
	switch state {
	case 0:
		return netlink.NUD_PERMANENT
	case 1:
		return netlink.NUD_NOARP
	case 2:
		return netlink.NUD_REACHABLE
	case 3:
		return netlink.NUD_STALE
	default:
		return netlink.NUD_PERMANENT
	}
}

// returns IP family netlink representation
func getIPFamily(family *l3.LinuxStaticArpEntries_ArpEntry_IpFamily) (arpIPFamily int) {
	if family == nil {
		return
	}
	if family.Family == l3.LinuxStaticArpEntries_ArpEntry_IpFamily_IPV4 {
		arpIPFamily = netlink.FAMILY_V4
	}
	if family.Family == l3.LinuxStaticArpEntries_ArpEntry_IpFamily_IPV6 {
		arpIPFamily = netlink.FAMILY_V6
	}
	if family.Family == l3.LinuxStaticArpEntries_ArpEntry_IpFamily_ALL {
		arpIPFamily = netlink.FAMILY_ALL
	}
	if family.Family == l3.LinuxStaticArpEntries_ArpEntry_IpFamily_MPLS {
		arpIPFamily = netlink.FAMILY_MPLS
	}
	return
}

func compareARPLinkIdxAndIP(arp1 *netlink.Neigh, arp2 *netlink.Neigh) bool {
	if arp1.LinkIndex != arp2.LinkIndex {
		return false
	}
	if arp1.IP.String() != arp2.IP.String() {
		return false
	}
	return true
}
