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
	"net"
	"strings"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// ProxyArpConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of L3 proxy arp entries as modelled by the proto file "../model/l3/l3.proto" and stored
// in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1/proxyarp". Configuration uses separate keys
// for proxy arp range and interfaces. Updates received from the northbound API are compared with the VPP
// run-time configuration and differences are applied through the VPP binary API.
type ProxyArpConfigurator struct {
	log logging.Logger

	// Mappings
	ifIndexes ifaceidx.SwIfIndex
	// ProxyArpIndices is a list of proxy ARP interface entries which are successfully configured on the VPP
	pArpIfIndexes idxvpp.NameToIdxRW
	// ProxyArpRngIndices is a list of proxy ARP range entries which are successfully configured on the VPP
	pArpRngIndexes idxvpp.NameToIdxRW
	// Cached interfaces
	pArpIfCache  []string
	pArpIndexSeq uint32

	// VPP channel
	vppChan govppapi.Channel
	// VPP API channel
	pArpHandler vppcalls.ProxyArpVppAPI
}

// Init VPP channel and vppcalls handler
func (c *ProxyArpConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("l3-proxy-arp-conf")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.pArpIfIndexes = nametoidx.NewNameToIdx(c.log, "proxyarp_if_indices", nil)
	c.pArpRngIndexes = nametoidx.NewNameToIdx(c.log, "proxyarp_rng_indices", nil)
	c.pArpIndexSeq = 1

	// VPP channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// VPP API handler
	c.pArpHandler = vppcalls.NewProxyArpVppHandler(c.vppChan, c.ifIndexes, c.log)

	c.log.Info("Proxy ARP configurator initialized")

	return nil
}

// Close VPP channel
func (c *ProxyArpConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose Proxy ARP configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *ProxyArpConfigurator) clearMapping() {
	c.pArpIfIndexes.Clear()
	c.pArpRngIndexes.Clear()
	c.log.Debugf("Proxy ARP configurator mapping cleared")
}

// GetArpIfIndexes exposes list of proxy ARP interface indexes
func (c *ProxyArpConfigurator) GetArpIfIndexes() idxvpp.NameToIdxRW {
	return c.pArpIfIndexes
}

// GetArpRngIndexes exposes list of proxy ARP range indexes
func (c *ProxyArpConfigurator) GetArpRngIndexes() idxvpp.NameToIdxRW {
	return c.pArpRngIndexes
}

// GetArpIfCache exposes list of cached ARP interfaces
func (c *ProxyArpConfigurator) GetArpIfCache() []string {
	return c.pArpIfCache
}

// AddInterface implements proxy arp handler.
func (c *ProxyArpConfigurator) AddInterface(pArpIf *l3.ProxyArpInterfaces_InterfaceList) error {
	for _, proxyArpIf := range pArpIf.Interfaces {
		ifName := proxyArpIf.Name
		if ifName == "" {
			return errors.Errorf("proxy ARP %s interface name not set", pArpIf.Label)
		}
		// Check interface, cache if does not exist
		ifIdx, _, found := c.ifIndexes.LookupIdx(ifName)
		if !found {
			c.pArpIfCache = append(c.pArpIfCache, ifName)
			c.log.Debugf("Interface %s does not exist, moving to cache", ifName)
			continue
		}

		// Call VPP API to enable interface for proxy ARP
		if err := c.pArpHandler.EnableProxyArpInterface(ifIdx); err != nil {
			return errors.Errorf("enabling interface %s for proxy ARP failed: %v", ifName, err)
		}
	}
	// Register
	c.pArpIfIndexes.RegisterName(pArpIf.Label, c.pArpIndexSeq, nil)
	c.pArpIndexSeq++
	c.log.Debugf("Proxy ARP configuration %s registered", pArpIf.Label)

	c.log.Infof("Interfaces enabled for proxy ARP config %s", pArpIf.Label)

	return nil
}

// ModifyInterface does nothing
func (c *ProxyArpConfigurator) ModifyInterface(newPArpIf, oldPArpIf *l3.ProxyArpInterfaces_InterfaceList) error {
	toEnable, toDisable := calculateIfDiff(newPArpIf.Interfaces, oldPArpIf.Interfaces)
	// Disable obsolete interfaces
	for _, ifName := range toDisable {
		// Check cache
		for idx, cachedIf := range c.pArpIfCache {
			if cachedIf == ifName {
				c.pArpIfCache = append(c.pArpIfCache[:idx], c.pArpIfCache[idx+1:]...)
				c.log.Debugf("Proxy ARP interface %s removed from cache", ifName)
				continue
			}
		}
		ifIdx, _, found := c.ifIndexes.LookupIdx(ifName)
		// If interface is not found, there is nothing to do
		if found {
			if err := c.pArpHandler.DisableProxyArpInterface(ifIdx); err != nil {
				return errors.Errorf("failed to disable interface %s for proxy ARP: %v", ifName, err)
			}
		}
	}
	// Enable new interfaces
	for _, ifName := range toEnable {
		// Put to cache if interface is missing
		ifIdx, _, found := c.ifIndexes.LookupIdx(ifName)
		if !found {
			c.pArpIfCache = append(c.pArpIfCache, ifName)
			c.log.Debugf("Interface %s does not exist, moving to cache", ifName)
			continue
		}
		// Configure
		if err := c.pArpHandler.EnableProxyArpInterface(ifIdx); err != nil {
			return errors.Errorf("failed to enable interface %s for proxy ARP: %v", ifName, err)
		}
	}

	c.log.Infof("Modifying proxy ARP interface configuration %s", newPArpIf.Label)

	return nil
}

