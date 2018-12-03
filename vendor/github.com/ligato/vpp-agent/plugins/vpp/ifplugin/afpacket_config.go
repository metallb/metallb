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
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
)

// AFPacketConfigurator is used by InterfaceConfigurator to execute afpacket-specific management operations.
// Most importantly it needs to ensure that Afpacket interface is create AFTER the associated host interface.
type AFPacketConfigurator struct {
	log       logging.Logger
	linux     interface{} // just flag if nil
	ifIndexes ifaceidx.SwIfIndexRW

	// Caches
	afPacketByHostIf    map[string]*AfPacketConfig // host interface name -> Af Packet interface configuration
	afPacketByName      map[string]*AfPacketConfig // af packet name -> Af Packet interface configuration
	linuxHostInterfaces map[string]struct{}        // a set of available host (Linux) interfaces

	ifHandler vppcalls.IfVppAPI // handler used by InterfaceConfigurator
}

// AfPacketConfig wraps the proto formatted configuration of an Afpacket interface together with a flag
// that tells if the interface is waiting for a host interface to get created.
type AfPacketConfig struct {
	config  *intf.Interfaces_Interface
	pending bool
}

// GetAfPacketStatusByName looks for cached interface by its name and returns its state and data
func (c *AFPacketConfigurator) GetAfPacketStatusByName(name string) (exists, pending bool, ifData *intf.Interfaces_Interface) {
	data, ok := c.afPacketByName[name]
	if data != nil {
		return ok, data.pending, data.config
	}
	return ok, pending, ifData
}

// GetAfPacketStatusByHost looks for cached interface by host interface and returns its state and data
func (c *AFPacketConfigurator) GetAfPacketStatusByHost(hostIf string) (exists, pending bool, ifData *intf.Interfaces_Interface) {
	data, ok := c.afPacketByHostIf[hostIf]
	if data != nil {
		return ok, data.pending, data.config
	}
	return ok, pending, ifData
}

// GetHostInterfacesEntry looks for cached host interface and returns true if exists
func (c *AFPacketConfigurator) GetHostInterfacesEntry(hostIf string) bool {
	_, ok := c.linuxHostInterfaces[hostIf]
	return ok
}

// Init members of AFPacketConfigurator.
func (c *AFPacketConfigurator) Init(logger logging.Logger, ifHandler vppcalls.IfVppAPI, linux interface{},
	indexes ifaceidx.SwIfIndexRW) (err error) {
	c.log = logger
	c.log.Infof("Initializing AF-Packet configurator")

	// VPP API handler
	c.ifHandler = ifHandler

	// Linux
	c.linux = linux

	// Mappings
	c.ifIndexes = indexes
	c.afPacketByHostIf = make(map[string]*AfPacketConfig)
	c.afPacketByName = make(map[string]*AfPacketConfig)
	c.linuxHostInterfaces = make(map[string]struct{})

	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *AFPacketConfigurator) clearMapping() {
	c.afPacketByHostIf = make(map[string]*AfPacketConfig)
	c.afPacketByName = make(map[string]*AfPacketConfig)
}

// ConfigureAfPacketInterface creates a new Afpacket interface or marks it as pending if the target host interface doesn't exist yet.
func (c *AFPacketConfigurator) ConfigureAfPacketInterface(afpacket *intf.Interfaces_Interface) (swIndex uint32, pending bool, err error) {
	if afpacket.Type != intf.InterfaceType_AF_PACKET_INTERFACE {
		return 0, false, errors.Errorf("expecting AfPacket-type interface, received %v", afpacket.Type)
	}

	if c.linux != nil {
		_, hostIfAvail := c.linuxHostInterfaces[afpacket.Afpacket.HostIfName]
		if !hostIfAvail {
			c.addToCache(afpacket, true)
			return 0, true, nil
		}
	}
	swIdx, err := c.ifHandler.AddAfPacketInterface(afpacket.Name, afpacket.PhysAddress, afpacket.Afpacket)
	if err != nil {
		c.addToCache(afpacket, true)
		return 0, true, err
	}
	c.addToCache(afpacket, false)
	// If the interface is not in pending state, register it
	c.ifIndexes.RegisterName(afpacket.Name, swIdx, afpacket)
	c.log.Debugf("Interface %s registered to mapping", afpacket.Name)

	return swIdx, false, nil
}

