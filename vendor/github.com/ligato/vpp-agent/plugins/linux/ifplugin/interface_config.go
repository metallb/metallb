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
	"context"
	"net"
	"sync"

	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
	vppIf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/vishvananda/netlink"
)

// LinkNotFoundErr represents netlink error return value from 'GetLinkByName' if interface does not exist
const LinkNotFoundErr = "Link not found"

// LinuxInterfaceConfig is used to cache the configuration of Linux interfaces.
type LinuxInterfaceConfig struct {
	config *interfaces.LinuxInterfaces_Interface
	peer   *LinuxInterfaceConfig
}

// LinuxInterfaceConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of interfaces as modelled by the proto file "model/interfaces/interfaces.proto"
// and stored in ETCD under the key "/vnf-agent/{vnf-agent}/linux/config/v1/interface".
// Updates received from the northbound API are compared with the Linux network configuration and differences
// are applied through the Netlink API.
type LinuxInterfaceConfigurator struct {
	log logging.Logger

	// In-memory mappings
	ifIndexes ifaceidx.LinuxIfIndexRW
	ifIdxSeq  uint32

	// mapMu protects ifByName and ifsByMs maps
	mapMu    sync.RWMutex
	ifByName map[string]*LinuxInterfaceConfig   // interface name -> interface configuration
	ifsByMs  map[string][]*LinuxInterfaceConfig // microservice label -> list of interfaces attached to this microservice

	ifCachedConfigs    ifaceidx.LinuxIfIndexRW
	pIfCachedConfigSeq uint32

	// Channels
	ifNotif   chan *LinuxInterfaceStateNotification
	ifMsNotif chan *nsplugin.MicroserviceEvent

	// Go routine management
	cfgLock sync.Mutex
	ctx     context.Context    // Context within which all goroutines are running
	cancel  context.CancelFunc // Cancel can be used to cancel all goroutines and their jobs inside of the plugin.
	wg      sync.WaitGroup     // Wait group allows to wait until all goroutines of the plugin have finished.

	// Linux namespace/calls handler
	ifHandler  linuxcalls.NetlinkAPI
	nsHandler  nsplugin.NamespaceAPI
	sysHandler nsplugin.SystemAPI
}

// Init linux plugin and start go routines.
func (c *LinuxInterfaceConfigurator) Init(logging logging.PluginLogger, ifHandler linuxcalls.NetlinkAPI, nsHandler nsplugin.NamespaceAPI,
	sysHandler nsplugin.SystemAPI, ifIndexes ifaceidx.LinuxIfIndexRW, ifMsNotif chan *nsplugin.MicroserviceEvent,
	ifNotif chan *LinuxInterfaceStateNotification) (err error) {
	// Logger
	c.log = logging.NewLogger("if-conf")

	// Mappings
	c.ifIndexes = ifIndexes
	c.ifByName = make(map[string]*LinuxInterfaceConfig)
	c.ifsByMs = make(map[string][]*LinuxInterfaceConfig)
	c.ifIdxSeq = 1
	c.ifCachedConfigs = ifaceidx.NewLinuxIfIndex(nametoidx.NewNameToIdx(c.log, "linux_if_cache", nil))
	c.pIfCachedConfigSeq = 1

	// Set channels
	c.ifNotif = ifNotif
	c.ifMsNotif = ifMsNotif

	c.ctx, c.cancel = context.WithCancel(context.Background())

	// Interface and namespace handlers
	c.ifHandler = ifHandler
	c.nsHandler = nsHandler
	c.sysHandler = sysHandler

	// Start watching on linux and microservice events
	go c.watchLinuxStateUpdater()
	go c.watchMicroservices(c.ctx)

	c.log.Info("Linux interface configurator initialized")

	return err
}

// Close does nothing for linux interface configurator. State and notification channels are closed in linux plugin.
func (c *LinuxInterfaceConfigurator) Close() error {
	return nil
}

// GetLinuxInterfaceIndexes returns in-memory mapping of linux inerfaces
func (c *LinuxInterfaceConfigurator) GetLinuxInterfaceIndexes() ifaceidx.LinuxIfIndex {
	return c.ifIndexes
}

// GetInterfaceByNameCache returns cache of interface <-> config entries
func (c *LinuxInterfaceConfigurator) GetInterfaceByNameCache() map[string]*LinuxInterfaceConfig {
	return c.ifByName
}

// GetInterfaceByMsCache returns cache of microservice <-> interface list
func (c *LinuxInterfaceConfigurator) GetInterfaceByMsCache() map[string][]*LinuxInterfaceConfig {
	return c.ifsByMs
}

// GetCachedLinuxIfIndexes gives access to mapping of not configured interface indexes.
func (c *LinuxInterfaceConfigurator) GetCachedLinuxIfIndexes() ifaceidx.LinuxIfIndex {
	return c.ifCachedConfigs
}

// ConfigureLinuxInterface reacts to a new northbound Linux interface config by creating and configuring
// the interface in the host network stack through Netlink API.
func (c *LinuxInterfaceConfigurator) ConfigureLinuxInterface(linuxIf *interfaces.LinuxInterfaces_Interface) error {
	c.cfgLock.Lock()
	defer c.cfgLock.Unlock()

	c.handleOptionalHostIfName(linuxIf)

	// Linux interface type resolution
	switch linuxIf.Type {
	case interfaces.LinuxInterfaces_VETH:
		// Get peer interface config if exists and cache the original configuration with peer
		if linuxIf.Veth == nil {
			return errors.Errorf("VETH interface %v has no peer defined", linuxIf.HostIfName)
		}
		peerConfig := c.getInterfaceConfig(linuxIf.Veth.PeerIfName)
		ifConfig := c.addToCache(linuxIf, peerConfig)

		if err := c.configureVethInterface(ifConfig, peerConfig); err != nil {
			return err
		}
	case interfaces.LinuxInterfaces_AUTO_TAP:
		hostIfName := linuxIf.HostIfName
		if linuxIf.Tap != nil && linuxIf.Tap.TempIfName != "" {
			hostIfName = linuxIf.Tap.TempIfName
		}
		if err := c.configureTapInterface(hostIfName, linuxIf); err != nil {
			return err
		}
	default:
		return errors.Errorf("unknown linux interface type: %v", linuxIf.Type)
	}

	c.log.Infof("Linux interface %s with hostIfName %s configured", linuxIf.Name, linuxIf.HostIfName)

	return nil
}

