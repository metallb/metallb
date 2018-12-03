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
	"fmt"
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
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	modelStn "github.com/ligato/vpp-agent/plugins/vpp/model/stn"
)

// StnConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of interfaces as modelled by the proto file "../model/stn/stn.proto"
// and stored in ETCD under the key "vpp/config/v1/stn/rules/".
type StnConfigurator struct {
	log logging.Logger
	// Indexes
	ifIndexes        ifaceidx.SwIfIndex
	allIndexes       idxvpp.NameToIdxRW
	allIndexesSeq    uint32
	unstoredIndexes  idxvpp.NameToIdxRW
	unstoredIndexSeq uint32
	// VPP
	vppChan govppapi.Channel
	// VPP API handler
	stnHandler vppcalls.StnVppAPI
}

// IndexExistsFor returns true if there is and mapping entry for provided name
func (c *StnConfigurator) IndexExistsFor(name string) bool {
	_, _, found := c.allIndexes.LookupIdx(name)
	return found
}

// UnstoredIndexExistsFor returns true if there is and mapping entry for provided name
func (c *StnConfigurator) UnstoredIndexExistsFor(name string) bool {
	_, _, found := c.unstoredIndexes.LookupIdx(name)
	return found
}

// Init initializes STN configurator
func (c *StnConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, ifIndexes ifaceidx.SwIfIndex) (err error) {
	// Init logger
	c.log = logger.NewLogger("stn-conf")

	// Init VPP API channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// Init indexes
	c.ifIndexes = ifIndexes
	c.allIndexes = nametoidx.NewNameToIdx(c.log, "stn-all-indexes", nil)
	c.unstoredIndexes = nametoidx.NewNameToIdx(c.log, "stn-unstored-indexes", nil)
	c.allIndexesSeq, c.unstoredIndexSeq = 1, 1

	// VPP API handler
	c.stnHandler = vppcalls.NewStnVppHandler(c.vppChan, c.ifIndexes, c.log)

	c.log.Info("STN configurator initialized")

	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *StnConfigurator) clearMapping() {
	c.allIndexes.Clear()
	c.unstoredIndexes.Clear()
}

// ResolveDeletedInterface resolves when interface is deleted. If there exist a rule for this interface
// the rule will be deleted also.
func (c *StnConfigurator) ResolveDeletedInterface(interfaceName string) error {
	if rule := c.ruleFromIndex(interfaceName, true); rule != nil {
		if err := c.Delete(rule); err != nil {
			return err
		}
	}
	return nil
}

// ResolveCreatedInterface will check rules and if there is one waiting for interfaces it will be written
// into VPP.
func (c *StnConfigurator) ResolveCreatedInterface(interfaceName string) error {
	if rule := c.ruleFromIndex(interfaceName, false); rule != nil {
		if err := c.Add(rule); err == nil {
			c.unstoredIndexes.UnregisterName(StnIdentifier(interfaceName))
			c.log.Debugf("STN rule %s unregistered", rule.RuleName)
		} else {
			return err
		}
	}
	return nil
}

// Add create a new STN rule.
func (c *StnConfigurator) Add(rule *modelStn.STN_Rule) error {
	// Check stn data
	stnRule, doVPPCall, err := c.checkStn(rule, c.ifIndexes)
	if err != nil {
		return err
	}
	if !doVPPCall {
		c.log.Debugf("There is no interface for rule: %v. Waiting for interface.", rule.Interface)
		c.indexSTNRule(rule, true)
	} else {
		// Create and register new stn
		if err := c.stnHandler.AddStnRule(stnRule.IfaceIdx, &stnRule.IPAddress); err != nil {
			return errors.Errorf("failed to add STN rule %s: %v", rule.RuleName, err)
		}
		c.indexSTNRule(rule, false)

		c.log.Infof("STN rule %s configured", rule.RuleName)
	}

	return nil
}

// Delete removes STN rule.
func (c *StnConfigurator) Delete(rule *modelStn.STN_Rule) error {
	// Check stn data
	stnRule, _, err := c.checkStn(rule, c.ifIndexes)
	if err != nil {
		return err
	}

	if withoutIf, _ := c.removeRuleFromIndex(rule.Interface); withoutIf {
		c.log.Debug("STN rule was not stored into VPP, removed only from indexes.")
		return nil
	}

	// Remove rule
	if err := c.stnHandler.DelStnRule(stnRule.IfaceIdx, &stnRule.IPAddress); err != nil {
		return errors.Errorf("failed to delete STN rule %s: %v", rule.RuleName, err)
	}

	c.log.Infof("STN rule %s removed", rule.RuleName)

	return nil
}

