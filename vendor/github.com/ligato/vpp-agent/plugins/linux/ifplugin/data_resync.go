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
	"strconv"

	"github.com/go-errors/errors"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
	"github.com/vishvananda/netlink"
)

// A List of known linux interface types which can be processed.
const (
	tap  = "tun"
	veth = "veth"
)

// LinuxDataPair stores linux interface with matching NB configuration
type LinuxDataPair struct {
	linuxIfData netlink.Link
	nbIfData    *interfaces.LinuxInterfaces_Interface
}

// Resync writes interfaces to Linux. Interface host name corresponds with Linux host interface name (but name can
// be different). Resync consists of following steps:
// 1. Iterate over all NB interfaces. Try to find interface with the same name in required namespace for every NB interface.
// 2. If interface does not exist, will be created anew
// 3. If interface exists, it is correlated and modified if needed.
// Resync configures an initial set of interfaces. Existing Linux interfaces are registered and potentially re-configured.
func (c *LinuxInterfaceConfigurator) Resync(nbIfs []*interfaces.LinuxInterfaces_Interface) error {
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()

	// Cache for interfaces modified later (interface name/link data)
	linkMap := make(map[string]*LinuxDataPair)

	var errs []error
	// Iterate over NB configuration. Look for interfaces with the same host name
	for _, nbIf := range nbIfs {
		c.handleOptionalHostIfName(nbIf)
		c.addInterfaceToCache(nbIf)

		// Find linux equivalent for every NB interface and register it
		linkIf, err := c.findLinuxInterface(nbIf, nsMgmtCtx)
		if err != nil {
			errs = append(errs, err)
		}
		if linkIf != nil {
			// If interface was found, it will be compared and modified in the next step
			c.log.Debugf("linux interface %s resync: interface found in namespace", nbIf.Name)
			linkMap[nbIf.Name] = &LinuxDataPair{
				linuxIfData: linkIf,
				nbIfData:    nbIf,
			}
		} else {
			// If not, configure it
			c.log.Debugf("linux interface %s resync: interface not found and will be configured", nbIf.Name)
			if err := c.ConfigureLinuxInterface(nbIf); err != nil {
				errs = append(errs, errors.Errorf("linux interface %s resync error: %v", nbIf.Name, err))
			}
		}
	}

	// Process all interfaces waiting for modification. All NB interfaces are already registered at this point.
	for linkName, linkDataPair := range linkMap {
		linuxIf, err := c.reconstructIfConfig(linkDataPair.linuxIfData, linkDataPair.nbIfData.Namespace, linkName)
		if err != nil {
			errs = append(errs, errors.Errorf("linux interface %s resync: failed to reconstruct interface configuration: %v",
				linkName, err))
		}

		// For VETH, resolve peer
		if linkDataPair.nbIfData.Type == interfaces.LinuxInterfaces_VETH {
			// Search registered config for peer
			var found bool
			c.mapMu.RLock()
			for _, cachedIfCfg := range c.ifByName {
				if cachedIfCfg.config != nil && cachedIfCfg.config.Type == interfaces.LinuxInterfaces_VETH {
					if cachedIfCfg.config.Veth != nil && cachedIfCfg.config.Veth.PeerIfName == linuxIf.HostIfName {
						found = true
						linuxIf.Veth = &interfaces.LinuxInterfaces_Interface_Veth{
							PeerIfName: cachedIfCfg.config.HostIfName,
						}
					}
				}
			}
			c.mapMu.RUnlock()
			if found {
				c.log.Debugf("linux interface %s resync: found peer %s", linkName, linuxIf.Veth.PeerIfName)
			} else {
				// No info about the peer, use the same as in the NB config
				linuxIf.Veth = &interfaces.LinuxInterfaces_Interface_Veth{
					PeerIfName: linkDataPair.nbIfData.Veth.PeerIfName,
				}
			}
		}
		// Check if interface needs to be modified
		if c.isLinuxIfModified(linkDataPair.nbIfData, linuxIf) {
			c.log.Debugf("linux interface %s resync: configuration changed, interface will be modified", linkName)
			if err := c.ModifyLinuxInterface(linkDataPair.nbIfData, linuxIf); err != nil {
				errs = append(errs, errors.Errorf("linux interface %s resync error: %v", linkName, err))
			}
		} else {
			c.log.Debugf("linux interface %s resync: data unchanged", linkName)
		}
	}

	// Register all interfaces in default namespace which were not already registered
	linkList, err := netlink.LinkList()
	if err != nil {
		errs = append(errs, errors.Errorf("linux interface resync error: failed to read interfaces: %v", err))
	}
	for _, link := range linkList {
		if link.Attrs() == nil {
			continue
		}
		attrs := link.Attrs()
		_, _, found := c.ifIndexes.LookupIdx(attrs.Name)
		if !found {
			// Register interface with name (other parameters can be read if needed)
			c.ifIndexes.RegisterName(attrs.Name, c.ifIdxSeq, &ifaceidx.IndexedLinuxInterface{
				Index: uint32(attrs.Index),
				Data: &interfaces.LinuxInterfaces_Interface{
					Name:       attrs.Name,
					HostIfName: attrs.Name,
				},
			})
			c.ifIdxSeq++
		}
	}

	if len(errs) > 0 {
		for _, e := range errs {
			c.log.Error(e)
		}
		return errors.Errorf("%v resync errors encountered", len(errs))
	}

	c.log.Info("Linux interface resync done")

	return nil
}