// ModifyLinuxInterface applies changes in the NB configuration of a Linux interface into the host network stack
// through Netlink API.
func (c *LinuxInterfaceConfigurator) ModifyLinuxInterface(newLinuxIf, oldLinuxIf *interfaces.LinuxInterfaces_Interface) (err error) {
	// If host names are not defined, name == host name
	c.handleOptionalHostIfName(newLinuxIf)
	c.handleOptionalHostIfName(oldLinuxIf)

	if oldLinuxIf.Type != newLinuxIf.Type {
		return errors.Errorf("modify linux interface %s: type change not allowed", newLinuxIf.Name)
	}

	// Get old and new peer/host interfaces (peers for VETH, host for TAP)
	var oldPeer, newPeer string
	if oldLinuxIf.Type == interfaces.LinuxInterfaces_VETH && oldLinuxIf.Veth != nil {
		oldPeer = oldLinuxIf.Veth.PeerIfName
	} else if oldLinuxIf.Type == interfaces.LinuxInterfaces_AUTO_TAP {
		oldPeer = oldLinuxIf.HostIfName
	}
	if newLinuxIf.Type == interfaces.LinuxInterfaces_VETH && newLinuxIf.Veth != nil {
		newPeer = newLinuxIf.Veth.PeerIfName
	} else if newLinuxIf.Type == interfaces.LinuxInterfaces_AUTO_TAP {
		newPeer = newLinuxIf.HostIfName
	}

	// Prepare namespace objects of new and old interfaces
	newIfaceNs := c.nsHandler.IfNsToGeneric(newLinuxIf.Namespace)
	oldIfaceNs := c.nsHandler.IfNsToGeneric(oldLinuxIf.Namespace)
	if newPeer != oldPeer || newLinuxIf.HostIfName != oldLinuxIf.HostIfName || newIfaceNs.CompareNamespaces(oldIfaceNs) != 0 {
		// Change of the peer interface (VETH) or host (TAP) or the namespace requires to create the interface from the scratch.
		err := c.DeleteLinuxInterface(oldLinuxIf)
		if err == nil {
			err = c.ConfigureLinuxInterface(newLinuxIf)
		}
		return err
	}

	c.cfgLock.Lock()
	defer c.cfgLock.Unlock()

	// Update the cached configuration.
	c.removeFromCache(oldLinuxIf)
	peer := c.getInterfaceConfig(newPeer)
	c.addToCache(newLinuxIf, peer)

	// Verify required namespace
	if !c.nsHandler.IsNamespaceAvailable(newLinuxIf.Namespace) {
		return errors.Errorf("failed to modify linux interface %s: new namespace is not available",
			newLinuxIf.Name)
	}

	// Validate peer for veth
	if newLinuxIf.Type == interfaces.LinuxInterfaces_VETH {
		if peer == nil {
			// Interface doesn't actually exist physically.
			return errors.Errorf("unable to modify linux veth interface %v: peer interface %v is not configured yet",
				newLinuxIf.HostIfName, newPeer)
		}
		if !c.nsHandler.IsNamespaceAvailable(oldLinuxIf.Namespace) {
			return errors.Errorf("unable to modify linux veth interface %v: peer interface namespace is not available",
				oldLinuxIf.HostIfName)
		}
	}

	// The namespace was not changed so interface can be reconfigured
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()

	if err := c.modifyLinuxInterface(nsMgmtCtx, oldLinuxIf, newLinuxIf); err != nil {
		return err
	}

	c.log.Infof("Linux interface %s modified", newLinuxIf.Name)
	return
}

// DeleteLinuxInterface reacts to a removed NB configuration of a Linux interface.
func (c *LinuxInterfaceConfigurator) DeleteLinuxInterface(linuxIf *interfaces.LinuxInterfaces_Interface) error {
	c.cfgLock.Lock()
	defer c.cfgLock.Unlock()

	c.handleOptionalHostIfName(linuxIf)

	oldConfig := c.removeFromCache(linuxIf)
	var peerConfig *LinuxInterfaceConfig
	if oldConfig != nil {
		peerConfig = oldConfig.peer
	}

	if linuxIf.Type == interfaces.LinuxInterfaces_AUTO_TAP {
		if _, _, exists := c.ifCachedConfigs.LookupIdx(linuxIf.HostIfName); exists {
			// Unregister TAP from the in-memory map
			c.ifCachedConfigs.UnregisterName(linuxIf.HostIfName)
			c.log.Debugf("tap linux interface removed from cache %s", linuxIf.Name)
			return nil
		}

		if err := c.deleteTapInterface(oldConfig); err != nil {
			return err
		}
	} else if linuxIf.Type == interfaces.LinuxInterfaces_VETH {
		if err := c.deleteVethInterface(oldConfig, peerConfig); err != nil {
			return err
		}
	}

	c.log.Infof("Linux interface %s removed", linuxIf.HostIfName)

	return nil
}

// Validate, create and configure VETH type linux interface
func (c *LinuxInterfaceConfigurator) configureVethInterface(ifConfig, peerConfig *LinuxInterfaceConfig) error {
	// Create VETH after both end's configs and target namespaces are available.
	if peerConfig == nil {
		c.log.Infof("cannot configure linux interface %s: peer interface %s is not configured yet",
			ifConfig.config.HostIfName, ifConfig.config.Veth.PeerIfName)
		return nil
	}
	if !c.nsHandler.IsNamespaceAvailable(ifConfig.config.Namespace) {
		return errors.Errorf("failed to configure veth interface %s: namespace %q is not available",
			ifConfig.config.Name, ifConfig.config.Namespace)
	}
	if !c.nsHandler.IsNamespaceAvailable(peerConfig.config.Namespace) {
		return errors.Errorf("failed to configure veth interface %s: peer namespace %q is not available",
			ifConfig.config.Name, peerConfig.config.Namespace)
	}

	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()

	// Prepare generic veth config namespace object
	vethNs := c.nsHandler.IfNsToGeneric(c.nsHandler.GetConfigNamespace())

	// Switch to veth cfg namespace
	revertNs, err := c.nsHandler.SwitchNamespace(vethNs, nsMgmtCtx)
	if err != nil {
		return errors.Errorf("failed to configure veth interface %s, error switching namespaces: %v",
			ifConfig.config.Name, err)
	}
	defer revertNs()

	if err := c.addVethInterfacePair(nsMgmtCtx, ifConfig.config, peerConfig.config); err != nil {
		return err
	}

	if err := c.configureLinuxInterface(nsMgmtCtx, ifConfig.config); err != nil {
		return err
	}
	if err := c.configureLinuxInterface(nsMgmtCtx, peerConfig.config); err != nil {
		return err
	}

	return nil
}