// Modify configured rule.
func (c *StnConfigurator) Modify(ruleOld *modelStn.STN_Rule, ruleNew *modelStn.STN_Rule) error {
	if ruleOld == nil {
		return errors.Errorf("failed to modify STN rule, provided old value is nil")
	}

	if ruleNew == nil {
		return errors.Errorf("failed to modify STN rule, provided new value is nil")
	}

	if err := c.Delete(ruleOld); err != nil {
		return err
	}

	if err := c.Add(ruleNew); err != nil {
		return err
	}

	c.log.Infof("STN rule %s modified", ruleNew.RuleName)

	return nil
}

// Dump STN rules configured on the VPP
func (c *StnConfigurator) Dump() (*vppcalls.StnDetails, error) {
	stnDetails, err := c.stnHandler.DumpStnRules()
	if err != nil {
		return nil, errors.Errorf("failed to dump STN rules: %v", err)
	}
	return stnDetails, nil
}

// Close GOVPP channel.
func (c *StnConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose STN configurator: %v", err))
	}
	return nil
}

// checkStn will check the rule raw data and change it to internal data structure.
// In case the rule contains a interface that doesn't exist yet, rule is stored into index map.
func (c *StnConfigurator) checkStn(stnInput *modelStn.STN_Rule, index ifaceidx.SwIfIndex) (stnRule *vppcalls.StnRule, doVPPCall bool, err error) {
	c.log.Debugf("Checking stn rule: %+v", stnInput)

	if stnInput == nil {
		return nil, false, errors.Errorf("failed to add STN rule, input is empty")
	}
	if stnInput.Interface == "" {
		return nil, false, errors.Errorf("failed to add STN rule %s, no interface provided",
			stnInput.RuleName)
	}
	if stnInput.IpAddress == "" {
		return nil, false, errors.Errorf("failed to add STN rule %s, no IP address provided",
			stnInput.RuleName)
	}

	ipWithMask := strings.Split(stnInput.IpAddress, "/")
	if len(ipWithMask) > 1 {
		c.log.Debugf("STN rule %s IP address %s mask is ignored", stnInput.RuleName, stnInput.IpAddress)
		stnInput.IpAddress = ipWithMask[0]
	}
	parsedIP := net.ParseIP(stnInput.IpAddress)
	if parsedIP == nil {
		return nil, false, errors.Errorf("failed to create STN rule %s, unable to parse IP %s",
			stnInput.RuleName, stnInput.IpAddress)
	}

	ifName := stnInput.Interface
	ifIndex, _, exists := index.LookupIdx(ifName)
	if exists {
		doVPPCall = true
	}

	stnRule = &vppcalls.StnRule{
		IPAddress: parsedIP,
		IfaceIdx:  ifIndex,
	}

	return
}

func (c *StnConfigurator) indexSTNRule(rule *modelStn.STN_Rule, withoutIface bool) {
	idx := StnIdentifier(rule.Interface)
	if withoutIface {
		c.unstoredIndexes.RegisterName(idx, c.unstoredIndexSeq, rule)
		c.unstoredIndexSeq++
		c.log.Debugf("STN rule %s cached to unstored", rule.RuleName)
	}
	c.allIndexes.RegisterName(idx, c.allIndexesSeq, rule)
	c.allIndexesSeq++
	c.log.Debugf("STN rule %s registered to all", rule.RuleName)
}

func (c *StnConfigurator) removeRuleFromIndex(iface string) (withoutIface bool, rule *modelStn.STN_Rule) {
	idx := StnIdentifier(iface)

	// Removing rule from main index
	_, ruleIface, exists := c.allIndexes.LookupIdx(idx)
	if exists {
		c.allIndexes.UnregisterName(idx)
		c.log.Debugf("STN rule %d unregistered from all", idx)
		stnRule, ok := ruleIface.(*modelStn.STN_Rule)
		if ok {
			rule = stnRule
		}
	}

	// Removing rule from not stored rules index
	_, _, existsWithout := c.unstoredIndexes.LookupIdx(idx)
	if existsWithout {
		withoutIface = true
		c.unstoredIndexes.UnregisterName(idx)
		c.log.Debugf("STN rule %s unregistered from unstored", rule.RuleName)
	}

	return
}

func (c *StnConfigurator) ruleFromIndex(iface string, fromAllRules bool) (rule *modelStn.STN_Rule) {
	idx := StnIdentifier(iface)

	var ruleIface interface{}
	var exists bool

	if !fromAllRules {
		_, ruleIface, exists = c.unstoredIndexes.LookupIdx(idx)
	} else {
		_, ruleIface, exists = c.allIndexes.LookupIdx(idx)
	}
	if exists {
		stnRule, ok := ruleIface.(*modelStn.STN_Rule)
		if ok {
			rule = stnRule
		}
	}

	return
}

// StnIdentifier creates unique identifier which serves as a name in name to index mapping
func StnIdentifier(iface string) string {
	return fmt.Sprintf("stn-iface-%v", iface)
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *StnConfigurator) LogError(err error) error {
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