// ModifyAfPacketInterface updates the cache with afpacket configurations and tells InterfaceConfigurator if the interface
// needs to be recreated for the changes to be applied.
func (c *AFPacketConfigurator) ModifyAfPacketInterface(newConfig *intf.Interfaces_Interface,
	oldConfig *intf.Interfaces_Interface) (recreate bool, err error) {

	if oldConfig.Type != intf.InterfaceType_AF_PACKET_INTERFACE ||
		newConfig.Type != intf.InterfaceType_AF_PACKET_INTERFACE {
		return false, errors.Errorf("expecting AfPacket-type interface, received %v/%v",
			oldConfig.Type, newConfig.Type)
	}

	afpacket, found := c.afPacketByName[oldConfig.Name]
	if !found || afpacket.pending || (newConfig.Afpacket.HostIfName != oldConfig.Afpacket.HostIfName) {
		return true, nil
	}

	// rewrite cached configuration
	c.addToCache(newConfig, false)

	return false, nil
}

// DeleteAfPacketInterface removes Afpacket interface from VPP and from the cache.
func (c *AFPacketConfigurator) DeleteAfPacketInterface(afpacket *intf.Interfaces_Interface, ifIdx uint32) (err error) {
	if afpacket.Type != intf.InterfaceType_AF_PACKET_INTERFACE {
		return errors.Errorf("expecting AfPacket-type interface, received %v", afpacket.Type)
	}

	config, found := c.afPacketByName[afpacket.Name]
	if !found || !config.pending {
		err = c.ifHandler.DeleteAfPacketInterface(afpacket.Name, ifIdx, afpacket.GetAfpacket())
		// unregister interface to let other plugins know that it is removed from the vpp
		c.ifIndexes.UnregisterName(afpacket.Name)
		c.log.Debugf("Interface %s unregistered from mapping", afpacket.Name)
	}
	c.removeFromCache(afpacket)

	return err
}

// ResolveCreatedLinuxInterface reacts to a newly created Linux interface.
func (c *AFPacketConfigurator) ResolveCreatedLinuxInterface(ifName, hostIfName string, ifIdx uint32) (*intf.Interfaces_Interface, error) {
	if c.linux == nil {
		c.log.Debugf("Registered linux interface %s not resolved, linux plugin disabled", ifName)
		return nil, nil
	}
	c.linuxHostInterfaces[hostIfName] = struct{}{}
	c.log.Debugf("Linux interface %s registered as host", hostIfName)

	afpacket, found := c.afPacketByHostIf[hostIfName]
	if found {
		if !afpacket.pending {
			// this should not happen, log as warning
			c.log.Warn("Re-creating already configured AFPacket interface %s (host name: %s)", ifName, hostIfName)
			// remove the existing afpacket and let the interface configurator to re-create it
			if err := c.DeleteAfPacketInterface(afpacket.config, ifIdx); err != nil {
				return nil, err
			}
		}
		// afpacket is now free to get created
		return afpacket.config, nil
	}
	return nil, nil // nothing to configure
}

// ResolveDeletedLinuxInterface reacts to a removed Linux interface.
func (c *AFPacketConfigurator) ResolveDeletedLinuxInterface(ifName, hostIfName string, ifIdx uint32) error {
	if c.linux == nil {
		c.log.Debugf("Unregistered linux interface %s not resolved, linux plugin disabled", ifName)
		return nil
	}
	delete(c.linuxHostInterfaces, hostIfName)
	c.log.Debugf("Linux interface %s unregistered as host", hostIfName)

	afpacket, found := c.afPacketByHostIf[hostIfName]
	if found {
		// remove the interface and re-add as pending
		if err := c.DeleteAfPacketInterface(afpacket.config, ifIdx); err != nil {
			return errors.Errorf("failed to remove af_packet interface %s (host name: %s): %v",
				ifName, hostIfName, err)
		}
		if _, _, err := c.ConfigureAfPacketInterface(afpacket.config); err != nil {
			return errors.Errorf("failed to configure af_packet interface %s (host name: %s): %v",
				ifName, hostIfName, err)
		}
	}
	return nil
}

// IsPendingAfPacket returns true if the given config belongs to pending Afpacket interface.
func (c *AFPacketConfigurator) IsPendingAfPacket(iface *intf.Interfaces_Interface) (pending bool) {
	afpacket, found := c.afPacketByName[iface.Name]
	return found && afpacket.pending
}

func (c *AFPacketConfigurator) addToCache(afpacket *intf.Interfaces_Interface, pending bool) {
	config := &AfPacketConfig{config: afpacket, pending: pending}
	c.afPacketByHostIf[afpacket.Afpacket.HostIfName] = config
	c.afPacketByName[afpacket.Name] = config
	c.log.Debugf("Afpacket interface with name %v added to cache (hostIf: %s, pending: %t)",
		afpacket.Name, afpacket.Afpacket.HostIfName, pending)
}

func (c *AFPacketConfigurator) removeFromCache(afpacket *intf.Interfaces_Interface) {
	delete(c.afPacketByName, afpacket.Name)
	delete(c.afPacketByHostIf, afpacket.Afpacket.HostIfName)
	c.log.Debugf("Afpacket interface with name %v removed from cache", afpacket.Name)
}