// Validate and apply linux TAP configuration to the interface. The interface is not created here, it is added
// to the default namespace when it's VPP end is configured
func (c *LinuxInterfaceConfigurator) configureTapInterface(hostIfName string, linuxIf *interfaces.LinuxInterfaces_Interface) error {
	// TAP (auto) interface looks for existing interface with the same host name or temp name (cached without peer)
	var ifConfig *LinuxInterfaceConfig
	_, ifConfigData, exists := c.ifCachedConfigs.LookupIdx(hostIfName)
	if exists {
		ifConfig = c.addToCache(ifConfigData.Data, nil)
		ifConfig.config = ifConfigData.Data
		c.log.Debugf("linux tap interface %s exists, can be configured", hostIfName)
	} else {
		if linuxIf != nil {
			ifConfig = c.addToCache(linuxIf, nil)
			c.ifCachedConfigs.RegisterName(hostIfName, c.pIfCachedConfigSeq, &ifaceidx.IndexedLinuxInterface{
				Index: c.pIfCachedConfigSeq,
				Data:  ifConfig.config,
			})
			c.pIfCachedConfigSeq++
			c.log.Debugf("creating new Linux Tap interface %v configuration entry", hostIfName)
		} else {
			c.log.Infof("there is no linux tap configuration entry for interface %v", hostIfName)
			return nil
		}
	}

	// Tap interfaces can be processed directly using config and also via linux interface events. This check
	// should prevent to process the same interface multiple times.
	_, _, exists = c.ifIndexes.LookupIdx(ifConfig.config.Name)
	if exists {
		c.log.Debugf("TAP interface %s already processed", ifConfig.config.Name)
		return nil
	}

	// At this point, agent knows that VPP TAP interface exists, so let's find out its linux side. First, check
	// if temporary name is defined
	if ifConfig.config.Tap == nil || ifConfig.config.Tap.TempIfName == "" {
		// In such a case, set temp name as host (look for interface named as host name)
		ifConfig.config.Tap = &interfaces.LinuxInterfaces_Interface_Tap{
			TempIfName: ifConfig.config.HostIfName,
		}
	}
	// Now look for interface
	_, err := c.ifHandler.GetLinkByName(ifConfig.config.Tap.TempIfName)
	if err != nil && err.Error() == LinkNotFoundErr {
		c.log.Debugf("linux tap interface %s is registered but not ready yet, configuration postponed",
			ifConfig.config.Tap.TempIfName)
		return nil
	} else if err != nil {
		return errors.Errorf("failed to read TAP interface %s from linux: %v", ifConfig.config.Tap.TempIfName, err)
	}

	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()

	// Verify availability of namespace from configuration
	if !c.nsHandler.IsNamespaceAvailable(ifConfig.config.Namespace) {
		return errors.Errorf("failed to apply linux tap %s config: destination namespace %s is not available",
			ifConfig.config.Name, ifConfig.config.Namespace)
	}

	c.ifCachedConfigs.UnregisterName(hostIfName)
	c.log.Debugf("cached linux tap interface %s unregistered", ifConfig.config.Name)

	return c.configureLinuxInterface(nsMgmtCtx, ifConfig.config)
}

// Set linux interface to proper namespace and configure attributes
func (c *LinuxInterfaceConfigurator) configureLinuxInterface(nsMgmtCtx *nsplugin.NamespaceMgmtCtx, ifConfig *interfaces.LinuxInterfaces_Interface) (err error) {
	if ifConfig.HostIfName == "" {
		return errors.Errorf("failed to configure linux interface %s: required host name not specified", ifConfig.Name)
	}

	// Use temporary/host name (according to type) to set interface to different namespace
	if ifConfig.Type == interfaces.LinuxInterfaces_AUTO_TAP {
		err = c.setInterfaceNamespace(nsMgmtCtx, ifConfig.Tap.TempIfName, ifConfig.Namespace)
		if err != nil {
			return errors.Errorf("failed to set TAP interface %s to namespace %s: %v",
				ifConfig.Tap.TempIfName, ifConfig.Namespace, err)
		}
	} else {
		err = c.setInterfaceNamespace(nsMgmtCtx, ifConfig.HostIfName, ifConfig.Namespace)
		if err != nil {
			return errors.Errorf("failed to set interface %s to namespace %s: %v",
				ifConfig.HostIfName, ifConfig.Namespace, err)
		}
	}
	// Continue configuring interface in its namespace.
	revertNs, err := c.nsHandler.SwitchToNamespace(nsMgmtCtx, ifConfig.Namespace)
	if err != nil {
		return errors.Errorf("failed to switch network namespace: %v", err)
	}
	defer revertNs()

	// For TAP interfaces only - rename interface to the actual host name if needed
	if ifConfig.Type == interfaces.LinuxInterfaces_AUTO_TAP {
		if ifConfig.HostIfName != ifConfig.Tap.TempIfName {
			if err := c.ifHandler.RenameInterface(ifConfig.Tap.TempIfName, ifConfig.HostIfName); err != nil {
				return errors.Errorf("failed to rename TAP interface from %s to %s: %v", ifConfig.Tap.TempIfName,
					ifConfig.HostIfName, err)
			}
		} else {
			c.log.Debugf("Renaming of TAP interface %v skipped, host name is the same as temporary", ifConfig.HostIfName)
		}
	}

	// Set interface up.
	if ifConfig.Enabled {
		err := c.ifHandler.SetInterfaceUp(ifConfig.HostIfName)
		if nil != err {
			return errors.Errorf("failed to set linux interface %s up: %v", ifConfig.Name, err)
		}
	}

	// Set interface MAC address
	if ifConfig.PhysAddress != "" {
		err = c.ifHandler.SetInterfaceMac(ifConfig.HostIfName, ifConfig.PhysAddress)
		if err != nil {
			return errors.Errorf("failed to set MAC address %s to linux interface %s: %v",
				ifConfig.PhysAddress, ifConfig.Name, err)
		}
	}

	// Set interface IP addresses
	ipAddresses, err := addrs.StrAddrsToStruct(ifConfig.IpAddresses)
	if err != nil {
		return errors.Errorf("failed to convert IP addresses %s: %v", ipAddresses, err)
	}
	// Get all configured interface addresses
	confAddresses, err := c.ifHandler.GetAddressList(ifConfig.HostIfName)
	if err != nil {
		return errors.Errorf("failed to read IP addresses from linux interface %s: %v", ifConfig.Name, err)
	}
	for _, ipAddress := range ipAddresses {
		// Check link local addresses which cannot be reassigned
		if addressExists(confAddresses, ipAddress) {
			c.log.Debugf("Cannot assign %s to linux interface %s, IP already exists",
				ipAddress.IP.String(), ifConfig.HostIfName)
			continue
		}
		err = c.ifHandler.AddInterfaceIP(ifConfig.HostIfName, ipAddress)
		if err != nil {
			return errors.Errorf("faield to add IP address %s to linux interface %s: %v",
				ipAddress, ifConfig.Name, err)
		}
	}

	if ifConfig.Mtu != 0 {
		if err := c.ifHandler.SetInterfaceMTU(ifConfig.HostIfName, int(ifConfig.Mtu)); err != nil {
			return errors.Errorf("failed to set MTU %d to linux interface %s: %v",
				ifConfig.Mtu, ifConfig.Name, err)
		}
	}

	netIf, err := c.ifHandler.GetInterfaceByName(ifConfig.HostIfName)
	if err != nil {
		return errors.Errorf("failed to get index of the linux interface %s: %v", ifConfig.HostIfName, err)
	}

	// Register interface with its original name and store host name in metadata
	c.ifIndexes.RegisterName(ifConfig.Name, c.ifIdxSeq, &ifaceidx.IndexedLinuxInterface{
		Index: uint32(netIf.Index),
		Data:  ifConfig,
	})
	c.ifIdxSeq++
	c.log.Debugf("Linux interface %s registered", ifConfig.Name)

	return nil
}

