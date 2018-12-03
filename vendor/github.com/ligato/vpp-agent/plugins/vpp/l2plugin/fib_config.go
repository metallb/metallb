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

package l2plugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// FIBConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of fib table entries as modelled by the proto file "../model/l2/l2.proto" and stored
// in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1/bd/<bd-label>/fib".
// Updates received from the northbound API are compared with the VPP run-time configuration
// and differences are applied through the VPP binary API.
type FIBConfigurator struct {
	log logging.Logger

	// In-memory mappings
	ifIndexes       ifaceidx.SwIfIndex
	bdIndexes       l2idx.BDIndex
	fibIndexes      l2idx.FIBIndexRW
	addCacheIndexes l2idx.FIBIndexRW // Serves as a cache for FIBs which cannot be configured immediately
	delCacheIndexes l2idx.FIBIndexRW // Serves as a cache for FIBs which cannot be removed immediately
	fibIndexSeq     uint32

	// VPP binary api call helper
	fibHandler vppcalls.FibVppAPI

	// VPP channels
	syncChannel  govppapi.Channel
	asyncChannel govppapi.Channel
}

// Init goroutines, mappings, channels..
func (c *FIBConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex,
	bdIndexes l2idx.BDIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("l2-fib-conf")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.bdIndexes = bdIndexes
	c.fibIndexes = l2idx.NewFIBIndex(nametoidx.NewNameToIdx(c.log, "fib_indexes", nil))
	c.addCacheIndexes = l2idx.NewFIBIndex(nametoidx.NewNameToIdx(c.log, "fib_add_indexes", nil))
	c.delCacheIndexes = l2idx.NewFIBIndex(nametoidx.NewNameToIdx(c.log, "fib_del_indexes", nil))
	c.fibIndexSeq = 1

	// VPP channels
	if c.syncChannel, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create sync API channel: %v", err)
	}
	if c.asyncChannel, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create async API channel: %v", err)
	}

	// VPP calls helper object
	c.fibHandler = vppcalls.NewFibVppHandler(c.syncChannel, c.asyncChannel, c.ifIndexes, c.bdIndexes, c.log)

	// FIB reply watcher
	go c.fibHandler.WatchFIBReplies()

	c.log.Info("L2 FIB configurator initialized")

	return nil
}

