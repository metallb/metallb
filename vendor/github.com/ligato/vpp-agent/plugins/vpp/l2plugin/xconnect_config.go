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

// XConnectConfigurator implements PluginHandlerVPP.
type XConnectConfigurator struct {
	log logging.Logger
	// Interface indexes
	ifIndexes ifaceidx.SwIfIndex
	// Cross connect indexes
	xcIndexes         l2idx.XcIndexRW
	xcAddCacheIndexes l2idx.XcIndexRW
	xcDelCacheIndexes l2idx.XcIndexRW
	xcIndexSeq        uint32

	vppChan   govppapi.Channel
	xcHandler vppcalls.XConnectVppAPI
}

// Init essential configurator fields.
func (c *XConnectConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex) (err error) {
	// Logger
	c.log = logger.NewLogger("xc-conf")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.xcIndexes = l2idx.NewXcIndex(nametoidx.NewNameToIdx(c.log, "xc-indexes", nil))
	c.xcAddCacheIndexes = l2idx.NewXcIndex(nametoidx.NewNameToIdx(c.log, "xc-add-cache-indexes", nil))
	c.xcDelCacheIndexes = l2idx.NewXcIndex(nametoidx.NewNameToIdx(c.log, "xc-del-cache-indexes", nil))
	c.xcIndexSeq = 1

	// VPP channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		errors.Errorf("failed to create API channel: %v", err)
	}

	// Cross-connect VPP API handler
	c.xcHandler = vppcalls.NewXConnectVppHandler(c.vppChan, c.ifIndexes, c.log)

	c.log.Info("L2 XConnect configurator initialized")

	return nil
}