// Update linux interface attributes in it's namespace
func (c *LinuxInterfaceConfigurator) modifyLinuxInterface(nsMgmtCtx *nsplugin.NamespaceMgmtCtx,
	oldIfConfig, newIfConfig *interfaces.LinuxInterfaces_Interface) error {
	// Switch to required namespace
	revertNs, err := c.nsHandler.SwitchToNamespace(nsMgmtCtx, oldIfConfig.Namespace)
	if err != nil {
		return errors.Errorf("linux interface %s modify: failed to switch to namespace: %v", newIfConfig.Name, err)
	}
	defer revertNs()

	// Check if the interface already exists in the Linux namespace. If not, it still may be configured somewhere else.
	_, err = c.ifHandler.GetInterfaceByName(oldIfConfig.HostIfName)
	if err != nil {
		c.log.Debugf("Host interface %v was not found: %v", oldIfConfig.HostIfName, err)
		// If host does not exist, configure new setup as a new one
		return c.ConfigureLinuxInterface(newIfConfig)
	}

	// Set admin status.
	if newIfConfig.Enabled != oldIfConfig.Enabled {
		if newIfConfig.Enabled {
			err = c.ifHandler.SetInterfaceUp(newIfConfig.HostIfName)
		} else {
			err = c.ifHandler.SetInterfaceDown(newIfConfig.HostIfName)
		}
		if nil != err {
			return errors.Errorf("failed to enable/disable Linux interface %s: %v", newIfConfig.Name, err)
		}
	}

	// Configure new MAC address if set.
	if newIfConfig.PhysAddress != "" && newIfConfig.PhysAddress != oldIfConfig.PhysAddress {
		err := c.ifHandler.SetInterfaceMac(newIfConfig.HostIfName, newIfConfig.PhysAddress)
		if err != nil {
			return errors.Errorf("failed to reconfigure MAC address for linux interface %s: %v",
				newIfConfig.Name, err)
		}
	}

	// IP addresses
	newAddrs, err := addrs.StrAddrsToStruct(newIfConfig.IpAddresses)
	if err != nil {
		return errors.Errorf("linux interface modify: failed to convert IP addresses for %s: %v",
			newIfConfig.Name, err)
	}
	oldAddrs, err := addrs.StrAddrsToStruct(oldIfConfig.IpAddresses)
	if err != nil {
		return errors.Errorf("linux interface modify: failed to convert IP addresses for %s: %v",
			newIfConfig.Name, err)
	}
	var del, add []*net.IPNet
	del, add = addrs.DiffAddr(newAddrs, oldAddrs)

	for i := range del {
		err := c.ifHandler.DelInterfaceIP(newIfConfig.HostIfName, del[i])
		if nil != err {
			return errors.Errorf("failed to remove IPv4 address from a Linux interface %s: %v",
				newIfConfig.Name, err)
		}
	}

	// Get all configured interface addresses
	confAddresses, err := c.ifHandler.GetAddressList(newIfConfig.HostIfName)
	if err != nil {
		return errors.Errorf("linux interface modify: failed to read IP addresses from %s: %v",
			newIfConfig.Name, err)
	}
	for i := range add {
		c.log.WithFields(logging.Fields{"IP address": add[i], "hostIfName": newIfConfig.HostIfName}).Debug("IP address added.")
		// Check link local addresses which cannot be reassigned
		if addressExists(confAddresses, add[i]) {
			c.log.Debugf("Cannot assign %s to interface %s, IP already exists",
				add[i].IP.String(), newIfConfig.HostIfName)
			continue
		}
		err := c.ifHandler.AddInterfaceIP(newIfConfig.HostIfName, add[i])
		if nil != err {
			return errors.Errorf("linux interface modify: failed to add IP addresses %s to %s: %v",
				add[i], newIfConfig.Name, err)
		}
	}

	// MTU
	if newIfConfig.Mtu != oldIfConfig.Mtu {
		mtu := newIfConfig.Mtu
		if mtu > 0 {
			err := c.ifHandler.SetInterfaceMTU(newIfConfig.HostIfName, int(mtu))
			if nil != err {
				return errors.Errorf("failed to reconfigure MTU for the linux interface %s: %v",
					newIfConfig.Name, err)
			}
		}
	}

	return nil
}

