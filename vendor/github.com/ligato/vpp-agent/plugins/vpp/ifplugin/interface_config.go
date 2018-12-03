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

// Package ifplugin implements the Interface plugin that handles management
// of VPP interfaces.
package ifplugin

import (
	"bytes"
	"net"
	"strings"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/dhcp"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
)

// InterfaceConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of interfaces as modelled by the proto file "../model/interfaces/interfaces.proto"
// and stored in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1interface".
// Updates received from the northbound API are compared with the VPP run-time configuration and differences
// are applied through the VPP binary API.
type InterfaceConfigurator struct {
	log logging.Logger

	linux interface{} // just flag if nil

	swIfIndexes ifaceidx.SwIfIndexRW
	dhcpIndexes ifaceidx.DhcpIndexRW

	uIfaceCache         map[string]string                     // cache for not-configurable unnumbered interfaces. map[unumbered-iface-name]required-iface
	memifScCache        map[string]uint32                     // memif socket filename/ID cache (all known sockets). Note: do not remove items from the map
	vxlanMulticastCache map[string]*intf.Interfaces_Interface // cache for multicast VxLANs expecting another interface

	defaultMtu uint32 // default MTU value can be read from config

	afPacketConfigurator *AFPacketConfigurator

	vppCh govppapi.Channel

	// VPP API handler
	ifHandler vppcalls.IfVppAPI

	// Notification channels
	NotifChan chan govppapi.Message // to publish SwInterfaceDetails to interface_state.go
	DhcpChan  chan govppapi.Message // channel to receive DHCP notifications
}

// Init members (channels...) and start go routines
func (c *InterfaceConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, linux interface{},
	notifChan chan govppapi.Message, defaultMtu uint32) (err error) {
	// Logger
	c.log = logger.NewLogger("if-conf")

	// State notification channel
	c.NotifChan = notifChan

	// Config file data
	c.defaultMtu = defaultMtu

	// VPP channel
	if c.vppCh, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handler
	c.ifHandler = vppcalls.NewIfVppHandler(c.vppCh, c.log)

	// Mappings
	c.swIfIndexes = ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(c.log, "sw_if_indexes", ifaceidx.IndexMetadata))
	c.dhcpIndexes = ifaceidx.NewDHCPIndex(nametoidx.NewNameToIdx(c.log, "dhcp_indices", ifaceidx.IndexDHCPMetadata))
	c.uIfaceCache = make(map[string]string)
	c.vxlanMulticastCache = make(map[string]*intf.Interfaces_Interface)
	c.memifScCache = make(map[string]uint32)

	// Init AF-packet configurator
	c.linux = linux
	c.afPacketConfigurator = &AFPacketConfigurator{}
	c.afPacketConfigurator.Init(c.log, c.ifHandler, c.linux, c.swIfIndexes)

	// DHCP channel
	c.DhcpChan = make(chan govppapi.Message, 1)
	if _, err := c.vppCh.SubscribeNotification(c.DhcpChan, &dhcp.DHCPComplEvent{}); err != nil {
		return err
	}

	go c.watchDHCPNotifications()

	c.log.Info("Interface configurator initialized")

	return nil
}

