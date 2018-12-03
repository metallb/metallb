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

// Package ipsecplugin implements the IPSec plugin that handles management of IPSec for VPP.
package ipsecplugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	iface_vppcalls "github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/ipsecplugin/ipsecidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ipsecplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/ipsec"
)

// SPDIfCacheEntry contains info about cached assignment of interface to SPD
type SPDIfCacheEntry struct {
	spdID     uint32
	ifaceName string
}

// IPSecConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of interfaces as modelled by the proto file "../model/ipsec/ipsec.proto"
// and stored in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1/ipsec".
// Updates received from the northbound API are compared with the VPP run-time configuration and differences
// are applied through the VPP binary API.
type IPSecConfigurator struct {
	log logging.Logger

	// In-memory mappings
	ifIndexes        ifaceidx.SwIfIndexRW
	spdIndexes       ipsecidx.SPDIndexRW
	cachedSpdIndexes ipsecidx.SPDIndexRW
	spdIndexSeq      uint32
	saIndexes        idxvpp.NameToIdxRW
	saIndexSeq       uint32

	// SPC interface cache
	spdIfCache []SPDIfCacheEntry

	// VPP channel
	vppCh govppapi.Channel

	// VPP API handlers
	ifHandler    iface_vppcalls.IfVppAPI
	ipSecHandler vppcalls.IPSecVppAPI
}

// Init members (channels...) and start go routines
func (c *IPSecConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndexRW) (err error) {
	// Logger
	c.log = logger.NewLogger("ipsec-plugin")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.spdIndexes = ipsecidx.NewSPDIndex(nametoidx.NewNameToIdx(c.log, "ipsec_spd_indexes", nil))
	c.cachedSpdIndexes = ipsecidx.NewSPDIndex(nametoidx.NewNameToIdx(c.log, "ipsec_cached_spd_indexes", nil))
	c.saIndexes = nametoidx.NewNameToIdx(c.log, "ipsec_sa_indexes", ifaceidx.IndexMetadata)
	c.spdIndexSeq = 1
	c.saIndexSeq = 1

	// VPP channel
	if c.vppCh, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handlers
	c.ifHandler = iface_vppcalls.NewIfVppHandler(c.vppCh, c.log)
	c.ipSecHandler = vppcalls.NewIPsecVppHandler(c.vppCh, c.ifIndexes, c.spdIndexes, c.log)

	c.log.Debug("IPSec configurator initialized")

	return nil
}