// Remove VETH type interface
func (c *LinuxInterfaceConfigurator) deleteVethInterface(ifConfig, peerConfig *LinuxInterfaceConfig) error {
	// Veth interface removal
	if ifConfig == nil || ifConfig.config == nil || !c.nsHandler.IsNamespaceAvailable(ifConfig.config.Namespace) ||
		peerConfig == nil || peerConfig.config == nil || !c.nsHandler.IsNamespaceAvailable(peerConfig.config.Namespace) {
		name := "<unknown>"
		if ifConfig != nil && ifConfig.config != nil {
			name = ifConfig.config.Name
		}
		c.log.Debug("VETH interface %s doesn't exist, nothing to remove", name)
		return nil
	}

	// Move to the namespace with the interface.
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	revertNs, err := c.nsHandler.SwitchToNamespace(nsMgmtCtx, ifConfig.config.Namespace)
	if err != nil {
		return errors.Errorf("delete interface %s: failed to switch network namespace: %v",
			ifConfig.config.Name, err)
	}
	defer revertNs()

	err = c.ifHandler.DelVethInterfacePair(ifConfig.config.HostIfName, peerConfig.config.HostIfName)
	if err != nil {
		return errors.Errorf("failed to delete VETH pait %s-%s: %v",
			ifConfig.config.Name, peerConfig.config.Name, err)
	}

	// Unregister both VETH ends from the in-memory map (following triggers notifications for all subscribers).
	c.ifIndexes.UnregisterName(ifConfig.config.Name)
	c.ifIndexes.UnregisterName(peerConfig.config.Name)
	c.log.Debugf("Interface %s and its peer %s were unregistered",
		ifConfig.config.Name, peerConfig.config.Name)

	c.log.Infof("Linux Interface %s removed", ifConfig.config.Name)

	return nil
}

func (c *LinuxInterfaceConfigurator) moveTapInterfaceToDefaultNamespace(ifConfig *interfaces.LinuxInterfaces_Interface) error {
	if !c.nsHandler.IsNamespaceAvailable(ifConfig.Namespace) {
		return errors.Errorf("failed to move tap interface %s to default namespace: namespace not available", ifConfig.Name)
	}

	// Move to the namespace with the interface.
	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()
	revertNs, err := c.nsHandler.SwitchToNamespace(nsMgmtCtx, ifConfig.Namespace)
	if err != nil {
		return errors.Errorf("failed to move tap interface %s to default namespace: cannot switch namespace: %v",
			ifConfig.Name, err)
	}
	defer revertNs()

	// Get all IP addresses currently configured on the interface. It is not enough to just remove all IP addresses
	// present in the ifConfig object, there can be default IP address which needs to be removed as well.
	var ipAddresses []*net.IPNet
	link, err := c.ifHandler.GetLinkList()
	for _, linuxIf := range link {
		if linuxIf.Attrs().Name == ifConfig.HostIfName {
			IPlist, err := c.ifHandler.GetAddressList(linuxIf.Attrs().Name)
			if err != nil {
				return errors.Errorf("failed to read IP addresses from interface %s: %v",
					ifConfig.Name, err)
			}
			for _, address := range IPlist {
				ipAddresses = append(ipAddresses, address.IPNet)
			}
			break
		}
	}
	// Remove all IP addresses from the TAP
	for _, ipAddress := range ipAddresses {
		if err := c.ifHandler.DelInterfaceIP(ifConfig.HostIfName, ipAddress); err != nil {
			return errors.Errorf("failed to remove IP address %s from interface %s: %v",
				ipAddresses, ifConfig.HostIfName, err)
		}
	}

	// Move back to default namespace
	if ifConfig.Type == interfaces.LinuxInterfaces_AUTO_TAP {
		// Rename to its original name (if possible)
		if ifConfig.Tap == nil || ifConfig.Tap.TempIfName == "" {
			c.log.Debugf("Cannot restore linux tap interface %s, original name (temp) is not available", ifConfig.HostIfName)
			ifConfig.Tap = &interfaces.LinuxInterfaces_Interface_Tap{
				TempIfName: ifConfig.HostIfName,
			}
		}
		if ifConfig.Tap.TempIfName == ifConfig.HostIfName {
			c.log.Debugf("Renaming of TAP interface %v skipped, host name is the same as temporary", ifConfig.HostIfName)
		} else {
			if err := c.ifHandler.RenameInterface(ifConfig.HostIfName, ifConfig.Tap.TempIfName); err != nil {

				return errors.Errorf("failed to rename TAP interface from %s to %s: %v", ifConfig.HostIfName,
					ifConfig.Tap.TempIfName, err)
			}
		}
		err = c.setInterfaceNamespace(nsMgmtCtx, ifConfig.Tap.TempIfName, &interfaces.LinuxInterfaces_Interface_Namespace{})
		if err != nil {
			return errors.Errorf("failed to set Linux TAP interface %s to default namespace: %v", ifConfig.Tap.TempIfName, err)
		}
	} else {
		err = c.setInterfaceNamespace(nsMgmtCtx, ifConfig.HostIfName, &interfaces.LinuxInterfaces_Interface_Namespace{})
		if err != nil {
			return errors.Errorf("failed to set Linux TAP interface %s to default namespace: %v", ifConfig.HostIfName, err)
		}
	}
	return nil
}

// Un-configure TAP interface, set original name and return it to the default namespace (do not delete,
// the interface will be removed together with the peer (VPP TAP))
func (c *LinuxInterfaceConfigurator) deleteTapInterface(ifConfig *LinuxInterfaceConfig) error {
	if ifConfig == nil || ifConfig.config == nil {
		return errors.Errorf("failed to remove tap interface, no data available")
	}

	// Move to default namespace
	err := c.moveTapInterfaceToDefaultNamespace(ifConfig.config)
	if err != nil {
		return err
	}
	// Unregister TAP from the in-memory map
	c.ifIndexes.UnregisterName(ifConfig.config.Name)
	c.log.Debugf("Interface %s unregistered", ifConfig.config)

	return nil
}

// removeObsoleteVeth deletes VETH interface which should no longer exist.
func (c *LinuxInterfaceConfigurator) removeObsoleteVeth(nsMgmtCtx *nsplugin.NamespaceMgmtCtx, vethName, hostIfName string, ns *interfaces.LinuxInterfaces_Interface_Namespace) error {
	revertNs, err := c.nsHandler.SwitchToNamespace(nsMgmtCtx, ns)
	defer revertNs()
	if err != nil {
		// Already removed as namespace no longer exists.
		c.ifIndexes.UnregisterName(vethName)
		c.log.Debugf("obsolete veth %s namespace does not exist, unregistered", vethName)
		return nil
	}
	exists, err := c.ifHandler.InterfaceExists(hostIfName)
	if err != nil {
		return errors.Errorf("failed to verify veth %s (hostname %s) presence: %v", vethName, hostIfName, err)
	}
	if !exists {
		// already removed
		if _, _, exists := c.ifIndexes.UnregisterName(vethName); exists {
			c.log.Debugf("obsolete veth %s does not exist, unregistered", vethName)
		}
		return nil
	}
	ifType, err := c.ifHandler.GetInterfaceType(hostIfName)
	if err != nil {
		return errors.Errorf("failed to get obsolete veth %s (hostname %s) type: %v", vethName, hostIfName, err)
	}
	if ifType != veth {
		return errors.Errorf("obsolete veth %s exists, but it is not an veth type interface", vethName)
	}
	peerName, err := c.ifHandler.GetVethPeerName(hostIfName)
	if err != nil {
		return errors.Errorf("failed to get veth %s (hostname %s) peer name: %v", vethName, hostIfName, err)
	}
	err = c.ifHandler.DelVethInterfacePair(hostIfName, peerName)
	if err != nil {
		return errors.Errorf("failed to remove obsolete veth %s: %v", vethName, err)
	}
	c.ifIndexes.UnregisterName(vethName)
	c.log.Debugf("obsolete veth %s unregistered", vethName)
	return nil
}