// Close GOVPP channel
func (c *InterfaceConfigurator) Close() error {
	if err := safeclose.Close(c.vppCh, c.DhcpChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose interface configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *InterfaceConfigurator) clearMapping() error {
	c.swIfIndexes.Clear()
	c.dhcpIndexes.Clear()
	c.uIfaceCache = make(map[string]string)
	c.vxlanMulticastCache = make(map[string]*intf.Interfaces_Interface)
	c.memifScCache = make(map[string]uint32)

	c.log.Debugf("interface configurator mapping cleared")
	return nil
}

// GetSwIfIndexes exposes interface name-to-index mapping
func (c *InterfaceConfigurator) GetSwIfIndexes() ifaceidx.SwIfIndexRW {
	return c.swIfIndexes
}

// GetDHCPIndexes exposes DHCP name-to-index mapping
func (c *InterfaceConfigurator) GetDHCPIndexes() ifaceidx.DhcpIndexRW {
	return c.dhcpIndexes
}

// IsSocketFilenameCached returns true if provided filename is presented in the cache
func (c *InterfaceConfigurator) IsSocketFilenameCached(filename string) bool {
	_, ok := c.memifScCache[filename]
	return ok
}

// IsUnnumberedIfCached returns true if provided interface is cached as unconfigurabel unnubered interface
func (c *InterfaceConfigurator) IsUnnumberedIfCached(ifName string) bool {
	_, ok := c.uIfaceCache[ifName]
	return ok
}

// IsMulticastVxLanIfCached returns true if provided interface is cached as VxLAN with missing multicast interface
func (c *InterfaceConfigurator) IsMulticastVxLanIfCached(ifName string) bool {
	_, ok := c.vxlanMulticastCache[ifName]
	return ok
}

// ConfigureVPPInterface reacts to a new northbound VPP interface config by creating and configuring
// the interface in the VPP network stack through the VPP binary API.
func (c *InterfaceConfigurator) ConfigureVPPInterface(iface *intf.Interfaces_Interface) (err error) {
	var ifIdx uint32

	switch iface.Type {
	case intf.InterfaceType_TAP_INTERFACE:
		ifIdx, err = c.ifHandler.AddTapInterface(iface.Name, iface.Tap)
	case intf.InterfaceType_MEMORY_INTERFACE:
		var id uint32 // Memif socket id
		if id, err = c.resolveMemifSocketFilename(iface.Memif); err != nil {
			return err
		}
		ifIdx, err = c.ifHandler.AddMemifInterface(iface.Name, iface.Memif, id)
	case intf.InterfaceType_VXLAN_TUNNEL:
		// VxLAN multicast interface. Interrupt the processing if there is an error or interface was cached
		multicastIfIdx, cached, err := c.getVxLanMulticast(iface)
		if err != nil || cached {
			return err
		}
		ifIdx, err = c.ifHandler.AddVxLanTunnel(iface.Name, iface.Vrf, multicastIfIdx, iface.Vxlan)
	case intf.InterfaceType_SOFTWARE_LOOPBACK:
		ifIdx, err = c.ifHandler.AddLoopbackInterface(iface.Name)
	case intf.InterfaceType_ETHERNET_CSMACD:
		var exists bool
		if ifIdx, _, exists = c.swIfIndexes.LookupIdx(iface.Name); !exists {
			c.log.Warnf("It is not yet supported to add (whitelist) a new physical interface")
			return nil
		}
	case intf.InterfaceType_AF_PACKET_INTERFACE:
		var pending bool
		if ifIdx, pending, err = c.afPacketConfigurator.ConfigureAfPacketInterface(iface); err != nil {
			return err
		}
		if pending {
			c.log.Debugf("Af-packet interface %s cannot be created yet and will be configured later", iface)
			return nil
		}
	}
	if err != nil {
		return err
	}

	// Rx-mode
	if err := c.configRxModeForInterface(iface, ifIdx); err != nil {
		return err
	}

	// Rx-placement TODO: simplify implementation for rx placement when the binary api call will be available (remove dump)
	if iface.RxPlacementSettings != nil {
		// Required in order to get vpp internal name. Must be called from here, calling in vppcalls causes
		// import cycle
		ifMap, err := c.ifHandler.DumpInterfaces()
		if err != nil {
			return errors.Errorf("failed to dump interfaces: %v", err)
		}
		ifData, ok := ifMap[ifIdx]
		if !ok || ifData == nil {
			return errors.Errorf("set rx-placement failed, no data available for interface index %d", ifIdx)
		}
		if err := c.ifHandler.SetRxPlacement(ifData.Meta.SwIfIndex, iface.RxPlacementSettings); err != nil {
			return errors.Errorf("failed to set rx-placement for interface %s: %v", ifData.Interface.Name, err)
		}
	}

	// MAC address (optional, for af-packet is configured in different way)
	if iface.PhysAddress != "" && iface.Type != intf.InterfaceType_AF_PACKET_INTERFACE {
		if err := c.ifHandler.SetInterfaceMac(ifIdx, iface.PhysAddress); err != nil {
			return errors.Errorf("failed to set MAC address %s to interface %s: %v",
				iface.PhysAddress, iface.Name, err)
		}
	}

	// DHCP client
	if iface.SetDhcpClient {
		if err := c.ifHandler.SetInterfaceAsDHCPClient(ifIdx, iface.Name); err != nil {
			return errors.Errorf("failed to set interface %s as DHCP client", iface.Name)
		}
	}

	// Get IP addresses
	IPAddrs, err := addrs.StrAddrsToStruct(iface.IpAddresses)
	if err != nil {
		return errors.Errorf("failed to convert %s IP address list to IPNet structures: %v", iface.Name, err)
	}

	// VRF (optional, unavailable for VxLAN interfaces), has to be done before IP addresses are configured
	if iface.Type != intf.InterfaceType_VXLAN_TUNNEL {
		// Configured separately for IPv4/IPv6
		isIPv4, isIPv6 := getIPAddressVersions(IPAddrs)
		if isIPv4 {
			if err := c.ifHandler.SetInterfaceVrf(ifIdx, iface.Vrf); err != nil {
				return errors.Errorf("failed to set interface %s as IPv4 VRF %d: %v", iface.Name, iface.Vrf, err)
			}
		}
		if isIPv6 {
			if err := c.ifHandler.SetInterfaceVrfIPv6(ifIdx, iface.Vrf); err != nil {
				return errors.Errorf("failed to set interface %s as IPv6 VRF %d: %v", iface.Name, iface.Vrf, err)
			}
		}
	}

	// Configure IP addresses or unnumbered config
	if err := c.configureIPAddresses(iface.Name, ifIdx, IPAddrs, iface.Unnumbered); err != nil {
		return err
	}

	// configure container IP address
	if iface.ContainerIpAddress != "" {
		if err := c.ifHandler.AddContainerIP(ifIdx, iface.ContainerIpAddress); err != nil {
			return errors.Errorf("failed to add container IP address %s to interface %s: %v",
				iface.ContainerIpAddress, iface.Name, err)
		}
	}

	// configure mtu. Prefer value in interface config, otherwise set default value if defined
	if iface.Type != intf.InterfaceType_VXLAN_TUNNEL {
		mtuToConfigure := iface.Mtu
		if mtuToConfigure == 0 && c.defaultMtu != 0 {
			mtuToConfigure = c.defaultMtu
		}
		if mtuToConfigure != 0 {
			iface.Mtu = mtuToConfigure
			if err := c.ifHandler.SetInterfaceMtu(ifIdx, mtuToConfigure); err != nil {
				return errors.Errorf("failed to set MTU %d to interface %s: %v", mtuToConfigure, iface.Name, err)
			}
		}
	}

	// register name to idx mapping if it is not an af_packet interface type (it is registered in ConfigureAfPacketInterface if needed)
	if iface.Type != intf.InterfaceType_AF_PACKET_INTERFACE {
		c.swIfIndexes.RegisterName(iface.Name, ifIdx, iface)
		c.log.Debugf("Interface %s registered to interface mapping", iface.Name)
	}

	// set interface up if enabled
	// NOTE: needs to be called after RegisterName, otherwise interface up/down notification won't map to a valid interface
	if iface.Enabled {
		if err := c.ifHandler.InterfaceAdminUp(ifIdx); err != nil {
			return errors.Errorf("failed to set interface %s up: %v", iface.Name, err)
		}
	}

	// load interface state data for newly added interface (no way to filter by swIfIndex, need to dump all of them)
	if err := c.propagateIfDetailsToStatus(); err != nil {
		return err
	}

	// Check whether there is no VxLAN interface waiting on created one as a multicast
	if err := c.resolveCachedVxLANMulticasts(iface.Name); err != nil {
		return err
	}

	c.log.Infof("Interface %s configured", iface.Name)

	return nil
}

/**
Set rx-mode on specified VPP interface

Legend:
P - polling
I - interrupt
A - adaptive

Interfaces - supported modes:
* tap interface - PIA
* memory interface - PIA
* vxlan tunnel - PIA
* software loopback - PIA
* ethernet csmad - P
* af packet - PIA
*/
func (c *InterfaceConfigurator) configRxModeForInterface(iface *intf.Interfaces_Interface, ifIdx uint32) error {
	rxModeSettings := iface.RxModeSettings
	if rxModeSettings != nil {
		switch iface.Type {
		case intf.InterfaceType_ETHERNET_CSMACD:
			if rxModeSettings.RxMode == intf.RxModeType_POLLING {
				return c.configRxMode(iface, ifIdx, rxModeSettings)
			}
		default:
			return c.configRxMode(iface, ifIdx, rxModeSettings)
		}
	}
	return nil
}

// Call specific vpp API method for setting rx-mode
func (c *InterfaceConfigurator) configRxMode(iface *intf.Interfaces_Interface, ifIdx uint32, rxModeSettings *intf.Interfaces_Interface_RxModeSettings) error {
	if err := c.ifHandler.SetRxMode(ifIdx, rxModeSettings); err != nil {
		return errors.Errorf("failed to set Rx-mode for interface %s: %v", iface.Name, err)
	}
	return nil
}

func (c *InterfaceConfigurator) configureIPAddresses(ifName string, ifIdx uint32, addresses []*net.IPNet, unnumbered *intf.Interfaces_Interface_Unnumbered) error {
	if unnumbered != nil && unnumbered.IsUnnumbered {
		ifWithIP := unnumbered.InterfaceWithIp
		if ifWithIP == "" {
			return errors.Errorf("unnubered interface %s has no interface with IP address set", ifName)
		}
		ifIdxIP, _, found := c.swIfIndexes.LookupIdx(ifWithIP)
		if !found {
			// cache not-configurable interface
			c.uIfaceCache[ifName] = ifWithIP
			c.log.Debugf("unnubered interface %s moved to cache (requires IP address from non-existing %s)", ifName, ifWithIP)
			return nil
		}
		// Set interface as un-numbered
		if err := c.ifHandler.SetUnnumberedIP(ifIdx, ifIdxIP); err != nil {
			return errors.Errorf("failed to set interface %v as unnumbered for %v: %v", ifName, ifIdxIP, err)
		}
	}

	// configure optional ip address
	for _, address := range addresses {
		if err := c.ifHandler.AddInterfaceIP(ifIdx, address); err != nil {
			return errors.Errorf("adding IP address %v to interface %v failed: %v", address, ifName, err)
		}
	}

	// with ip address configured, the interface can be used as a source for un-numbered interfaces (if any)
	if err := c.resolveDependentUnnumberedInterfaces(ifName, ifIdx); err != nil {
		return err
	}
	return nil
}

func (c *InterfaceConfigurator) removeIPAddresses(ifIdx uint32, addresses []*net.IPNet, unnumbered *intf.Interfaces_Interface_Unnumbered) error {
	if unnumbered != nil && unnumbered.IsUnnumbered {
		// Set interface as un-numbered
		if err := c.ifHandler.UnsetUnnumberedIP(ifIdx); err != nil {
			return errors.Errorf("faield to unset unnumbered IP for interface %d: %v", ifIdx, err)
		}
	}

	// delete IP Addresses
	for _, addr := range addresses {
		err := c.ifHandler.DelInterfaceIP(ifIdx, addr)
		if err != nil {
			return errors.Errorf("deleting IP address %s from interface %d failed: %v", addr, ifIdx, err)
		}
	}

	return nil
}

// Iterate over all un-numbered interfaces in cache (which could not be configured before) and find all interfaces
// dependent on the provided one
func (c *InterfaceConfigurator) resolveDependentUnnumberedInterfaces(ifNameIP string, ifIdxIP uint32) error {
	for uIface, ifWithIP := range c.uIfaceCache {
		if ifWithIP == ifNameIP {
			// find index of the dependent interface
			uIdx, _, found := c.swIfIndexes.LookupIdx(uIface)
			if !found {
				delete(c.uIfaceCache, uIface)
				c.log.Debugf("Unnumbered interface %s removed from cache (not found)", uIface)
				continue
			}
			if err := c.ifHandler.SetUnnumberedIP(uIdx, ifIdxIP); err != nil {
				return errors.Errorf("setting unnumbered IP %v for interface %v (%v) failed: %v", ifIdxIP, uIface, uIdx, err)
			}
			delete(c.uIfaceCache, uIface)
			c.log.Debugf("Unnumbered interface %s set and removed from cache", uIface)
		}
	}
	return nil
}

// ModifyVPPInterface applies changes in the NB configuration of a VPP interface into the running VPP
// through the VPP binary API.
func (c *InterfaceConfigurator) ModifyVPPInterface(newConfig *intf.Interfaces_Interface,
	oldConfig *intf.Interfaces_Interface) error {

	// Recreate pending Af-packet
	if newConfig.Type == intf.InterfaceType_AF_PACKET_INTERFACE && c.afPacketConfigurator.IsPendingAfPacket(oldConfig) {
		return c.recreateVPPInterface(newConfig, oldConfig, 0)
	}

	// Re-create cached VxLAN
	if newConfig.Type == intf.InterfaceType_VXLAN_TUNNEL {
		if _, ok := c.vxlanMulticastCache[newConfig.Name]; ok {
			delete(c.vxlanMulticastCache, newConfig.Name)
			c.log.Debugf("Interface %s removed from VxLAN multicast cache, will be configured", newConfig.Name)
			return c.ConfigureVPPInterface(newConfig)
		}
	}

	// Lookup index. If not found, create interface a a new on.
	ifIdx, meta, found := c.swIfIndexes.LookupIdx(newConfig.Name)
	if !found {
		c.log.Warnf("Modify interface %s: index was not found in the mapping, creating as a new one", newConfig.Name)
		return c.ConfigureVPPInterface(newConfig)
	}

	if err := c.modifyVPPInterface(newConfig, oldConfig, ifIdx, meta.Type); err != nil {
		return err
	}

	c.log.Infof("Interface %s modified", newConfig.Name)

	return nil
}

// ModifyVPPInterface applies changes in the NB configuration of a VPP interface into the running VPP
// through the VPP binary API.
func (c *InterfaceConfigurator) modifyVPPInterface(newConfig, oldConfig *intf.Interfaces_Interface,
	ifIdx uint32, ifaceType intf.InterfaceType) (err error) {

	switch ifaceType {
	case intf.InterfaceType_TAP_INTERFACE:
		if !c.canTapBeModifWithoutDelete(newConfig.Tap, oldConfig.Tap) {
			return c.recreateVPPInterface(newConfig, oldConfig, ifIdx)
		}
	case intf.InterfaceType_MEMORY_INTERFACE:
		if !c.canMemifBeModifWithoutDelete(newConfig.Memif, oldConfig.Memif) {
			return c.recreateVPPInterface(newConfig, oldConfig, ifIdx)
		}
	case intf.InterfaceType_VXLAN_TUNNEL:
		if !c.canVxlanBeModifWithoutDelete(newConfig.Vxlan, oldConfig.Vxlan) ||
			oldConfig.Vrf != newConfig.Vrf {
			return c.recreateVPPInterface(newConfig, oldConfig, ifIdx)
		}
	case intf.InterfaceType_AF_PACKET_INTERFACE:
		recreate, err := c.afPacketConfigurator.ModifyAfPacketInterface(newConfig, oldConfig)
		if err != nil {
			return err
		}
		if recreate {
			return c.recreateVPPInterface(newConfig, oldConfig, ifIdx)
		}
	case intf.InterfaceType_SOFTWARE_LOOPBACK:
	case intf.InterfaceType_ETHERNET_CSMACD:
	}

	// Rx-mode
	if !(oldConfig.RxModeSettings == nil && newConfig.RxModeSettings == nil) {
		if err := c.modifyRxModeForInterfaces(oldConfig, newConfig, ifIdx); err != nil {
			return err
		}
	}

	// Rx-placement
	if newConfig.RxPlacementSettings != nil {
		// Required in order to get vpp internal name. Must be called from here, calling in vppcalls causes
		// import cycle
		ifMap, err := c.ifHandler.DumpInterfaces()
		if err != nil {
			return errors.Errorf("failed to dump interfaces: %v", err)
		}
		ifData, ok := ifMap[ifIdx]
		if !ok || ifData == nil {
			return errors.Errorf("set rx-placement for new config failed, no data available for interface index %d", ifIdx)
		}
		if err := c.ifHandler.SetRxPlacement(ifData.Meta.SwIfIndex, newConfig.RxPlacementSettings); err != nil {
			return errors.Errorf("failed to set rx-placement for interface %s: %v", newConfig.Name, err)
		}
	}

	// Admin status
	if newConfig.Enabled != oldConfig.Enabled {
		if newConfig.Enabled {
			if err = c.ifHandler.InterfaceAdminUp(ifIdx); err != nil {
				return errors.Errorf("failed to set interface %s up: %v", newConfig.Name, err)
			}
		} else {
			if err = c.ifHandler.InterfaceAdminDown(ifIdx); err != nil {
				return errors.Errorf("failed to set interface %s down: %v", newConfig.Name, err)
			}
		}
	}

	// Configure new mac address if set (and only if it was changed)
	if newConfig.PhysAddress != "" && newConfig.PhysAddress != oldConfig.PhysAddress {
		if err := c.ifHandler.SetInterfaceMac(ifIdx, newConfig.PhysAddress); err != nil {
			return errors.Errorf("setting interface %s MAC address %s failed: %v",
				newConfig.Name, newConfig.PhysAddress, err)
		}
	}

	// Reconfigure DHCP
	if oldConfig.SetDhcpClient != newConfig.SetDhcpClient {
		if newConfig.SetDhcpClient {
			if err := c.ifHandler.SetInterfaceAsDHCPClient(ifIdx, newConfig.Name); err != nil {
				return errors.Errorf("failed to set interface %s as DHCP client: %v", newConfig.Name, err)
			}
		} else {
			if err := c.ifHandler.UnsetInterfaceAsDHCPClient(ifIdx, newConfig.Name); err != nil {
				return errors.Errorf("failed to unset interface %s as DHCP client: %v", newConfig.Name, err)
			}
			// Remove from DHCP mapping
			c.dhcpIndexes.UnregisterName(newConfig.Name)
			c.log.Debugf("Interface %s unregistered as DHCP client", oldConfig.Name)
		}
	}

	// Ip addresses
	newAddrs, err := addrs.StrAddrsToStruct(newConfig.IpAddresses)
	if err != nil {
		return errors.Errorf("failed to convert %s IP address list to IPNet structures: %v", newConfig.Name, err)
	}
	oldAddrs, err := addrs.StrAddrsToStruct(oldConfig.IpAddresses)
	if err != nil {
		return errors.Errorf("failed to convert %s IP address list to IPNet structures: %v", oldConfig.Name, err)
	}

	// Reconfigure VRF
	if ifaceType != intf.InterfaceType_VXLAN_TUNNEL {
		// Interface must not have IP when setting VRF
		if err := c.removeIPAddresses(ifIdx, oldAddrs, oldConfig.Unnumbered); err != nil {
			return err
		}

		// Get VRF IP version using new list of addresses. During modify, interface VRF IP version
		// should be updated as well.
		isIPv4, isIPv6 := getIPAddressVersions(newAddrs)
		if isIPv4 {
			if err := c.ifHandler.SetInterfaceVrf(ifIdx, newConfig.Vrf); err != nil {
				return errors.Errorf("failed to set IPv4 VRF %d for interface %s: %v",
					newConfig.Vrf, newConfig.Name, err)
			}
		}
		if isIPv6 {
			if err := c.ifHandler.SetInterfaceVrfIPv6(ifIdx, newConfig.Vrf); err != nil {
				return errors.Errorf("failed to set IPv6 VRF %d for interface %s: %v",
					newConfig.Vrf, newConfig.Name, err)
			}
		}

		if err = c.configureIPAddresses(newConfig.Name, ifIdx, newAddrs, newConfig.Unnumbered); err != nil {
			return err
		}
	}

	// Container ip address
	if newConfig.ContainerIpAddress != oldConfig.ContainerIpAddress {
		if err := c.ifHandler.AddContainerIP(ifIdx, newConfig.ContainerIpAddress); err != nil {
			return errors.Errorf("failed to add container IP %s to interface %s: %v",
				newConfig.ContainerIpAddress, newConfig.Name, err)
		}
	}

	// Set MTU if changed in interface config
	if newConfig.Mtu != 0 && newConfig.Mtu != oldConfig.Mtu {
		if err := c.ifHandler.SetInterfaceMtu(ifIdx, newConfig.Mtu); err != nil {
			return errors.Errorf("failed to set MTU to interface %s: %v", newConfig.Name, err)
		}
	} else if newConfig.Mtu == 0 && c.defaultMtu != 0 {
		if err := c.ifHandler.SetInterfaceMtu(ifIdx, c.defaultMtu); err != nil {
			return errors.Errorf("failed to set MTU to interface %s: %v", newConfig.Name, err)
		}
	}

	c.swIfIndexes.UpdateMetadata(newConfig.Name, newConfig)
	c.log.Debugf("Metadata updated in interface mapping for %s", newConfig.Name)

	return nil
}

/**
Modify rx-mode on specified VPP interface
*/
func (c *InterfaceConfigurator) modifyRxModeForInterfaces(oldIntf, newIntf *intf.Interfaces_Interface, ifIdx uint32) error {
	oldRx := oldIntf.RxModeSettings
	newRx := newIntf.RxModeSettings

	if oldRx == nil && newRx != nil || oldRx != nil && newRx == nil || !proto.Equal(oldRx, newRx) {
		// If new rx mode is nil, value is reset to default version (differs for interface types)
		switch newIntf.Type {
		case intf.InterfaceType_ETHERNET_CSMACD:
			if newRx == nil {
				return c.modifyRxMode(newIntf.Name, ifIdx, &intf.Interfaces_Interface_RxModeSettings{RxMode: intf.RxModeType_POLLING})
			} else if newRx.RxMode != intf.RxModeType_POLLING {
				return errors.Errorf("attempt to set unsupported rx-mode %s to Ethernet interface %s", newRx.RxMode, newIntf.Name)
			}
		case intf.InterfaceType_AF_PACKET_INTERFACE:
			if newRx == nil {
				return c.modifyRxMode(newIntf.Name, ifIdx, &intf.Interfaces_Interface_RxModeSettings{RxMode: intf.RxModeType_INTERRUPT})
			}
		default: // All the other interface types
			if newRx == nil {
				return c.modifyRxMode(newIntf.Name, ifIdx, &intf.Interfaces_Interface_RxModeSettings{RxMode: intf.RxModeType_DEFAULT})
			}
		}
		return c.modifyRxMode(newIntf.Name, ifIdx, newRx)
	}

	return nil
}

/**
Direct call of vpp api to change rx-mode of specified interface
*/
func (c *InterfaceConfigurator) modifyRxMode(ifName string, ifIdx uint32, rxMode *intf.Interfaces_Interface_RxModeSettings) error {
	if err := c.ifHandler.SetRxMode(ifIdx, rxMode); err != nil {
		return errors.Errorf("failed to set rx-mode for interface %s: %v", ifName, err)
	}
	return nil
}

// recreateVPPInterface removes and creates an interface from scratch.
func (c *InterfaceConfigurator) recreateVPPInterface(newConfig *intf.Interfaces_Interface,
	oldConfig *intf.Interfaces_Interface, ifIdx uint32) error {

	if oldConfig.Type == intf.InterfaceType_AF_PACKET_INTERFACE {
		if err := c.afPacketConfigurator.DeleteAfPacketInterface(oldConfig, ifIdx); err != nil {
			return err
		}
	} else {
		if err := c.deleteVPPInterface(oldConfig, ifIdx); err != nil {
			return err
		}
	}
	return c.ConfigureVPPInterface(newConfig)
}

// DeleteVPPInterface reacts to a removed NB configuration of a VPP interface.
// It results in the interface being removed from VPP.
func (c *InterfaceConfigurator) DeleteVPPInterface(iface *intf.Interfaces_Interface) error {
	// Remove VxLAN from cache if exists
	if iface.Type == intf.InterfaceType_VXLAN_TUNNEL {
		if _, ok := c.vxlanMulticastCache[iface.Name]; ok {
			delete(c.vxlanMulticastCache, iface.Name)
			c.log.Debugf("Interface %s removed from VxLAN multicast cache, will be removed", iface.Name)
			return nil
		}
	}

	if c.afPacketConfigurator.IsPendingAfPacket(iface) {
		ifIdx, _, found := c.afPacketConfigurator.ifIndexes.LookupIdx(iface.Name)
		if !found {
			// Just remove from cache
			c.afPacketConfigurator.removeFromCache(iface)
			return nil
		}

		return c.afPacketConfigurator.DeleteAfPacketInterface(iface, ifIdx)
	}

	// unregister name to init mapping (following triggers notifications for all subscribers, skip physical interfaces)
	if iface.Type != intf.InterfaceType_ETHERNET_CSMACD {
		ifIdx, prev, found := c.swIfIndexes.UnregisterName(iface.Name)
		if !found {
			return errors.Errorf("Unable to find interface %s in the mapping", iface.Name)
		}
		c.log.Debugf("Interface %s unregistered from interface mapping", iface.Name)

		// delete from unnumbered map (if the interface is present)
		delete(c.uIfaceCache, iface.Name)
		c.log.Debugf("Unnumbered interface %s removed from cache (will be removed)", iface.Name)

		if err := c.deleteVPPInterface(prev, ifIdx); err != nil {
			return err
		}
	} else {
		// Find index of the Physical interface and un-configure it
		ifIdx, prev, found := c.swIfIndexes.LookupIdx(iface.Name)
		if !found {
			return errors.Errorf("unable to find index for physical interface %s, cannot delete", iface.Name)
		}
		if err := c.deleteVPPInterface(prev, ifIdx); err != nil {
			return err
		}
	}

	c.log.Infof("Interface %v removed", iface.Name)

	return nil
}

func (c *InterfaceConfigurator) deleteVPPInterface(oldConfig *intf.Interfaces_Interface, ifIdx uint32) error {
	// Skip setting interface to ADMIN_DOWN unless the type AF_PACKET_INTERFACE
	if oldConfig.Type != intf.InterfaceType_AF_PACKET_INTERFACE {
		if err := c.ifHandler.InterfaceAdminDown(ifIdx); err != nil {
			return errors.Errorf("failed to set interface %s down: %v", oldConfig.Name, err)
		}
	}

	// Remove DHCP if it was set
	if oldConfig.SetDhcpClient {
		if err := c.ifHandler.UnsetInterfaceAsDHCPClient(ifIdx, oldConfig.Name); err != nil {
			return errors.Errorf("failed to unset interface %s as DHCP client: %v", oldConfig.Name, err)
		}
		// Remove from DHCP mapping
		c.dhcpIndexes.UnregisterName(oldConfig.Name)
		c.log.Debugf("Interface %v unregistered as DHCP client", oldConfig.Name)
	}

	if oldConfig.ContainerIpAddress != "" {
		if err := c.ifHandler.DelContainerIP(ifIdx, oldConfig.ContainerIpAddress); err != nil {
			return errors.Errorf("failed to delete container IP %s from interface %s: %v",
				oldConfig.ContainerIpAddress, oldConfig.Name, err)
		}
	}

	for i, oldIP := range oldConfig.IpAddresses {
		if strings.HasPrefix(oldIP, "fe80") {
			// TODO: skip link local addresses (possible workaround for af_packet)
			oldConfig.IpAddresses = append(oldConfig.IpAddresses[:i], oldConfig.IpAddresses[i+1:]...)
			c.log.Debugf("delete vpp interface %s: link local address %s skipped", oldConfig.Name, oldIP)
		}
	}
	oldAddrs, err := addrs.StrAddrsToStruct(oldConfig.IpAddresses)
	if err != nil {
		return errors.Errorf("failed to convert %s IP address list to IPNet structures: %v", oldConfig.Name, err)
	}
	for _, oldAddr := range oldAddrs {
		if err := c.ifHandler.DelInterfaceIP(ifIdx, oldAddr); err != nil {
			return errors.Errorf("failed to remove IP address %s from interface %s: %v",
				oldAddr, oldConfig.Name, err)
		}
	}

	// let's try to do following even if previously error occurred
	switch oldConfig.Type {
	case intf.InterfaceType_TAP_INTERFACE:
		err = c.ifHandler.DeleteTapInterface(oldConfig.Name, ifIdx, oldConfig.Tap.Version)
	case intf.InterfaceType_MEMORY_INTERFACE:
		err = c.ifHandler.DeleteMemifInterface(oldConfig.Name, ifIdx)
	case intf.InterfaceType_VXLAN_TUNNEL:
		err = c.ifHandler.DeleteVxLanTunnel(oldConfig.Name, ifIdx, oldConfig.Vrf, oldConfig.GetVxlan())
	case intf.InterfaceType_SOFTWARE_LOOPBACK:
		err = c.ifHandler.DeleteLoopbackInterface(oldConfig.Name, ifIdx)
	case intf.InterfaceType_ETHERNET_CSMACD:
		c.log.Debugf("Interface removal skipped: cannot remove (blacklist) physical interface") // Not an error
		return nil
	case intf.InterfaceType_AF_PACKET_INTERFACE:
		err = c.afPacketConfigurator.DeleteAfPacketInterface(oldConfig, ifIdx)
	}
	if err != nil {
		return errors.Errorf("failed to remove interface %s, index %d: %v", oldConfig.Name, ifIdx, err)
	}

	return nil
}

// ResolveCreatedLinuxInterface reacts to a newly created Linux interface.
func (c *InterfaceConfigurator) ResolveCreatedLinuxInterface(ifName, hostIfName string, ifIdx uint32) error {
	pendingAfpacket, err := c.afPacketConfigurator.ResolveCreatedLinuxInterface(ifName, hostIfName, ifIdx)
	if err != nil {
		return err
	}
	if pendingAfpacket != nil {
		// there is a pending af-packet that can be now configured
		return c.ConfigureVPPInterface(pendingAfpacket)
	}
	return nil
}

// ResolveDeletedLinuxInterface reacts to a removed Linux interface.
func (c *InterfaceConfigurator) ResolveDeletedLinuxInterface(ifName, hostIfName string, ifIdx uint32) error {
	return c.afPacketConfigurator.ResolveDeletedLinuxInterface(ifName, hostIfName, ifIdx)
}

// PropagateIfDetailsToStatus looks up all VPP interfaces
func (c *InterfaceConfigurator) propagateIfDetailsToStatus() error {
	req := &interfaces.SwInterfaceDump{}
	reqCtx := c.vppCh.SendMultiRequest(req)

	for {
		msg := &interfaces.SwInterfaceDetails{}
		stop, err := reqCtx.ReceiveReply(msg)
		if stop {
			break
		}
		if err != nil {
			return errors.Errorf("failed to receive interface dump details: %v", err)
		}

		_, _, found := c.swIfIndexes.LookupName(msg.SwIfIndex)
		if !found {
			c.log.Warnf("Unregistered interface %v with ID %v found on vpp",
				string(bytes.SplitN(msg.InterfaceName, []byte{0x00}, 2)[0]), msg.SwIfIndex)
			// Do not register unknown interface here, cuz it may cause inconsistencies in the ifplugin.
			// All new interfaces should be registered during configuration
			continue
		}

		// Propagate interface state information to notification channel.
		c.NotifChan <- msg
	}

	return nil
}

// returns memif socket filename ID. Registers it if does not exists yet
func (c *InterfaceConfigurator) resolveMemifSocketFilename(memifIf *intf.Interfaces_Interface_Memif) (uint32, error) {
	if memifIf.SocketFilename == "" {
		return 0, errors.Errorf("memif configuration does not contain socket file name")
	}
	registeredID, ok := c.memifScCache[memifIf.SocketFilename]
	if !ok {
		// Register new socket. ID is generated (default filename ID is 0, first is ID 1, second ID 2, etc)
		registeredID = uint32(len(c.memifScCache))
		err := c.ifHandler.RegisterMemifSocketFilename([]byte(memifIf.SocketFilename), registeredID)
		if err != nil {
			return 0, errors.Errorf("error registering socket file name %s (ID %d): %v", memifIf.SocketFilename, registeredID, err)
		}
		c.memifScCache[memifIf.SocketFilename] = registeredID
		c.log.Debugf("Memif socket filename %s registered under ID %d", memifIf.SocketFilename, registeredID)
	}
	return registeredID, nil
}

// Returns VxLAN multicast interface index if set and exists. Returns index of the interface an whether the vxlan was cached.
func (c *InterfaceConfigurator) getVxLanMulticast(vxlan *intf.Interfaces_Interface) (ifIdx uint32, cached bool, err error) {
	if vxlan.Vxlan == nil {
		c.log.Debugf("VxLAN multicast: no data available for %s", vxlan.Name)
		return 0, false, nil
	}
	if vxlan.Vxlan.Multicast == "" {
		c.log.Debugf("VxLAN %s has no multicast interface defined", vxlan.Name)
		return 0, false, nil
	}
	mcIfIdx, mcIf, found := c.swIfIndexes.LookupIdx(vxlan.Vxlan.Multicast)
	if !found {
		c.vxlanMulticastCache[vxlan.Name] = vxlan
		c.log.Debugf("multicast interface %s not found, cached", vxlan.Vxlan.Multicast, vxlan.Name)
		return 0, true, nil
	}
	// Check wheteher at least one of the addresses is from multicast range
	if len(mcIf.IpAddresses) == 0 {
		return 0, false, errors.Errorf("VxLAN %s refers to multicast interface %s which does not have any IP address",
			vxlan.Name, mcIf.Name)
	}
	var IPVerified bool
	for _, mcIfAddr := range mcIf.IpAddresses {
		mcIfAddrWithoutMask := strings.Split(mcIfAddr, "/")[0]
		IPVerified = net.ParseIP(mcIfAddrWithoutMask).IsMulticast()
		if IPVerified {
			if vxlan.Vxlan.DstAddress != mcIfAddr {
				c.log.Warn("VxLAN %s contains destination address %s which will be replaced with multicast %s",
					vxlan.Name, vxlan.Vxlan.DstAddress, mcIfAddr)
			}
			vxlan.Vxlan.DstAddress = mcIfAddrWithoutMask
			break
		}
	}
	if !IPVerified {
		return 0, false, errors.Errorf("VxLAN %s refers to multicast interface %s which does not have multicast IP address",
			vxlan.Name, mcIf.Name)
	}

	return mcIfIdx, false, nil
}

// Look over cached VxLAN multicast interfaces and configure them if possible
func (c *InterfaceConfigurator) resolveCachedVxLANMulticasts(createdIfName string) error {
	for vxlanName, vxlan := range c.vxlanMulticastCache {
		if vxlan.Vxlan.Multicast == createdIfName {
			delete(c.vxlanMulticastCache, vxlanName)
			c.log.Debugf("Interface %s removed from VxLAN multicast cache, will be configured", vxlanName)
			if err := c.ConfigureVPPInterface(vxlan); err != nil {
				return errors.Errorf("failed to configure VPP interface %s as VxLAN multicast: %v",
					createdIfName, err)
			}
		}
	}

	return nil
}

func (c *InterfaceConfigurator) canMemifBeModifWithoutDelete(newConfig *intf.Interfaces_Interface_Memif, oldConfig *intf.Interfaces_Interface_Memif) bool {
	if newConfig == nil || oldConfig == nil {
		return true
	}

	if !proto.Equal(newConfig, oldConfig) {
		c.log.Debug("Difference between new & old config causing recreation of memif")
		return false
	}

	return true
}

func (c *InterfaceConfigurator) canVxlanBeModifWithoutDelete(newConfig *intf.Interfaces_Interface_Vxlan, oldConfig *intf.Interfaces_Interface_Vxlan) bool {
	if newConfig == nil || oldConfig == nil {
		return true
	}
	if !proto.Equal(newConfig, oldConfig) {
		c.log.Debug("Difference between new & old config causing recreation of VxLAN")
		return false
	}

	return true
}

func (c *InterfaceConfigurator) canTapBeModifWithoutDelete(newConfig *intf.Interfaces_Interface_Tap, oldConfig *intf.Interfaces_Interface_Tap) bool {
	if newConfig == nil || oldConfig == nil {
		return true
	}
	if !proto.Equal(newConfig, oldConfig) {
		c.log.Debug("Difference between new & old config causing recreation of tap")
		return false
	}

	return true
}

// watch and process DHCP notifications. DHCP configuration is registered to dhcp mapping for every interface
func (c *InterfaceConfigurator) watchDHCPNotifications() {
	c.log.Debug("Started watcher on DHCP notifications")

	for {
		select {
		case notification := <-c.DhcpChan:
			switch dhcpNotif := notification.(type) {
			case *dhcp.DHCPComplEvent:
				var ipAddr, rIPAddr net.IP = dhcpNotif.Lease.HostAddress, dhcpNotif.Lease.RouterAddress
				var hwAddr net.HardwareAddr = dhcpNotif.Lease.HostMac
				var ipStr, rIPStr string

				name := string(bytes.SplitN(dhcpNotif.Lease.Hostname, []byte{0x00}, 2)[0])

				if dhcpNotif.Lease.IsIPv6 == 1 {
					ipStr = ipAddr.To16().String()
					rIPStr = rIPAddr.To16().String()
				} else {
					ipStr = ipAddr[:4].To4().String()
					rIPStr = rIPAddr[:4].To4().String()
				}

				c.log.Debugf("DHCP assigned %v to interface %q (router address %v)", ipStr, name, rIPStr)

				ifIdx, _, found := c.swIfIndexes.LookupIdx(name)
				if !found {
					c.log.Warnf("Expected interface %v not found in the mapping", name)
					continue
				}

				// Register DHCP config
				c.dhcpIndexes.RegisterName(name, ifIdx, &ifaceidx.DHCPSettings{
					IfName: name,
					IsIPv6: func(isIPv6 uint8) bool {
						if isIPv6 == 1 {
							return true
						}
						return false
					}(dhcpNotif.Lease.IsIPv6),
					IPAddress:     ipStr,
					Mask:          uint32(dhcpNotif.Lease.MaskWidth),
					PhysAddress:   hwAddr.String(),
					RouterAddress: rIPStr,
				})
				c.log.Debugf("Interface %s registered as DHCP client", name)
			}
		}
	}
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *InterfaceConfigurator) LogError(err error) error {
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

// Returns two flags, whether provided list of addresses contains IPv4 and/or IPv6 type addresses
func getIPAddressVersions(ipAddrs []*net.IPNet) (isIPv4, isIPv6 bool) {
	for _, ip := range ipAddrs {
		if ip.IP.To4() != nil {
			isIPv4 = true
		} else {
			isIPv6 = true
		}
	}

	return
}