// Close govpp channel.
func (c *XConnectConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		c.LogError(errors.Errorf("failed to safeclose XConnect configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *XConnectConfigurator) clearMapping() {
	c.xcIndexes.Clear()
	c.xcAddCacheIndexes.Clear()
	c.xcDelCacheIndexes.Clear()
	c.log.Debugf("XConnect configurator mapping cleared")
}

// GetXcIndexes returns cross connect memory indexes
func (c *XConnectConfigurator) GetXcIndexes() l2idx.XcIndexRW {
	return c.xcIndexes
}

// GetXcAddCache returns cross connect 'add' cache (test purposes)
func (c *XConnectConfigurator) GetXcAddCache() l2idx.XcIndexRW {
	return c.xcAddCacheIndexes
}

// GetXcDelCache returns cross connect 'del' cache (test purposes)
func (c *XConnectConfigurator) GetXcDelCache() l2idx.XcIndexRW {
	return c.xcDelCacheIndexes
}

// ConfigureXConnectPair adds new cross connect pair
func (c *XConnectConfigurator) ConfigureXConnectPair(xc *l2.XConnectPairs_XConnectPair) error {
	if err := c.validateConfig(xc); err != nil {
		return errors.Errorf("failed to configure XConnect %s-%s, config is invalid: %v",
			xc.ReceiveInterface, xc.TransmitInterface, err)
	}
	// Verify interface presence, eventually store cross connect to cache if either is missing
	rxIfIdx, _, rxFound := c.ifIndexes.LookupIdx(xc.ReceiveInterface)
	txIfIdx, _, txFound := c.ifIndexes.LookupIdx(xc.TransmitInterface)
	if !rxFound || !txFound {
		c.putOrUpdateCache(xc, true)
		return nil
	}
	// XConnect can be configured now
	if err := c.xcHandler.AddL2XConnect(rxIfIdx, txIfIdx); err != nil {
		return errors.Errorf("failed to add L2 XConnect %v-%v: %v", rxIfIdx, txIfIdx, err)
	}
	// Unregister from 'del' cache in case it is present
	c.xcDelCacheIndexes.UnregisterName(xc.ReceiveInterface)
	c.log.Debugf("XConnect %s-%s removed from (del) cache", rxIfIdx, txIfIdx)
	// Register
	c.xcIndexes.RegisterName(xc.ReceiveInterface, c.xcIndexSeq, xc)
	c.xcIndexSeq++
	c.log.Debugf("XConnect %s-%s registered", rxIfIdx, txIfIdx)

	c.log.Infof("L2 XConnect pair %s-%s configured", xc.ReceiveInterface, xc.TransmitInterface)

	return nil
}

// ModifyXConnectPair modifies cross connect pair (its transmit interface). Old entry is replaced.
func (c *XConnectConfigurator) ModifyXConnectPair(newXc, oldXc *l2.XConnectPairs_XConnectPair) error {
	if err := c.validateConfig(newXc); err != nil {
		return err
	}
	// Verify receive interface presence
	rxIfIdx, _, rxFound := c.ifIndexes.LookupIdx(newXc.ReceiveInterface)
	if !rxFound {
		c.putOrUpdateCache(newXc, true)
		c.xcIndexes.UnregisterName(oldXc.ReceiveInterface)
		c.log.Debugf("XC Modify: Receive interface %s not found, unregistered.", newXc.ReceiveInterface)
		// Can return, without receive interface the entry cannot exist
		return nil
	}
	// Verify transmit interface
	txIfIdx, _, txFound := c.ifIndexes.LookupIdx(newXc.TransmitInterface)
	if !txFound {
		c.putOrUpdateCache(newXc, true)
		// If new transmit interface is missing and XConnect cannot be updated now, configurator can try to remove old
		// entry, so the VPP output won't be confusing
		oldTxIfIdx, _, oldTxFound := c.ifIndexes.LookupIdx(oldXc.TransmitInterface)
		if !oldTxFound {
			return nil // Nothing more can be done
		}
		c.log.Debugf("XC Modify: Removing obsolete l2 XConnect %s-%s", oldXc.ReceiveInterface, oldXc.TransmitInterface)
		if err := c.xcHandler.DeleteL2XConnect(rxIfIdx, oldTxIfIdx); err != nil {
			return errors.Errorf("failed to remove obsolete L2 XConnect %s-%s: %v",
				oldXc.ReceiveInterface, oldXc.TransmitInterface, err)
		}
		c.xcIndexes.UnregisterName(oldXc.ReceiveInterface)
		c.log.Debugf("XConnect %s-%s unregistered", rxIfIdx, txIfIdx)
		return nil
	}
	// Replace existing entry
	if err := c.xcHandler.AddL2XConnect(rxIfIdx, txIfIdx); err != nil {
		c.log.Errorf("Replacing l2 XConnect failed: %v", err)
		return err
	}
	c.xcIndexes.RegisterName(newXc.ReceiveInterface, c.xcIndexSeq, newXc)
	c.xcIndexSeq++
	c.log.Debugf("Modifying XConnect: new entry %s-%s added", newXc.ReceiveInterface, newXc.TransmitInterface)

	c.log.Infof("L2 XConnect pair (rx: %s) modified", newXc.ReceiveInterface)

	return nil
}

// DeleteXConnectPair removes XConnect if possible. Note: Xconnect pair cannot be removed if any interface is missing.
func (c *XConnectConfigurator) DeleteXConnectPair(xc *l2.XConnectPairs_XConnectPair) error {
	if err := c.validateConfig(xc); err != nil {
		return err
	}
	// If receive interface is missing, XConnect entry is not configured on the VPP.
	rxIfIdx, _, rxFound := c.ifIndexes.LookupIdx(xc.ReceiveInterface)
	if !rxFound {
		c.log.Debugf("XC Del: Receive interface %s not found.", xc.ReceiveInterface)
		// Remove from all caches
		c.xcIndexes.UnregisterName(xc.ReceiveInterface)
		c.xcAddCacheIndexes.UnregisterName(xc.ReceiveInterface)
		c.xcDelCacheIndexes.UnregisterName(xc.ReceiveInterface)
		c.log.Debugf("XC Del: %s unregistered from (add) cache, (del) cache and mapping.", xc.ReceiveInterface)
		return nil
	}
	// Verify transmit interface. If it is missing, XConnect cannot be removed and will be put to cache for deleted
	// interfaces
	txIfIdx, _, txFound := c.ifIndexes.LookupIdx(xc.TransmitInterface)
	if !txFound {
		c.log.Debugf("XC Del: Transmit interface %s for XConnect %s not found.",
			xc.TransmitInterface, xc.ReceiveInterface)
		c.putOrUpdateCache(xc, false)
		// Remove from other caches
		c.xcIndexes.UnregisterName(xc.ReceiveInterface)
		c.xcAddCacheIndexes.UnregisterName(xc.ReceiveInterface)
		c.log.Debugf("XC Del: %s unregistered from mapping and (add) cache", xc.ReceiveInterface)
		return nil
	}
	// XConnect can be removed now
	if err := c.xcHandler.DeleteL2XConnect(rxIfIdx, txIfIdx); err != nil {
		return errors.Errorf("failed to remove XConnect pair %v-%v: %v", rxIfIdx, txIfIdx, err)
	}
	// Unregister
	c.xcIndexes.UnregisterName(xc.ReceiveInterface)
	c.log.Debugf("XConnect pair %s-%s removed from mapping", rxIfIdx, txIfIdx)

	c.log.Infof("L2 XConnect pair %s-%s removed", xc.ReceiveInterface, xc.TransmitInterface)

	return nil
}

// ResolveCreatedInterface resolves XConnects waiting for an interface.
func (c *XConnectConfigurator) ResolveCreatedInterface(ifName string) error {
	// XConnects waiting to be configured
	for _, xcRxIf := range c.xcAddCacheIndexes.GetMapping().ListNames() {
		_, xc, _ := c.xcAddCacheIndexes.LookupIdx(xcRxIf)
		if xc == nil {
			return errors.Errorf("failed to process registered interface %s: XC entry %s has no metadata",
				ifName, xcRxIf)
		}
		if xc.TransmitInterface == ifName || xc.ReceiveInterface == ifName {
			if _, _, found := c.xcAddCacheIndexes.UnregisterName(xcRxIf); found {
				c.log.Debugf("XConnect %s unregistered from (add) cache", xcRxIf)
			}
			if err := c.ConfigureXConnectPair(xc); err != nil {
				return errors.Errorf("failed to add new XConnect %s with registered interface %s: %v",
					xc.ReceiveInterface, ifName, err)
			}
		}
	}
	// XConnects waiting for removal
	for _, xcRxIf := range c.xcDelCacheIndexes.GetMapping().ListNames() {
		_, xc, _ := c.xcDelCacheIndexes.LookupIdx(xcRxIf)
		if xc == nil {
			return errors.Errorf("failed to process registered interface %s: XC entry %s has no metadata",
				ifName, xcRxIf)
		}
		if xc.TransmitInterface == ifName || xc.ReceiveInterface == ifName {
			if _, _, found := c.xcDelCacheIndexes.UnregisterName(xcRxIf); found {
				c.log.Debugf("XConnect %s unregistered from (del) cache", xcRxIf)
			}
			if err := c.DeleteXConnectPair(xc); err != nil {
				return errors.Errorf("failed to delete XConnect %s with registered interface %s: %v",
					xc.ReceiveInterface, ifName, err)
			}
		}
	}

	return nil
}

// ResolveDeletedInterface resolves XConnects using deleted interface
// If deleted interface is a received interface, the XConnect was removed by the VPP
// If deleted interface is a transmit interface, it will get flag 'DELETED' in VPP, but the entry will be kept
func (c *XConnectConfigurator) ResolveDeletedInterface(ifName string) error {
	for _, xcRxIf := range c.xcIndexes.GetMapping().ListNames() {
		_, xc, _ := c.xcIndexes.LookupIdx(xcRxIf)
		if xc == nil {
			return errors.Errorf("failed to process unregistered interface %s: XC entry %s has no metadata",
				ifName, xcRxIf)
		}
		if xc.ReceiveInterface == ifName {
			if _, _, found := c.xcIndexes.UnregisterName(xc.ReceiveInterface); found {
				c.log.Debugf("XConnect %s unregistered from mapping", xcRxIf)
			}
			c.xcAddCacheIndexes.RegisterName(xc.ReceiveInterface, c.xcIndexSeq, xc)
			c.xcIndexSeq++
			c.log.Debugf("XConnect %s registered to (add) cache", xcRxIf)
			continue
		}
		// Nothing to do for transmit
	}

	return nil
}

// Add XConnect to 'add' or 'del' cache, or just update metadata
func (c *XConnectConfigurator) putOrUpdateCache(xc *l2.XConnectPairs_XConnectPair, cacheTypeAdd bool) {
	if cacheTypeAdd {
		if _, _, found := c.xcAddCacheIndexes.LookupIdx(xc.ReceiveInterface); found {
			c.xcAddCacheIndexes.UpdateMetadata(xc.ReceiveInterface, xc)
			c.log.Debugf("XConnect %s-%s cached (add) medatada updated", xc.ReceiveInterface, xc.TransmitInterface)
		} else {
			c.xcAddCacheIndexes.RegisterName(xc.ReceiveInterface, c.xcIndexSeq, xc)
			c.xcIndexSeq++
			c.log.Debugf("XConnect %s-%s registered to (add) cache", xc.ReceiveInterface, xc.TransmitInterface)
		}
	} else {
		if _, _, found := c.xcDelCacheIndexes.LookupIdx(xc.ReceiveInterface); found {
			c.xcDelCacheIndexes.UpdateMetadata(xc.ReceiveInterface, xc)
			c.log.Debugf("XConnect %s-%s cached (del) medatada updated", xc.ReceiveInterface, xc.TransmitInterface)
		} else {
			c.xcDelCacheIndexes.RegisterName(xc.ReceiveInterface, c.xcIndexSeq, xc)
			c.xcIndexSeq++
			c.log.Debugf("XConnect %s-%s registered to (del) cache", xc.ReceiveInterface, xc.TransmitInterface)
		}
	}
}

func (c *XConnectConfigurator) validateConfig(xc *l2.XConnectPairs_XConnectPair) error {
	if xc.ReceiveInterface == "" {
		return errors.Errorf("invalid XConnect configuration, receive interface is not set")
	}
	if xc.TransmitInterface == "" {
		return errors.Errorf("invalid XConnect configuration, transmit interface is not set")
	}
	if xc.ReceiveInterface == xc.TransmitInterface {
		return errors.Errorf("invalid XConnect configuration, recevie interface is the same as transmit (%s)",
			xc.ReceiveInterface)
	}
	return nil
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *XConnectConfigurator) LogError(err error) error {
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