// addVethInterfacePair creates a new VETH interface with a "clean" configuration.
func (c *LinuxInterfaceConfigurator) addVethInterfacePair(nsMgmtCtx *nsplugin.NamespaceMgmtCtx,
	iface, peer *interfaces.LinuxInterfaces_Interface) error {
	err := c.removeObsoleteVeth(nsMgmtCtx, iface.Name, iface.HostIfName, iface.Namespace)
	if err != nil {
		return err
	}
	err = c.removeObsoleteVeth(nsMgmtCtx, peer.Name, peer.HostIfName, peer.Namespace)
	if err != nil {
		return err
	}
	// VETH is first created in its own cfg namespace so it has to be removed there as well.
	err = c.removeObsoleteVeth(nsMgmtCtx, iface.Name, iface.HostIfName, c.nsHandler.GetConfigNamespace())
	if err != nil {
		return err
	}
	err = c.removeObsoleteVeth(nsMgmtCtx, peer.Name, peer.HostIfName, c.nsHandler.GetConfigNamespace())
	if err != nil {
		return err
	}
	err = c.ifHandler.AddVethInterfacePair(iface.HostIfName, peer.HostIfName)
	if err != nil {
		return errors.Errorf("failed to create new VETH %s: %v", iface.Name, err)
	}

	return nil
}

// getInterfaceConfig returns cached configuration of a given interface.
func (c *LinuxInterfaceConfigurator) getInterfaceConfig(ifName string) *LinuxInterfaceConfig {
	c.mapMu.RLock()
	defer c.mapMu.RUnlock()

	config, ok := c.ifByName[ifName]
	if ok {
		return config
	}
	return nil
}

// addToCache adds interface configuration into the cache.
func (c *LinuxInterfaceConfigurator) addToCache(iface *interfaces.LinuxInterfaces_Interface, peerIface *LinuxInterfaceConfig) *LinuxInterfaceConfig {
	c.log.Debugf("linux interface config %s cached to if-by-name", iface.Name)

	c.mapMu.Lock()
	defer c.mapMu.Unlock()

	config := &LinuxInterfaceConfig{
		config: iface,
		peer:   peerIface,
	}
	c.ifByName[iface.Name] = config
	if peerIface != nil {
		peerIface.peer = config
	}

	if iface.Namespace != nil && iface.Namespace.Type == interfaces.LinuxInterfaces_Interface_Namespace_MICROSERVICE_REF_NS {
		if _, ok := c.ifsByMs[iface.Namespace.Microservice]; ok {
			c.ifsByMs[iface.Namespace.Microservice] = append(c.ifsByMs[iface.Namespace.Microservice], config)
			c.log.Debugf("linux interface config %s cached for microservice-ref-ns", iface.Name)
		} else {
			c.ifsByMs[iface.Namespace.Microservice] = []*LinuxInterfaceConfig{config}
			c.log.Debugf("linux interface config %s cached for microservice", iface.Name)
		}
	}
	return config
}

// removeFromCache removes interfaces configuration from the cache.
func (c *LinuxInterfaceConfigurator) removeFromCache(iface *interfaces.LinuxInterfaces_Interface) *LinuxInterfaceConfig {
	c.mapMu.Lock()
	defer c.mapMu.Unlock()

	if config, ok := c.ifByName[iface.Name]; ok {
		if config.peer != nil {
			config.peer.peer = nil
		}
		if iface.Namespace != nil && iface.Namespace.Type == interfaces.LinuxInterfaces_Interface_Namespace_MICROSERVICE_REF_NS {
			var filtered []*LinuxInterfaceConfig
			for _, intf := range c.ifsByMs[iface.Namespace.Microservice] {
				if intf.config.Name != iface.Name {
					filtered = append(filtered, intf)
				}
			}
			c.ifsByMs[iface.Namespace.Microservice] = filtered
		}

		delete(c.ifByName, iface.Name)
		c.log.Debugf("Linux interface with name %v was removed from if-by-name cache", iface.Name)

		return config
	}
	return nil
}

// Watcher receives events from state updater about created/removed linux interfaces and performs appropriate actions
func (c *LinuxInterfaceConfigurator) watchLinuxStateUpdater() {
	c.log.Debugf("Linux interface state watcher started")

	for {
		linuxIf, ok := <-c.ifNotif
		if !ok {
			c.log.Debugf("linux interface watcher ended")
			return
		}
		ifName := linuxIf.attributes.Name

		switch {
		case linuxIf.interfaceType == tap:
			if linuxIf.interfaceState == netlink.OperDown {
				// Find whether it is a registered tap interface and un-register it. Otherwise the change is ignored.
				for _, idxName := range c.ifIndexes.GetMapping().ListNames() {
					_, ifMeta, found := c.ifIndexes.LookupIdx(idxName)
					if !found {
						// Should not happen
						c.log.Warnf("Interface %s not found in the mapping", idxName)
						continue
					}
					if ifMeta == nil {
						// Should not happen
						c.log.Warnf("Interface %s metadata does not exist", idxName)
						continue
					}
					if ifMeta.Data.HostIfName == "" {
						c.log.Warnf("No info about host name for %s", idxName)
						continue
					}
					if ifMeta.Data.HostIfName == ifName {
						// Registered Linux TAP interface was removed, add it to cache. Pull out metadata, so they can be
						// saved in cache as well
						_, unregMeta, _ := c.ifIndexes.UnregisterName(ifName)
						c.log.Debugf("Tap interface %s unregistered according to linux state event", ifName)
						if _, _, found := c.ifCachedConfigs.LookupIdx(ifName); !found && unregMeta != nil {
							c.ifCachedConfigs.RegisterName(ifMeta.Data.HostIfName, c.pIfCachedConfigSeq, unregMeta)
							c.pIfCachedConfigSeq++
							c.log.Debugf("removed linux TAP %s registered to cache according to linux state event",
								ifName)
						}
					}
				}
			} else {
				// Event that a TAP interface was created. Look for TAP which is using this interface as the other end.
				for _, cachedIfConfig := range c.getCachedIfConfigByName(ifName) {
					// Host interface was found, configure linux TAP
					err := c.ConfigureLinuxInterface(cachedIfConfig)
					if err != nil {
						c.LogError(errors.Errorf("failed to process linux interface %s creation from event: %v", ifName, err))
					}
				}
			}
		default:
			c.log.Debugf("Linux interface type %v state processing skipped", linuxIf.interfaceType)
		}

	}
}