// DeleteInterface disables proxy ARP interface or removes it from cache
func (c *ProxyArpConfigurator) DeleteInterface(pArpIf *l3.ProxyArpInterfaces_InterfaceList) error {
ProxyArpIfLoop:
	for _, proxyArpIf := range pArpIf.Interfaces {
		ifName := proxyArpIf.Name
		// Check if interface is cached
		for idx, cachedIf := range c.pArpIfCache {
			if cachedIf == ifName {
				c.pArpIfCache = append(c.pArpIfCache[:idx], c.pArpIfCache[idx+1:]...)
				c.log.Debugf("Proxy ARP interface %s removed from cache", ifName)
				continue ProxyArpIfLoop
			}
		}
		// Look for interface
		ifIdx, _, found := c.ifIndexes.LookupIdx(ifName)
		if !found {
			// Interface does not exist, nothing more to do
			continue
		}
		// Call VPP API to disable interface for proxy ARP
		if err := c.pArpHandler.DisableProxyArpInterface(ifIdx); err != nil {
			return errors.Errorf("failed to enable interface %s for proxy ARP: %v", ifName, err)
		}
	}

	// Un-register
	c.pArpIfIndexes.UnregisterName(pArpIf.Label)
	c.log.Debugf("Proxy ARP interface config %s unregistered", pArpIf.Label)

	c.log.Infof("Disabling interfaces from proxy ARP config %s", pArpIf.Label)

	return nil
}

// AddRange configures new IP range for proxy ARP
func (c *ProxyArpConfigurator) AddRange(pArpRng *l3.ProxyArpRanges_RangeList) error {
	for _, proxyArpRange := range pArpRng.Ranges {
		// Prune addresses
		firstIP, err := c.pruneIP(proxyArpRange.FirstIp)
		if err != nil {
			return err
		}
		lastIP, err := c.pruneIP(proxyArpRange.LastIp)
		if err != nil {
			return err
		}
		// Convert to byte representation
		bFirstIP := net.ParseIP(firstIP).To4()
		bLastIP := net.ParseIP(lastIP).To4()
		// Call VPP API to configure IP range for proxy ARP
		if err := c.pArpHandler.AddProxyArpRange(bFirstIP, bLastIP); err != nil {
			return errors.Errorf("failed to configure proxy ARP address range %s - %s: %v", firstIP, lastIP, err)
		}
	}

	// Register
	c.pArpRngIndexes.RegisterName(pArpRng.Label, c.pArpIndexSeq, nil)
	c.pArpIndexSeq++
	c.log.Debugf("Proxy ARP range config %s registered", pArpRng.Label)

	c.log.Infof("Proxy ARP IP range config %s set", pArpRng.Label)

	return nil
}

// ModifyRange does nothing
func (c *ProxyArpConfigurator) ModifyRange(newPArpRng, oldPArpRng *l3.ProxyArpRanges_RangeList) error {
	toAdd, toDelete := calculateRngDiff(newPArpRng.Ranges, oldPArpRng.Ranges)
	// Remove old ranges
	for _, rng := range toDelete {
		// Prune
		firstIP, err := c.pruneIP(rng.FirstIp)
		if err != nil {
			return err
		}
		lastIP, err := c.pruneIP(rng.LastIp)
		if err != nil {
			return err
		}
		// Convert to byte representation
		bFirstIP := net.ParseIP(firstIP).To4()
		bLastIP := net.ParseIP(lastIP).To4()
		// Call VPP API to configure IP range for proxy ARP
		if err := c.pArpHandler.DeleteProxyArpRange(bFirstIP, bLastIP); err != nil {
			return errors.Errorf("failed to remove proxy ARP address range %s - %s: %v", firstIP, lastIP, err)
		}
	}
	// Add new ranges
	for _, rng := range toAdd {
		// Prune addresses
		firstIP, err := c.pruneIP(rng.FirstIp)
		if err != nil {
			return err
		}
		lastIP, err := c.pruneIP(rng.LastIp)
		if err != nil {
			return err
		}
		// Convert to byte representation
		bFirstIP := net.ParseIP(firstIP).To4()
		bLastIP := net.ParseIP(lastIP).To4()
		// Call VPP API to configure IP range for proxy ARP
		if err := c.pArpHandler.AddProxyArpRange(bFirstIP, bLastIP); err != nil {
			return errors.Errorf("failed to configure proxy ARP address range %s - %s: %v", firstIP, lastIP, err)
		}
	}

	c.log.Infof("Proxy ARP range config %s modified", oldPArpRng.Label)

	return nil
}