// Reconstruct common interface configuration from netlink.Link data
func (c *LinuxInterfaceConfigurator) reconstructIfConfig(linuxIf netlink.Link, ns *interfaces.LinuxInterfaces_Interface_Namespace, name string) (*interfaces.LinuxInterfaces_Interface, error) {
	linuxIfAttr := linuxIf.Attrs()

	// Prepare list of addresses
	addresses, err := c.getLinuxAddresses(linuxIf, ns)
	if err != nil {
		return nil, err
	}

	return &interfaces.LinuxInterfaces_Interface{
		Name: name,
		Type: func(ifType string) interfaces.LinuxInterfaces_InterfaceType {
			if ifType == veth {
				return interfaces.LinuxInterfaces_VETH
			}
			return interfaces.LinuxInterfaces_AUTO_TAP
		}(linuxIf.Type()),
		Enabled: func(state netlink.LinkOperState) bool {
			if state == netlink.OperDown {
				return false
			}
			return true
		}(linuxIfAttr.OperState),
		IpAddresses: addresses,
		PhysAddress: func(hwAddr net.HardwareAddr) (mac string) {
			if hwAddr != nil {
				mac = hwAddr.String()
			}
			return
		}(linuxIfAttr.HardwareAddr),
		Mtu:        uint32(linuxIfAttr.MTU),
		HostIfName: linuxIfAttr.Name,
	}, nil
}

// Reads linux interface IP addresses
func (c *LinuxInterfaceConfigurator) getLinuxAddresses(linuxIf netlink.Link, ns *interfaces.LinuxInterfaces_Interface_Namespace) (addresses []string, err error) {
	// Move to proper namespace
	if ns != nil {
		if !c.nsHandler.IsNamespaceAvailable(ns) {
			return nil, errors.Errorf("linux interface %s resync error: namespace is not available",
				linuxIf.Attrs().Name)
		}
		// Switch to namespace
		revertNs, err := c.nsHandler.SwitchToNamespace(nsplugin.NewNamespaceMgmtCtx(), ns)
		if err != nil {
			return nil, errors.Errorf("linux interface %s resync error: failed to switch to namespace",
				linuxIf.Attrs().Name)
		}
		defer revertNs()
	}

	addressList, err := netlink.AddrList(linuxIf, netlink.FAMILY_ALL)
	if err != nil {
		return nil, errors.Errorf("linux interface %s resync error: failed to read IP addresses",
			linuxIf.Attrs().Name)
	}

	for _, address := range addressList {
		mask, _ := address.Mask.Size()
		addrStr := address.IP.String() + "/" + strconv.Itoa(mask)
		addresses = append(addresses, addrStr)
	}

	return addresses, nil
}