func (c *LinuxInterfaceConfigurator) getCachedIfConfigByName(ifName string) (ifConfigs []*interfaces.LinuxInterfaces_Interface) {
	c.mapMu.RLock()
	defer c.mapMu.RUnlock()

	for _, ifConfig := range c.ifByName {
		if ifConfig == nil || ifConfig.config == nil {
			c.log.Warnf("Cached config for interface %s is empty", ifName)
			continue
		}
		if ifConfig.config.GetTap().GetTempIfName() == ifName || ifConfig.config.GetHostIfName() == ifName {
			// Skip processed interfaces
			_, _, exists := c.ifIndexes.LookupIdx(ifConfig.config.Name)
			if exists {
				continue
			}
			ifConfigs = append(ifConfigs, ifConfig.config)
		}
	}
	return
}

func (c *LinuxInterfaceConfigurator) getIfsByMsLabel(label string) []*LinuxInterfaceConfig {
	c.mapMu.RLock()
	defer c.mapMu.RUnlock()

	return c.ifsByMs[label]
}

// watchMicroservices handles events from namespace plugin
func (c *LinuxInterfaceConfigurator) watchMicroservices(ctx context.Context) {
	c.wg.Add(1)
	defer c.wg.Done()

	nsMgmtCtx := nsplugin.NewNamespaceMgmtCtx()

	for {
		select {
		case msEvent := <-c.ifMsNotif:
			if msEvent == nil {
				continue
			}
			microservice := msEvent.Microservice
			if microservice == nil {
				c.log.Error("Empty microservice event")
				continue
			}
			if msEvent.EventType == nsplugin.NewMicroservice {
				skip := make(map[string]struct{}) /* interfaces to be skipped in subsequent iterations */

				for _, iface := range c.getIfsByMsLabel(microservice.Label) {
					if _, toSkip := skip[iface.config.Name]; toSkip {
						continue
					}
					peer := iface.peer
					if peer != nil {
						// peer will be processed in this iteration and skipped in the subsequent ones.
						skip[peer.config.Name] = struct{}{}
					}
					if peer != nil && c.nsHandler.IsNamespaceAvailable(peer.config.Namespace) {
						// Prepare generic vet cfg namespace object
						ifaceNs := c.nsHandler.IfNsToGeneric(c.nsHandler.GetConfigNamespace())

						// Switch to veth cfg namespace
						revertNs, err := c.nsHandler.SwitchNamespace(ifaceNs, nsMgmtCtx)
						if err != nil {
							c.log.Errorf("failed to switch namespace: %v", err)
							return
						}

						// VETH is ready to be created and configured
						err = c.addVethInterfacePair(nsMgmtCtx, iface.config, peer.config)
						if err != nil {
							c.log.Error(err.Error())
							continue
						}

						if err := c.configureLinuxInterface(nsMgmtCtx, iface.config); err != nil {
							c.log.Errorf("failed to configure VETH interface %s: %v", iface.config.Name, err)
						} else if err := c.configureLinuxInterface(nsMgmtCtx, peer.config); err != nil {
							c.log.Errorf("failed to configure VETH interface %s: %v", peer.config.Name, err)
						}
						revertNs()
					} else {
						c.log.Debugf("peer VETH %v is not ready yet, microservice: %+v", iface.config.Name, microservice)
					}
				}
			} else if msEvent.EventType == nsplugin.TerminatedMicroservice {
				for _, iface := range c.getIfsByMsLabel(microservice.Label) {
					c.removeObsoleteVeth(nsMgmtCtx, iface.config.Name, iface.config.HostIfName, iface.config.Namespace)
					if iface.peer != nil && iface.peer.config != nil {
						c.removeObsoleteVeth(nsMgmtCtx, iface.peer.config.Name, iface.peer.config.HostIfName, iface.peer.config.Namespace)
					} else {
						c.log.Warnf("Obsolete peer for %s not removed, no peer data", iface.config.Name)
					}
				}
			} else {
				c.log.Errorf("Unknown microservice event type: %s", msEvent.EventType)
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// If hostIfName is not set, symbolic name will be used.
func (c *LinuxInterfaceConfigurator) handleOptionalHostIfName(config *interfaces.LinuxInterfaces_Interface) {
	if config.HostIfName == "" {
		config.HostIfName = config.Name
	}
}

func addressExists(configured []netlink.Addr, provided *net.IPNet) bool {
	for _, confAddr := range configured {
		if bytes.Equal(confAddr.IP, provided.IP) {
			return true
		}
	}
	return false
}

// ResolveCreatedVPPInterface resolves a new vpp interfaces
func (c *LinuxInterfaceConfigurator) ResolveCreatedVPPInterface(ifConfigMetaData *vppIf.Interfaces_Interface) error {
	if ifConfigMetaData == nil {
		return errors.Errorf("unable to resolve registered VPP interface %s, no configuration data available",
			ifConfigMetaData.Name)
	}

	if ifConfigMetaData.Type != vppIf.InterfaceType_TAP_INTERFACE {
		return nil
	}

	var hostIfName string
	if ifConfigMetaData.Tap != nil {
		hostIfName = ifConfigMetaData.GetTap().GetHostIfName()
	}
	if hostIfName == "" {
		hostIfName = ifConfigMetaData.GetName()
	}
	if hostIfName == "" {
		return errors.Errorf("unable to resolve registered VPP interface %s, incomplete configuration data",
			ifConfigMetaData.Name)
	}

	var linuxIf *interfaces.LinuxInterfaces_Interface
	_, data, exists := c.ifCachedConfigs.LookupIdx(hostIfName)
	if exists && data != nil {
		linuxIf = data.Data
	}

	if err := c.configureTapInterface(hostIfName, linuxIf); err != nil {
		return errors.Errorf("failed to configure linux interface %s with registered VPP interface %s: %v",
			ifConfigMetaData.Name, linuxIf, err)
	}

	return nil
}

// ResolveDeletedVPPInterface resolves removed vpp interfaces
func (c *LinuxInterfaceConfigurator) ResolveDeletedVPPInterface(ifConfigMetaData *vppIf.Interfaces_Interface) error {
	if ifConfigMetaData == nil {
		return errors.Errorf("unable to resolve unregistered VPP interface %s, no configuration data available",
			ifConfigMetaData.Name)
	}

	if ifConfigMetaData.Type != vppIf.InterfaceType_TAP_INTERFACE {
		return nil
	}

	var hostIfName string
	if ifConfigMetaData.Tap != nil {
		hostIfName = ifConfigMetaData.GetTap().GetHostIfName()
	}
	if hostIfName == "" {
		hostIfName = ifConfigMetaData.GetName()
	}
	if hostIfName == "" {
		return errors.Errorf("unable to resolve unregistered VPP interface %s, incomplete configuration data",
			ifConfigMetaData.Name)
	}

	_, ifConfig, exists := c.ifIndexes.LookupIdx(ifConfigMetaData.Name)
	if exists {
		// Move linux tap configuration to cache
		c.ifCachedConfigs.RegisterName(hostIfName, c.pIfCachedConfigSeq, &ifaceidx.IndexedLinuxInterface{
			Index: c.pIfCachedConfigSeq,
			Data:  ifConfig.Data,
		})
		// Unregister TAP from the in-memory map
		c.ifIndexes.UnregisterName(ifConfig.Data.Name)
		c.pIfCachedConfigSeq++
		c.log.Infof("Linux tap %s configuration unregistered and moved to cache", ifConfig.Data.Name)
		if err := c.moveTapInterfaceToDefaultNamespace(ifConfig.Data); err != nil {
			return errors.Errorf("failed to resolve unregistered VPP interface %s: %v", ifConfigMetaData.Name, err)
		}
	}

	return nil
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *LinuxInterfaceConfigurator) LogError(err error) error {
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

// SetInterfaceNamespace moves a given Linux interface into a specified namespace.
func (c *LinuxInterfaceConfigurator) setInterfaceNamespace(ctx *nsplugin.NamespaceMgmtCtx, ifName string, namespace *interfaces.LinuxInterfaces_Interface_Namespace) error {
	// Convert microservice namespace
	var err error
	if namespace != nil && namespace.Type == interfaces.LinuxInterfaces_Interface_Namespace_MICROSERVICE_REF_NS {
		// Convert namespace
		ifNs := c.nsHandler.ConvertMicroserviceNsToPidNs(namespace.Microservice)
		// Back to interface ns type
		namespace, err = ifNs.GenericToIfaceNs()
		if err != nil {
			return errors.Errorf("failed to convert generic interface namespace: %v", err)
		}
		if namespace == nil {
			return errors.Errorf("microservice is not available for %s", ifName)
		}
	}

	ifaceNs := c.nsHandler.IfNsToGeneric(namespace)

	// Get network namespace file descriptor
	ns, err := c.nsHandler.GetOrCreateNamespace(ifaceNs)
	if err != nil {
		return errors.Errorf("faield to get or create namespace %s: %v", namespace.Name, err)
	}
	defer ns.Close()

	// Get the link plugin.
	link, err := c.ifHandler.GetLinkByName(ifName)
	if err != nil {
		return errors.Errorf("failed to get link for interface %s: %v", ifName, err)
	}

	// When interface moves from one namespace to another, it loses all its IP addresses, admin status
	// and MTU configuration -- we need to remember the interface configuration before the move
	// and re-configure the interface in the new namespace.
	addresses, isIPv6, err := c.getLinuxIfAddrs(link.Attrs().Name)
	if err != nil {
		return errors.Errorf("failed to get IP address list from interface %s: %v", link.Attrs().Name, err)
	}

	// Move the interface into the namespace.
	err = c.sysHandler.LinkSetNsFd(link, int(ns))
	if err != nil {
		return errors.Errorf("failed to set interface %s file descriptor: %v", link.Attrs().Name, err)
	}

	// Re-configure interface in its new namespace
	revertNs, err := c.nsHandler.SwitchNamespace(ifaceNs, ctx)
	if err != nil {
		return errors.Errorf("failed to switch namespace: %v", err)
	}
	defer revertNs()

	if link.Attrs().Flags&net.FlagUp == 1 {
		// Re-enable interface
		err = c.ifHandler.SetInterfaceUp(ifName)
		if nil != err {
			return errors.Errorf("failed to re-enable Linux interface `%s`: %v", ifName, err)
		}
	}

	// Re-add IP addresses
	for _, address := range addresses {
		// Skip IPv6 link local address if there is no other IPv6 address
		if !isIPv6 && address.IP.IsLinkLocalUnicast() {
			continue
		}
		err = c.ifHandler.AddInterfaceIP(ifName, address)
		if err != nil {
			if err.Error() == "file exists" {
				continue
			}
			return errors.Errorf("failed to re-assign IP address to a Linux interface `%s`: %v", ifName, err)
		}
	}

	// Revert back the MTU config
	err = c.ifHandler.SetInterfaceMTU(ifName, link.Attrs().MTU)
	if nil != err {
		return errors.Errorf("failed to re-assign MTU of a Linux interface `%s`: %v", ifName, err)
	}

	return nil
}

// getLinuxIfAddrs returns a list of IP addresses for given linux interface with info whether there is IPv6 address
// (except default link local)
func (c *LinuxInterfaceConfigurator) getLinuxIfAddrs(ifName string) ([]*net.IPNet, bool, error) {
	var networks []*net.IPNet
	addresses, err := c.ifHandler.GetAddressList(ifName)
	if err != nil {
		return nil, false, errors.Errorf("failed to get IP address set from linux interface %s", ifName)
	}
	var containsIPv6 bool
	for _, ipAddr := range addresses {
		network, ipv6, err := addrs.ParseIPWithPrefix(ipAddr.String())
		if err != nil {
			return nil, false, errors.Errorf("failed to parse IP address %s", ipAddr.String())
		}
		// Set once if IP address is version 6 and not a link local address
		if !containsIPv6 && ipv6 && !ipAddr.IP.IsLinkLocalUnicast() {
			containsIPv6 = true
		}
		networks = append(networks, network)
	}

	return networks, containsIPv6, nil
}