// DeleteRange implements proxy arp handler.
func (c *ProxyArpConfigurator) DeleteRange(pArpRng *l3.ProxyArpRanges_RangeList) error {
	for _, proxyArpRange := range pArpRng.Ranges {
		// Prune addresses
		firstIP, err := c.pruneIP(proxyArpRange.FirstIp)
		if err != nil {
			return err
		}
		lastIP, err := c.pruneIP(proxyArpRange.LastIp)
		if err != nil {
			return err
		}
		// Convert to byte representation
		bFirstIP := net.ParseIP(firstIP).To4()
		bLastIP := net.ParseIP(lastIP).To4()
		// Call VPP API to configure IP range for proxy ARP
		if err := c.pArpHandler.DeleteProxyArpRange(bFirstIP, bLastIP); err != nil {
			return errors.Errorf("failed to remove proxy ARP address range %s - %s: %v", firstIP, lastIP, err)
		}
	}

	// Un-register
	c.pArpRngIndexes.UnregisterName(pArpRng.Label)
	c.log.Debugf("Proxy ARP range config %s unregistered", pArpRng.Label)

	c.log.Infof("Proxy ARP IP range config %s removed", pArpRng.Label)

	return nil
}

// ResolveCreatedInterface handles new registered interface for proxy ARP
func (c *ProxyArpConfigurator) ResolveCreatedInterface(ifName string, ifIdx uint32) error {
	// Look for interface in cache
	for idx, cachedIf := range c.pArpIfCache {
		if cachedIf == ifName {
			// Configure cached interface
			if err := c.pArpHandler.EnableProxyArpInterface(ifIdx); err != nil {
				return errors.Errorf("failed to enable registered interface %s for proxy ARP: %v", ifName, err)
			}
			// Remove from cache
			c.pArpIfCache = append(c.pArpIfCache[:idx], c.pArpIfCache[idx+1:]...)
			c.log.Debugf("Registered interface %s configured for Proxy ARP and removed from cache", ifName)
			return nil
		}
	}

	return nil
}

// ResolveDeletedInterface handles new registered interface for proxy ARP
func (c *ProxyArpConfigurator) ResolveDeletedInterface(ifName string) error {
	// Check if interface was enabled for proxy ARP
	_, _, found := c.pArpIfIndexes.LookupIdx(ifName)
	if found {
		// Put interface to cache (no need to call delete)
		c.pArpIfCache = append(c.pArpIfCache, ifName)
		c.log.Debugf("Unregistered interface %s removed from proxy ARP cache", ifName)
	}

	return nil
}

// Remove IP mask if set
func (c *ProxyArpConfigurator) pruneIP(ip string) (string, error) {
	ipParts := strings.Split(ip, "/")
	if len(ipParts) == 1 {
		return ipParts[0], nil
	}
	if len(ipParts) == 2 {
		c.log.Warnf("Proxy ARP range: removing unnecessary mask from IP address %s", ip)
		return ipParts[0], nil
	}
	return ip, errors.Errorf("proxy ARP range: invalid IP address format: %s", ip)
}

// Calculate difference between old and new interfaces
func calculateIfDiff(newIfs, oldIfs []*l3.ProxyArpInterfaces_InterfaceList_Interface) (toEnable, toDisable []string) {
	// Find missing new interfaces
	for _, newIf := range newIfs {
		var found bool
		for _, oldIf := range oldIfs {
			if newIf.Name == oldIf.Name {
				found = true
			}
		}
		if !found {
			toEnable = append(toEnable, newIf.Name)
		}
	}
	// Find obsolete interfaces
	for _, oldIf := range oldIfs {
		var found bool
		for _, newIf := range newIfs {
			if oldIf.Name == newIf.Name {
				found = true
			}
		}
		if !found {
			toDisable = append(toDisable, oldIf.Name)
		}
	}
	return
}

// Calculate difference between old and new ranges
func calculateRngDiff(newRngs, oldRngs []*l3.ProxyArpRanges_RangeList_Range) (toAdd, toDelete []*l3.ProxyArpRanges_RangeList_Range) {
	// Find missing ranges
	for _, newRng := range newRngs {
		var found bool
		for _, oldRng := range oldRngs {
			if newRng.FirstIp == oldRng.FirstIp && newRng.LastIp == oldRng.LastIp {
				found = true
			}
		}
		if !found {
			toAdd = append(toAdd, newRng)
		}
	}
	// Find obsolete interfaces
	for _, oldRng := range oldRngs {
		var found bool
		for _, newRng := range newRngs {
			if oldRng.FirstIp == newRng.FirstIp && oldRng.LastIp == newRng.LastIp {
				found = true
			}
		}
		if !found {
			toDelete = append(toDelete, oldRng)
		}
	}
	return
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *ProxyArpConfigurator) LogError(err error) error {
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