// Compare interface fields in order to find differences.
func (c *LinuxInterfaceConfigurator) isLinuxIfModified(nbIf, linuxIf *interfaces.LinuxInterfaces_Interface) bool {
	c.log.Debugf("Linux interface RESYNC comparison started for interface %s", nbIf.Name)
	// Type
	if nbIf.Type != linuxIf.Type {
		c.log.Debugf("Linux interface RESYNC comparison: type changed (NB: %v, Linux: %v)",
			nbIf.Type, linuxIf.Type)
		return true
	}
	// Enabled
	if nbIf.Enabled != linuxIf.Enabled {
		c.log.Debugf("Linux interface RESYNC comparison: enabled value changed (NB: %t, Linux: %t)",
			nbIf.Enabled, linuxIf.Enabled)
		return true
	}
	// Remove link local addresses
	var newIPIdx int
	for ipIdx, ipAddress := range linuxIf.IpAddresses {
		if !net.ParseIP(linuxIf.IpAddresses[ipIdx]).IsLinkLocalUnicast() {
			linuxIf.IpAddresses[newIPIdx] = ipAddress
			newIPIdx++
		}
	}
	// Prune IP address list
	linuxIf.IpAddresses = linuxIf.IpAddresses[:newIPIdx]
	// IP address count
	if len(nbIf.IpAddresses) != len(linuxIf.IpAddresses) {
		c.log.Debugf("Linux interface RESYNC comparison: ip address count does not match (NB: %d, Linux: %d)",
			len(nbIf.IpAddresses), len(linuxIf.IpAddresses))
		return true
	}
	// IP address comparison
	for _, nbIP := range nbIf.IpAddresses {
		var ipFound bool
		for _, linuxIP := range linuxIf.IpAddresses {
			pNbIP, nbIPNet, err := net.ParseCIDR(nbIP)
			if err != nil {
				c.log.Error(err)
				continue
			}
			pVppIP, vppIPNet, err := net.ParseCIDR(linuxIP)
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
			c.log.Debugf("Interface RESYNC comparison: linux interface %v does not contain IP %s", nbIf.Name, nbIP)
			return true
		}
	}
	// Physical address
	if nbIf.PhysAddress != "" && nbIf.PhysAddress != linuxIf.PhysAddress {
		c.log.Debugf("Interface RESYNC comparison: MAC address changed (NB: %s, Linux: %s)",
			nbIf.PhysAddress, linuxIf.PhysAddress)
		return true
	}
	// MTU (if NB value is set)
	if nbIf.Mtu != 0 && nbIf.Mtu != linuxIf.Mtu {
		c.log.Debugf("Interface RESYNC comparison: MTU changed (NB: %d, Linux: %d)",
			nbIf.Mtu, linuxIf.Mtu)
		return true
	}
	switch nbIf.Type {
	case interfaces.LinuxInterfaces_VETH:
		if nbIf.Veth == nil && linuxIf.Veth != nil || nbIf.Veth != nil && linuxIf.Veth == nil {
			c.log.Debugf("Interface RESYNC comparison: VETH setup changed (NB: %v, VPP: %v)",
				nbIf.Veth, linuxIf.Veth)
			return true
		}
		if nbIf.Veth != nil && linuxIf.Veth != nil {
			// VETH peer name
			if nbIf.Veth.PeerIfName != linuxIf.Veth.PeerIfName {
				c.log.Debugf("Interface RESYNC comparison: VETH peer name changed (NB: %s, VPP: %s)",
					nbIf.Veth.PeerIfName, linuxIf.Veth.PeerIfName)
				return true
			}
		}
	case interfaces.LinuxInterfaces_AUTO_TAP:
		// Host name for TAP
		if nbIf.HostIfName != linuxIf.HostIfName {
			c.log.Debugf("Interface RESYNC comparison: TAP host name changed (NB: %d, Linux: %d)",
				nbIf.HostIfName, linuxIf.HostIfName)
			return true
		}
		// Note: do not compare TAP temporary name. It is local-only parameter which cannot be leveraged externally.
	}

	return false
}

// Looks for linux interface. Returns net.Link object if found
func (c *LinuxInterfaceConfigurator) findLinuxInterface(nbIf *interfaces.LinuxInterfaces_Interface, nsMgmtCtx *nsplugin.NamespaceMgmtCtx) (netlink.Link, error) {
	// Move to proper namespace
	if nbIf.Namespace != nil {
		if !c.nsHandler.IsNamespaceAvailable(nbIf.Namespace) {
			// Not and error
			c.log.Debugf("Interface %s is not ready to be configured, namespace %s is not available",
				nbIf.Name, nbIf.Namespace.Name)
			return nil, nil
		}
		// Switch to namespace
		revertNs, err := c.nsHandler.SwitchToNamespace(nsMgmtCtx, nbIf.Namespace)
		if err != nil {
			return nil, errors.Errorf("linux interface %s resync: failed to switch to namespace %s: %v",
				nbIf.HostIfName, nbIf.Namespace.Name, err)
		}
		defer revertNs()
	}
	// Look for interface
	linkIf, err := netlink.LinkByName(nbIf.HostIfName)
	if err != nil {
		// Link not found is not an error in this case
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			// Interface was not found
			return nil, nil
		}
		return nil, errors.Errorf("linux interface %s resync: %v", nbIf.HostIfName, err)
	}
	if linkIf == nil || linkIf.Attrs() == nil {
		return nil, errors.Errorf("linux interface %s resync: link is nil", nbIf.HostIfName)
	}

	// Add interface to cache
	c.registerLinuxInterface(uint32(linkIf.Attrs().Index), nbIf)

	return linkIf, nil
}

// Register linux interface
func (c *LinuxInterfaceConfigurator) registerLinuxInterface(linuxIfIdx uint32, nbIf *interfaces.LinuxInterfaces_Interface) {
	// Register interface with its name
	c.ifIndexes.RegisterName(nbIf.Name, c.ifIdxSeq, &ifaceidx.IndexedLinuxInterface{
		Index: linuxIfIdx,
		Data:  nbIf,
	})
	c.ifIdxSeq++
	c.log.Debugf("linux interface %s registered", nbIf.Name)
}

// Add interface to cache
func (c *LinuxInterfaceConfigurator) addInterfaceToCache(nbIf *interfaces.LinuxInterfaces_Interface) *LinuxInterfaceConfig {
	switch nbIf.Type {
	case interfaces.LinuxInterfaces_AUTO_TAP:
		return c.addToCache(nbIf, nil)
	case interfaces.LinuxInterfaces_VETH:
		peerConfig := c.getInterfaceConfig(nbIf.Veth.PeerIfName)
		return c.addToCache(nbIf, peerConfig)
	}
	return nil
}