// Close GOVPP channel
func (c *IPSecConfigurator) Close() error {
	if err := safeclose.Close(c.vppCh); err != nil {
		c.LogError(errors.Errorf("failed to safeclose IPSec configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *IPSecConfigurator) clearMapping() {
	c.spdIndexes.Clear()
	c.cachedSpdIndexes.Clear()
	c.saIndexes.Clear()

	c.log.Debugf("IPSec configurator mapping cleared")
}

// GetSaIndexes returns security association indexes
func (c *IPSecConfigurator) GetSaIndexes() idxvpp.NameToIdxRW {
	return c.saIndexes
}

// GetSpdIndexes returns security policy database indexes
func (c *IPSecConfigurator) GetSpdIndexes() ipsecidx.SPDIndex {
	return c.spdIndexes
}

// ConfigureSPD configures Security Policy Database in VPP
func (c *IPSecConfigurator) ConfigureSPD(spd *ipsec.SecurityPolicyDatabases_SPD) error {
	spdID := c.spdIndexSeq
	c.spdIndexSeq++

	for _, entry := range spd.PolicyEntries {
		if entry.Sa != "" {
			if _, _, exists := c.saIndexes.LookupIdx(entry.Sa); !exists {
				c.cachedSpdIndexes.RegisterName(spd.Name, spdID, spd)
				c.log.Debugf("SA %q for SPD %q not found, SPD configuration cached", entry.Sa, spd.Name)
				return nil
			}
		}
	}

	if err := c.configureSPD(spdID, spd); err != nil {
		return err
	}

	c.log.Debugf("SPD %s configured", spd.Name)

	return nil
}

func (c *IPSecConfigurator) configureSPD(spdID uint32, spd *ipsec.SecurityPolicyDatabases_SPD) error {
	if err := c.ipSecHandler.AddSPD(spdID); err != nil {
		return errors.Errorf("failed to add SPD with ID %d: %v", spdID, err)
	}

	c.spdIndexes.RegisterName(spd.Name, spdID, spd)
	c.log.Debugf("Registered SPD %s (%d)", spd.Name, spdID)

	for _, iface := range spd.Interfaces {
		swIfIdx, _, exists := c.ifIndexes.LookupIdx(iface.Name)
		if !exists {
			c.cacheSPDInterfaceAssignment(spdID, iface.Name)
			c.log.Debugf("Interface %q for SPD %q not found, assignment of interface to SPD cached", iface.Name, spd.Name)
			continue
		}

		if err := c.ipSecHandler.InterfaceAddSPD(spdID, swIfIdx); err != nil {
			return errors.Errorf("failed to assign interface %d to SPD %d: %v", swIfIdx, spdID, err)
		}
	}

	for _, entry := range spd.PolicyEntries {
		var saID uint32
		if entry.Sa != "" {
			var exists bool
			if saID, _, exists = c.saIndexes.LookupIdx(entry.Sa); !exists {
				c.log.Debugf("SA %q for SPD %q not found, skipping SPD policy entry configuration", entry.Sa, spd.Name)
				continue
			}
		}

		if err := c.ipSecHandler.AddSPDEntry(spdID, saID, entry); err != nil {
			return errors.Errorf("failed to add SPD %d entry: %v", saID, err)
		}
	}

	c.log.Infof("Configured SPD %v", spd.Name)

	return nil
}

// ModifySPD modifies Security Policy Database in VPP
func (c *IPSecConfigurator) ModifySPD(oldSpd, newSpd *ipsec.SecurityPolicyDatabases_SPD) error {
	if err := c.DeleteSPD(oldSpd); err != nil {
		return errors.Errorf("failed to modify SPD %v, error while removing old entry: %v", oldSpd.Name, err)
	}
	if err := c.ConfigureSPD(newSpd); err != nil {
		return errors.Errorf("failed to modify SPD %v, error while adding new entry: %v", oldSpd.Name, err)
	}

	c.log.Debugf("SPD %s modified", oldSpd.Name)

	return nil
}

// DeleteSPD deletes Security Policy Database in VPP
func (c *IPSecConfigurator) DeleteSPD(oldSpd *ipsec.SecurityPolicyDatabases_SPD) error {
	if spdID, _, found := c.cachedSpdIndexes.LookupIdx(oldSpd.Name); found {
		c.cachedSpdIndexes.UnregisterName(oldSpd.Name)
		c.log.Debugf("Cached SPD %d removed", spdID)
		return nil
	}

	spdID, _, exists := c.spdIndexes.LookupIdx(oldSpd.Name)
	if !exists {
		return errors.Errorf("cannot remove SPD %s, entry not found in the mapping", oldSpd.Name)
	}
	if err := c.ipSecHandler.DelSPD(spdID); err != nil {
		return errors.Errorf("failed to remove SPD %d: %v", spdID, err)
	}

	// remove cache entries related to the SPD
	for i, entry := range c.spdIfCache {
		if entry.spdID == spdID {
			c.spdIfCache = append(c.spdIfCache[:i], c.spdIfCache[i+1:]...)
			c.log.Debugf("Removed cache entry for assignment of SPD %q to interface %q", entry.spdID, entry.ifaceName)
		}
	}

	c.spdIndexes.UnregisterName(oldSpd.Name)
	c.log.Debugf("SPD %s unregistered", oldSpd.Name)

	c.log.Infof("Deleted SPD %v", oldSpd.Name)

	return nil
}

// ConfigureSA configures Security Association in VPP
func (c *IPSecConfigurator) ConfigureSA(sa *ipsec.SecurityAssociations_SA) error {
	saID := c.saIndexSeq
	c.saIndexSeq++

	if err := c.ipSecHandler.AddSAEntry(saID, sa); err != nil {
		return errors.Errorf("failed to add sA %d: %v", saID, err)
	}

	c.saIndexes.RegisterName(sa.Name, saID, nil)
	c.log.Debugf("Registered SA %v (%d)", sa.Name, saID)

	for _, cached := range c.cachedSpdIndexes.LookupBySA(sa.Name) {
		for _, entry := range cached.SPD.PolicyEntries {
			if entry.Sa != "" {
				if _, _, exists := c.saIndexes.LookupIdx(entry.Sa); !exists {
					c.log.Debugf("SA %q for SPD %q not found, keeping SPD in cache", entry.Sa, cached.SPD.Name)
					return nil
				}
			}
		}
		if err := c.configureSPD(cached.SpdID, cached.SPD); err != nil {
			return errors.Errorf("failed to configure SPD %s", cached.SPD.Name)
		}
		c.cachedSpdIndexes.UnregisterName(cached.SPD.Name)
		c.log.Debugf("SPD %s unregistered from cache", cached.SPD.Name)
	}

	c.log.Infof("SA %v configured", sa.Name)

	return nil
}

// ModifySA modifies Security Association in VPP
func (c *IPSecConfigurator) ModifySA(oldSa *ipsec.SecurityAssociations_SA, newSa *ipsec.SecurityAssociations_SA) error {
	// TODO: check if only keys change and use IpsecSaSetKey vpp call

	if err := c.DeleteSA(oldSa); err != nil {
		return errors.Errorf("failed to delete SA %s: %v", oldSa.Name, err)
	}
	if err := c.ConfigureSA(newSa); err != nil {
		return errors.Errorf("failed to configure SA %s: %v", newSa.Name, err)
	}

	c.log.Debugf("SA %s modified", oldSa.Name)

	return nil
}

// DeleteSA deletes Security Association in VPP
func (c *IPSecConfigurator) DeleteSA(oldSa *ipsec.SecurityAssociations_SA) error {
	saID, _, exists := c.saIndexes.LookupIdx(oldSa.Name)
	if !exists {
		return errors.Errorf("cannot delete SA %s, not found in the mapping", oldSa.Name)
	}

	for _, entry := range c.spdIndexes.LookupBySA(oldSa.Name) {
		if err := c.DeleteSPD(entry.SPD); err != nil {
			return errors.Errorf("attempt to remove SPD %v in order to cache it failed: %v", entry.SPD.Name, err)
		}
		c.cachedSpdIndexes.RegisterName(entry.SPD.Name, entry.SpdID, entry.SPD)
		c.log.Debugf("caching SPD %s due removed SA %s", entry.SPD.Name, oldSa.Name)
	}

	if err := c.ipSecHandler.DelSAEntry(saID, oldSa); err != nil {
		return errors.Errorf("failed to remove SA %d: %v", saID, err)
	}

	c.saIndexes.UnregisterName(oldSa.Name)
	c.log.Debugf("SA %s unregistered", oldSa.Name)

	c.log.Infof("SA %s deleted", oldSa.Name)

	return nil
}

// ConfigureTunnel configures Tunnel interface in VPP
func (c *IPSecConfigurator) ConfigureTunnel(tunnel *ipsec.TunnelInterfaces_Tunnel) error {
	ifIdx, err := c.ipSecHandler.AddTunnelInterface(tunnel)
	if err != nil {
		return errors.Errorf("failed to add IPSec tunnel interface %s: %v", tunnel.Name, err)
	}

	// Register with necessary metadata info
	c.ifIndexes.RegisterName(tunnel.Name, ifIdx, &interfaces.Interfaces_Interface{
		Name:        tunnel.Name,
		Enabled:     tunnel.Enabled,
		IpAddresses: tunnel.IpAddresses,
		Vrf:         tunnel.Vrf,
	})
	c.log.Debugf("Registered tunnel %s (%d)", tunnel.Name, ifIdx)

	if err := c.ifHandler.SetInterfaceVrf(ifIdx, tunnel.Vrf); err != nil {
		return errors.Errorf("failed to set VRF %d to tunnel interface %s: %v", tunnel.Vrf, tunnel.Name, err)
	}

	ipAddrs, err := addrs.StrAddrsToStruct(tunnel.IpAddresses)
	if err != nil {
		return errors.Errorf("failed to parse IP Addresses %s: %v", tunnel.IpAddresses, err)
	}
	for _, ip := range ipAddrs {
		if err := c.ifHandler.AddInterfaceIP(ifIdx, ip); err != nil {
			return errors.Errorf("failed to add IP addresses %s to tunnel interface %s: %v",
				tunnel.IpAddresses, tunnel.Name, err)
		}
	}

	if tunnel.Enabled {
		if err := c.ifHandler.InterfaceAdminUp(ifIdx); err != nil {
			return errors.Errorf("failed to set interface %s admin status up: %v",
				tunnel.Name, err)
		}
	}

	c.log.Infof("IPSec %s tunnel configured", tunnel.Name)

	return nil
}

// ModifyTunnel modifies Tunnel interface in VPP
func (c *IPSecConfigurator) ModifyTunnel(oldTunnel, newTunnel *ipsec.TunnelInterfaces_Tunnel) error {
	if err := c.DeleteTunnel(oldTunnel); err != nil {
		return errors.Errorf("failed to remove modified tunnel interface %s: %v", oldTunnel.Name, err)
	}
	if err := c.ConfigureTunnel(newTunnel); err != nil {
		return errors.Errorf("failed to configure modified tunnel interface %s: %v", oldTunnel.Name, err)
	}

	c.log.Infof("IPSec Tunnel %s modified", oldTunnel.Name)

	return nil
}

// DeleteTunnel deletes Tunnel interface in VPP
func (c *IPSecConfigurator) DeleteTunnel(oldTunnel *ipsec.TunnelInterfaces_Tunnel) error {
	ifIdx, _, exists := c.ifIndexes.LookupIdx(oldTunnel.Name)
	if !exists {
		return errors.Errorf("cannot delete IPSec tunnel interface %s, not found in the mapping", oldTunnel.Name)
	}

	if err := c.ipSecHandler.DelTunnelInterface(ifIdx, oldTunnel); err != nil {
		return errors.Errorf("failed to delete tunnel interface %s: %v", oldTunnel.Name, err)
	}

	c.ifIndexes.UnregisterName(oldTunnel.Name)
	c.log.Debugf("tunnel interface %s unregistered", oldTunnel.Name)

	c.log.Infof("Tunnel %s removed", oldTunnel.Name)

	return nil
}

// ResolveCreatedInterface is responsible for reconfiguring cached assignments
func (c *IPSecConfigurator) ResolveCreatedInterface(ifName string, swIfIdx uint32) error {
	for i, entry := range c.spdIfCache {
		if entry.ifaceName == ifName {
			// TODO: loop through stored deletes, this is now needed because old assignment might still exist
			if err := c.ipSecHandler.InterfaceDelSPD(entry.spdID, swIfIdx); err != nil {
				// Do not return error here
				c.log.Errorf("un-assigning interface from SPD failed: %v", err)
			}

			if err := c.ipSecHandler.InterfaceAddSPD(entry.spdID, swIfIdx); err != nil {
				return errors.Errorf("failed to assign interface %s to SPD %d: %v", ifName, entry.spdID, err)
			}

			c.log.Debugf("interface %s assigned to the SPD %d", ifName, entry.spdID)
			c.spdIfCache = append(c.spdIfCache[:i], c.spdIfCache[i+1:]...)
			c.log.Debugf("interface %s removed from SPD cache", ifName)
		}
	}

	return nil
}

// ResolveDeletedInterface is responsible for caching assignments for future reconfiguration
func (c *IPSecConfigurator) ResolveDeletedInterface(ifName string, swIfIdx uint32) error {
	for _, assign := range c.spdIndexes.LookupByInterface(ifName) {
		// TODO: just store this for future, because this will fail since swIfIdx no longer exists
		if err := c.ipSecHandler.InterfaceDelSPD(assign.SpdID, swIfIdx); err != nil {
			// Do not return error here
			c.log.Errorf("un-assigning interface from SPD failed: %v", err)
		}

		c.cacheSPDInterfaceAssignment(assign.SpdID, ifName)
	}

	return nil
}

func (c *IPSecConfigurator) cacheSPDInterfaceAssignment(spdID uint32, ifaceName string) {
	c.log.Debugf("caching SPD %v interface assignment to %v", spdID, ifaceName)
	c.spdIfCache = append(c.spdIfCache, SPDIfCacheEntry{
		ifaceName: ifaceName,
		spdID:     spdID,
	})
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *IPSecConfigurator) LogError(err error) error {
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