// Close vpp channel.
func (c *FIBConfigurator) Close() error {
	if err := safeclose.Close(c.syncChannel, c.asyncChannel); err != nil {
		c.LogError(errors.Errorf("failed to safeclose FIB configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *FIBConfigurator) clearMapping() {
	c.fibIndexes.Clear()
	c.addCacheIndexes.Clear()
	c.delCacheIndexes.Clear()
	c.log.Debugf("FIB configurator mapping cleared")
}

// GetFibIndexes returns FIB memory indexes
func (c *FIBConfigurator) GetFibIndexes() l2idx.FIBIndexRW {
	return c.fibIndexes
}

// GetFibAddCacheIndexes returns FIB memory 'add' cache indexes, for testing purpose
func (c *FIBConfigurator) GetFibAddCacheIndexes() l2idx.FIBIndexRW {
	return c.addCacheIndexes
}

// GetFibDelCacheIndexes returns FIB memory 'del' cache indexes, for testing purpose
func (c *FIBConfigurator) GetFibDelCacheIndexes() l2idx.FIBIndexRW {
	return c.delCacheIndexes
}

// Add configures provided FIB input. Every entry has to contain info about MAC address, interface, and bridge domain.
// If interface or bridge domain is missing or interface is not a part of the bridge domain, FIB data is cached
// and recalled if particular entity is registered/updated.
func (c *FIBConfigurator) Add(fib *l2.FibTable_FibEntry, callback func(error)) error {
	if fib.PhysAddress == "" {
		return errors.Errorf("failed to configure FIB entry, no MAC address defined")
	}
	if fib.BridgeDomain == "" {
		return errors.Errorf("failed to configure FIB entry (MAC %s), no bridge domain defined", fib.PhysAddress)
	}

	// Remove FIB from (del) cache if it's there
	_, _, exists := c.delCacheIndexes.UnregisterName(fib.PhysAddress)
	if exists {
		c.log.Debugf("FIB entry %s was removed from (del) cache before configuration", fib.PhysAddress)
	}

	// Validate required items and move to (add) cache if something's missing
	cached, ifIdx, bdIdx := c.validateFibRequirements(fib, true)
	if cached {
		return nil
	}

	if err := c.fibHandler.Add(fib.PhysAddress, bdIdx, ifIdx, fib.BridgedVirtualInterface, fib.StaticConfig,
		func(err error) {
			if err != nil {
				c.log.Errorf("FIB %s callback error: %v", fib.PhysAddress, err)
			} else {
				// Register
				c.fibIndexes.RegisterName(fib.PhysAddress, c.fibIndexSeq, fib)
				c.log.Debugf("Fib entry with MAC %s registered", fib.PhysAddress)
				c.fibIndexSeq++
			}
			callback(err)
		}); err != nil {
		return errors.Errorf("failed to add FIB entry with MAC %s: %v", fib.PhysAddress, err)
	}

	c.log.Infof("FIB table entry with MAC %s configured", fib.PhysAddress)

	return nil
}

// Modify provides changes for FIB entry. Old fib entry is removed (if possible) and a new one is registered
// if all the conditions are fulfilled (interface and bridge domain presence), otherwise new configuration is cached.
func (c *FIBConfigurator) Modify(oldFib *l2.FibTable_FibEntry,
	newFib *l2.FibTable_FibEntry, callback func(error)) error {
	// Remove FIB from (add) cache if present
	_, _, exists := c.addCacheIndexes.UnregisterName(oldFib.PhysAddress)
	if exists {
		c.log.Debugf("Modified FIB %s removed from (add) cache", oldFib.PhysAddress)
	}

	// Remove an old entry if possible
	oldIfIdx, _, ifFound := c.ifIndexes.LookupIdx(oldFib.OutgoingInterface)
	if !ifFound {
		c.log.Debugf("FIB %s cannot be removed now, interface %s no longer exists",
			oldFib.PhysAddress, oldFib.OutgoingInterface)
	} else {
		oldBdIdx, _, bdFound := c.bdIndexes.LookupIdx(oldFib.BridgeDomain)
		if !bdFound {
			c.log.Debugf("FIB %s cannot be removed, bridge domain %s no longer exists",
				oldFib.PhysAddress, oldFib.BridgeDomain)
		} else {
			if err := c.fibHandler.Delete(oldFib.PhysAddress, oldBdIdx, oldIfIdx, func(err error) {
				c.fibIndexes.UnregisterName(oldFib.PhysAddress)
				c.log.Debugf("FIB %s unregistered", oldFib.PhysAddress)
				callback(err)
			}); err != nil {
				return errors.Errorf("failed to delete FIB entry %s: %v", oldFib.PhysAddress, err)
			}
			c.addCacheIndexes.UnregisterName(oldFib.PhysAddress)
			c.log.Debugf("FIB %s unregistered from (add) cache", oldFib.PhysAddress)
		}
	}

	cached, ifIdx, bdIdx := c.validateFibRequirements(newFib, true)
	if cached {
		return nil
	}

	if err := c.fibHandler.Add(newFib.PhysAddress, bdIdx, ifIdx, newFib.BridgedVirtualInterface, newFib.StaticConfig,
		func(err error) {
			if err != nil {
				c.log.Errorf("FIB %s callback error: %v", newFib.PhysAddress, err)
			} else {
				// Register
				c.fibIndexes.RegisterName(newFib.PhysAddress, c.fibIndexSeq, newFib)
				c.log.Debugf("Fib entry with MAC %s registered", newFib.PhysAddress)
				c.fibIndexSeq++
			}
			callback(err)
		}); err != nil {
		return errors.Errorf("failed to create FIB entry %s: %v", oldFib.PhysAddress, err)
	}

	c.log.Infof("FIB table entry with MAC %s modified", newFib.PhysAddress)

	return nil
}

// Delete removes FIB table entry. The request to be successful, both interface and bridge domain indices
// have to be available. Request does nothing without this info. If interface (or bridge domain) was removed before,
// provided FIB data is just unregistered and agent assumes, that VPP removed FIB entry itself.
func (c *FIBConfigurator) Delete(fib *l2.FibTable_FibEntry, callback func(error)) error {
	// Check if FIB is in cache (add). In such a case, just remove it.
	_, _, exists := c.addCacheIndexes.UnregisterName(fib.PhysAddress)
	if exists {
		c.log.Infof("FIB %s does not exist, unregistered from (add) cache", fib.PhysAddress)
		return nil
	}

	// Check whether the FIB can be actually removed
	cached, ifIdx, bdIdx := c.validateFibRequirements(fib, false)
	if cached {
		return nil
	}

	// Unregister from (del) cache and from indexes
	c.delCacheIndexes.UnregisterName(fib.PhysAddress)
	c.log.Debugf("FIB %s unregistered from (del) cache", fib.PhysAddress)
	c.fibIndexes.UnregisterName(fib.PhysAddress)
	c.log.Debugf("FIB %s removed from mapping", fib.PhysAddress)

	if err := c.fibHandler.Delete(fib.PhysAddress, bdIdx, ifIdx, func(err error) {
		callback(err)
	}); err != nil {
		return errors.Errorf("failed to delete FIB entry %s: %v", fib.PhysAddress, err)
	}

	c.log.Infof("FIB table entry with MAC %s removed", fib.PhysAddress)

	return nil
}

// ResolveCreatedInterface uses FIB cache to additionally configure any FIB entries for this interface. Bridge domain
// is checked for existence. If resolution is successful, new FIB entry is configured, registered and removed from cache.
func (c *FIBConfigurator) ResolveCreatedInterface(ifName string, ifIdx uint32, callback func(error)) error {
	if err := c.resolveRegisteredItem(callback); err != nil {
		return err
	}
	return nil
}

// ResolveDeletedInterface handles removed interface. In that case, FIB entry remains on the VPP but it is not possible
// to delete it.
func (c *FIBConfigurator) ResolveDeletedInterface(ifName string, ifIdx uint32, callback func(error)) error {
	count := c.resolveUnRegisteredItem(ifName, "")

	c.log.Debugf("%d FIB entries belongs to removed interface %s. These FIBs cannot be deleted or changed while interface is missing",
		count, ifName)

	return nil
}

// ResolveCreatedBridgeDomain uses FIB cache to configure any FIB entries for this bridge domain.
// Required interface is checked for existence. If resolution is successful, new FIB entry is configured,
// registered and removed from cache.
func (c *FIBConfigurator) ResolveCreatedBridgeDomain(bdName string, bdID uint32, callback func(error)) error {
	if err := c.resolveRegisteredItem(callback); err != nil {
		return err
	}
	return nil
}

// ResolveUpdatedBridgeDomain handles case where metadata of bridge domain are updated. If interface-bridge domain pair
// required for a FIB entry was not bound together, but it was changed in the bridge domain later, FIB is resolved and
// eventually configred here.
func (c *FIBConfigurator) ResolveUpdatedBridgeDomain(bdName string, bdID uint32, callback func(error)) error {
	// Updated bridge domain is resolved the same as new (metadata were changed)
	if err := c.resolveRegisteredItem(callback); err != nil {
		return err
	}
	return nil
}

// ResolveDeletedBridgeDomain handles removed bridge domain. In that case, FIB entry remains on the VPP but it is not possible
// to delete it.
func (c *FIBConfigurator) ResolveDeletedBridgeDomain(bdName string, bdID uint32, callback func(error)) error {
	count := c.resolveUnRegisteredItem("", bdName)

	c.log.Debugf("%d FIB entries belongs to removed bridge domain %s. These FIBs cannot be deleted or changed while bridge domain is missing",
		count, bdName)

	return nil
}

// Common method called in either interface was created or bridge domain was created or updated. It tries to
// validate every 'add' or 'del' cached entry and configure/un-configure entries which are now possible
func (c *FIBConfigurator) resolveRegisteredItem(callback func(error)) error {
	// First, remove FIBs which cannot be removed due to missing interface
	for _, cachedFibID := range c.delCacheIndexes.GetMapping().ListNames() {
		_, fibData, found := c.delCacheIndexes.LookupIdx(cachedFibID)
		if !found || fibData == nil {
			// Should not happen
			continue
		}
		// Re-validate FIB, configure or keep cached
		cached, ifIdx, bdIdx := c.validateFibRequirements(fibData, false)
		if cached {
			continue
		}
		if err := c.fibHandler.Delete(cachedFibID, bdIdx, ifIdx, func(err error) {
			// Handle registration
			c.fibIndexes.UnregisterName(cachedFibID)
			c.log.Debugf("Obsolete FIB %s unregistered", cachedFibID)
			callback(err)
		}); err != nil {
			return errors.Errorf("failed to remove obsolete FIB %s: %v", cachedFibID, err)
		}
		c.delCacheIndexes.UnregisterName(cachedFibID)
		c.log.Debugf("FIB %s removed from (del) cache", cachedFibID)

		c.log.Infof("Cached FIB %s removed", cachedFibID)
	}

	// Configure un-configurable FIBs
	for _, cachedFibID := range c.addCacheIndexes.GetMapping().ListNames() {
		_, fibData, found := c.addCacheIndexes.LookupIdx(cachedFibID)
		if !found || fibData == nil {
			// Should not happen
			continue
		}
		// Re-validate FIB, configure or keep cached
		cached, ifIdx, bdIdx := c.validateFibRequirements(fibData, true)
		if cached {
			continue
		}
		if err := c.fibHandler.Add(cachedFibID, bdIdx, ifIdx, fibData.BridgedVirtualInterface, fibData.StaticConfig, func(err error) {
			if err != nil {
				c.log.Errorf("FIB %s callback error: %v", cachedFibID, err)
			} else {
				// Register
				c.fibIndexes.RegisterName(cachedFibID, c.fibIndexSeq, fibData)
				c.log.Debugf("Fib entry with MAC %s registered", cachedFibID)
				c.fibIndexSeq++
			}
			callback(err)
		}); err != nil {
			return errors.Errorf("failed to add FIB %s: %v", cachedFibID, err)
		}
		c.addCacheIndexes.UnregisterName(cachedFibID)
		c.log.Debugf("FIB %s removed from (add) cache", cachedFibID)

		c.log.Infof("Cached FIB %s added", cachedFibID)
	}

	return nil
}

// Just informative method which returns a count of entries affected by change
func (c *FIBConfigurator) resolveUnRegisteredItem(ifName, bdName string) int {
	var counter int
	for _, fib := range c.fibIndexes.GetMapping().ListNames() {
		_, meta, found := c.fibIndexes.LookupIdx(fib)
		if !found || meta == nil {
			// Should not happen
			continue
		}
		// Check interface if set
		if ifName != "" && ifName != meta.OutgoingInterface {
			continue
		}
		// Check bridge domain if set
		if bdName != "" && bdName != meta.BridgeDomain {
			continue
		}

		counter++
	}

	return counter
}

func (c *FIBConfigurator) validateFibRequirements(fib *l2.FibTable_FibEntry, add bool) (cached bool, ifIdx, bdIdx uint32) {
	var ifFound, bdFound, tied bool
	// Check interface presence
	ifIdx, _, ifFound = c.ifIndexes.LookupIdx(fib.OutgoingInterface)
	if !ifFound {
		c.log.Debugf("FIB entry %s is configured for interface %s which does not exists",
			fib.PhysAddress, fib.OutgoingInterface)
	}

	// Check bridge domain presence
	var bdMeta *l2idx.BdMetadata
	bdIdx, bdMeta, bdFound = c.bdIndexes.LookupIdx(fib.BridgeDomain)
	if !bdFound {
		c.log.Debugf("FIB entry %s is configured for bridge domain %s which does not exists",
			fib.PhysAddress, fib.BridgeDomain)
	}

	// Check that interface is tied with bridge domain. If interfaces are not found, metadata do not exists.
	// They can be updated later, configurator will handle it, but they should not be missing
	if bdMeta != nil {
		for _, configured := range bdMeta.ConfiguredInterfaces {
			if configured == fib.OutgoingInterface {
				tied = true
				break
			}
		}
	}

	// If either interface or bridge domain is missing, cache FIB entry
	if !bdFound || !ifFound || !tied {
		if add {
			// FIB table entry is cached and will be configured again when all required items are available
			_, _, found := c.addCacheIndexes.LookupIdx(fib.PhysAddress)
			if !found {
				c.addCacheIndexes.RegisterName(fib.PhysAddress, c.fibIndexSeq, fib)
				c.log.Debugf("FIB entry with name %s added to cache (add)", fib.PhysAddress)
				c.fibIndexSeq++
			} else {
				c.addCacheIndexes.UpdateMetadata(fib.PhysAddress, fib)
				c.log.Debugf("FIB entry %s metadata updated", fib.PhysAddress)
			}
		} else {
			// FIB table entry is cached and will be removed again when all required items are available
			_, _, found := c.delCacheIndexes.LookupIdx(fib.PhysAddress)
			if !found {
				c.delCacheIndexes.RegisterName(fib.PhysAddress, c.fibIndexSeq, fib)
				c.log.Debugf("FIB entry with name %s added to cache (del)", fib.PhysAddress)
				c.fibIndexSeq++
			} else {
				c.delCacheIndexes.UpdateMetadata(fib.PhysAddress, fib)
				c.log.Debugf("FIB entry %s metadata updated", fib.PhysAddress)
			}
		}
		cached = true
	}

	return
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *FIBConfigurator) LogError(err error) error {
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
