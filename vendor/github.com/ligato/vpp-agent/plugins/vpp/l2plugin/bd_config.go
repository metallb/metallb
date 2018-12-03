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

// Package l2plugin implements the L2 plugin that handles Bridge Domains and L2 FIBs.
package l2plugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	ifvppcalls "github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// BDConfigurator runs in the background in its own goroutine where it watches for any changes
// in the configuration of bridge domains as modelled by the proto file "../model/l2/l2.proto" and stored
// in ETCD under the key "/vnf-agent/{vnf-agent}/vpp/config/v1bd". Updates received from the northbound API
// are compared with the VPP run-time configuration and differences are applied through the VPP binary API.
type BDConfigurator struct {
	log logging.Logger

	// In-memory mappings
	ifIndexes    ifaceidx.SwIfIndex
	bdIndexes    l2idx.BDIndexRW
	bdIDSeq      uint32
	regIfCounter uint32

	// VPP channel
	vppChan govppapi.Channel

	// VPP API handlers
	bdHandler vppcalls.BridgeDomainVppAPI

	// State notification channel
	notificationChan chan BridgeDomainStateMessage // Injected, do not close here

	// VPP API handlers
	ifHandler ifvppcalls.IfVppAPI
}

// BridgeDomainStateMessage is message with bridge domain state + bridge domain name (since a state message does not
// contain it). This state is sent to the bd_state.go to further processing after every change.
type BridgeDomainStateMessage struct {
	Message govppapi.Message
	Name    string
}

// GetBdIndexes exposes interface name-to-index mapping
func (c *BDConfigurator) GetBdIndexes() l2idx.BDIndexRW {
	return c.bdIndexes
}

// Init members (channels...) and start go routines.
func (c *BDConfigurator) Init(logger logging.PluginLogger, goVppMux govppmux.API, swIfIndexes ifaceidx.SwIfIndex,
	notificationChannel chan BridgeDomainStateMessage) (err error) {
	// Logger
	c.log = logger.NewLogger("l2-bd-conf")

	// Mappings
	c.ifIndexes = swIfIndexes
	c.bdIndexes = l2idx.NewBDIndex(nametoidx.NewNameToIdx(c.log, "bd_indexes", l2idx.IndexMetadata))
	c.bdIDSeq = 1
	c.regIfCounter = 1

	// VPP channel
	if c.vppChan, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// Init notification channel.
	c.notificationChan = notificationChannel

	// VPP API handlers
	c.ifHandler = ifvppcalls.NewIfVppHandler(c.vppChan, c.log)
	c.bdHandler = vppcalls.NewBridgeDomainVppHandler(c.vppChan, c.ifIndexes, c.log)

	c.log.Debug("L2 Bridge domains configurator initialized")

	return nil
}

// Close GOVPP channel.
func (c *BDConfigurator) Close() error {
	if err := safeclose.Close(c.vppChan); err != nil {
		return c.LogError(errors.Errorf("failed to safeclose L2 bridge domain configurator: %v", err))
	}
	return nil
}

// clearMapping prepares all in-memory-mappings and other cache fields. All previous cached entries are removed.
func (c *BDConfigurator) clearMapping() {
	c.bdIndexes.Clear()
}

// ConfigureBridgeDomain handles the creation of new bridge domain including interfaces, ARP termination
// entries and pushes status update notification.
func (c *BDConfigurator) ConfigureBridgeDomain(bdConfig *l2.BridgeDomains_BridgeDomain) error {
	isValid, _ := c.vppValidateBridgeDomainBVI(bdConfig, nil)
	if !isValid {
		return errors.Errorf("New bridge domain %s configuration is invalid", bdConfig.Name)
	}

	// Set index of the new bridge domain and increment global index.
	bdIdx := c.bdIDSeq
	c.bdIDSeq++

	// Create bridge domain with respective index.
	if err := c.bdHandler.VppAddBridgeDomain(bdIdx, bdConfig); err != nil {
		return errors.Errorf("failed to configure bridge domain %s: %v", bdConfig.Name, err)
	}

	// Find all interfaces belonging to this bridge domain and set them up.
	configuredIfs, err := c.bdHandler.SetInterfacesToBridgeDomain(bdConfig.Name, bdIdx, bdConfig.Interfaces, c.ifIndexes)
	if err != nil {
		return errors.Errorf("failed to set interfaces to bridge domain %s: %v", bdConfig.Name, err)
	}

	// Resolve ARP termination table entries.
	arpTerminationTable := bdConfig.GetArpTerminationTable()
	if arpTerminationTable != nil && len(arpTerminationTable) != 0 {
		arpTable := bdConfig.ArpTerminationTable
		for _, arpEntry := range arpTable {
			if err := c.bdHandler.VppAddArpTerminationTableEntry(bdIdx, arpEntry.PhysAddress, arpEntry.IpAddress); err != nil {
				return errors.Errorf("failed to add ARP termination table entry (MAC %v) to bridge domain %s: %v",
					arpEntry.PhysAddress, bdConfig.Name, err)
			}
		}
	}

	// Register created bridge domain.
	c.bdIndexes.RegisterName(bdConfig.Name, bdIdx, l2idx.NewBDMetadata(bdConfig, configuredIfs))
	c.log.Debugf("Bridge domain %s registered", bdConfig.Name)

	// Push to bridge domain state.
	if errLookup := c.propagateBdDetailsToStatus(bdIdx, bdConfig.Name); errLookup != nil {
		return errLookup
	}

	c.log.Infof("Bridge domain %s configured (ID: %d)", bdConfig.Name, bdIdx)

	return nil
}

// ModifyBridgeDomain processes the NB config and propagates it to bin api calls.
func (c *BDConfigurator) ModifyBridgeDomain(newBdConfig, oldBdConfig *l2.BridgeDomains_BridgeDomain) error {
	// Validate updated config.
	isValid, recreate := c.vppValidateBridgeDomainBVI(newBdConfig, oldBdConfig)
	if !isValid {
		return errors.Errorf("Modified bridge domain %s configuration is invalid", newBdConfig.Name)
	}

	// In case bridge domain params changed, it needs to be recreated
	if recreate {
		if err := c.DeleteBridgeDomain(oldBdConfig); err != nil {
			return errors.Errorf("bridge domain %s recreate error: %v", oldBdConfig.Name, err)
		}
		if err := c.ConfigureBridgeDomain(newBdConfig); err != nil {
			return errors.Errorf("bridge domain %s recreate error: %v", newBdConfig.Name, err)
		}
		c.log.Infof("Bridge domain %v modify done.", newBdConfig.Name)

		return nil
	}

	// Modify without recreation
	bdIdx, bdMeta, found := c.bdIndexes.LookupIdx(oldBdConfig.Name)
	if !found {
		// If old config is missing, the diff cannot be done. Bridge domain will be created as a new one. This
		// case should NOT happen, it means that the agent's state is inconsistent.
		c.log.Warnf("Bridge domain %v modify failed due to missing old configuration, will be created as a new one",
			newBdConfig.Name)
		return c.ConfigureBridgeDomain(newBdConfig)
	}

	// Update interfaces.
	toSet, toUnset := c.calculateIfaceDiff(newBdConfig.Interfaces, oldBdConfig.Interfaces)
	unConfIfs, err := c.bdHandler.UnsetInterfacesFromBridgeDomain(newBdConfig.Name, bdIdx, toUnset, c.ifIndexes)
	if err != nil {
		return errors.Errorf("failed to unset interfaces from modified bridge domain %s: %v", newBdConfig.Name, err)
	}
	newConfIfs, err := c.bdHandler.SetInterfacesToBridgeDomain(newBdConfig.Name, bdIdx, toSet, c.ifIndexes)
	if err != nil {
		return errors.Errorf("failed to set interfaces to modified bridge domain %s: %v", newBdConfig.Name, err)
	}
	// Refresh configured interfaces
	configuredIfs := reckonInterfaces(bdMeta.ConfiguredInterfaces, newConfIfs, unConfIfs)

	// Update ARP termination table.
	toAdd, toRemove := calculateARPDiff(newBdConfig.ArpTerminationTable, oldBdConfig.ArpTerminationTable)
	for _, entry := range toAdd {
		if err := c.bdHandler.VppAddArpTerminationTableEntry(bdIdx, entry.PhysAddress, entry.IpAddress); err != nil {
			return errors.Errorf("failed to set ARP termination (MAC %s) to bridge domain %s: %v",
				entry.PhysAddress, newBdConfig.Name, err)
		}
	}
	for _, entry := range toRemove {
		if err := c.bdHandler.VppRemoveArpTerminationTableEntry(bdIdx, entry.PhysAddress, entry.IpAddress); err != nil {
			return errors.Errorf("failed to remove ARP termination (MAC %s) to bridge domain %s: %v",
				entry.PhysAddress, newBdConfig.Name, err)
		}
	}

	// Push change to bridge domain state.
	if errLookup := c.propagateBdDetailsToStatus(bdIdx, newBdConfig.Name); errLookup != nil {
		return errLookup
	}

	// Update bridge domain's registered metadata
	c.log.Debugf("Updating bridge domain %s mapping metadata", newBdConfig.Name)
	if success := c.bdIndexes.UpdateMetadata(newBdConfig.Name, l2idx.NewBDMetadata(newBdConfig, configuredIfs)); !success {
		c.log.Errorf("Failed to update metadata for bridge domain %s", newBdConfig.Name)
	}

	c.log.Infof("Bridge domain %s modified", newBdConfig.Name)

	return nil
}

// DeleteBridgeDomain processes the NB config and propagates it to bin api calls.
func (c *BDConfigurator) DeleteBridgeDomain(bdConfig *l2.BridgeDomains_BridgeDomain) error {
	bdIdx, _, found := c.bdIndexes.LookupIdx(bdConfig.Name)
	if !found {
		return errors.Errorf("cannot remove bridge domain %s, index not found in the mapping", bdConfig.Name)
	}

	if err := c.deleteBridgeDomain(bdConfig, bdIdx); err != nil {
		return errors.Errorf("failed to remove bridge domain %s: %v", bdConfig.Name, err)
	}

	c.log.Infof("Bridge domain %s deleted", bdConfig.Name)

	return nil
}

func (c *BDConfigurator) deleteBridgeDomain(bdConfig *l2.BridgeDomains_BridgeDomain, bdIdx uint32) error {
	// Unmap all interfaces from removed bridge domain.
	if _, err := c.bdHandler.UnsetInterfacesFromBridgeDomain(bdConfig.Name, bdIdx, bdConfig.Interfaces, c.ifIndexes); err != nil {
		c.log.Error(err) // Do not return, try to remove bridge domain anyway
	}

	if err := c.bdHandler.VppDeleteBridgeDomain(bdIdx); err != nil {
		return errors.Errorf("failed to remove bridge domain %s: %v", bdConfig, err)
	}

	c.bdIndexes.UnregisterName(bdConfig.Name)
	c.log.Debugf("Bridge domain %s unregistered", bdConfig.Name)

	// Update bridge domain state.
	if err := c.propagateBdDetailsToStatus(bdIdx, bdConfig.Name); err != nil {
		return err
	}

	return nil
}

// PropagateBdDetailsToStatus looks for existing VPP bridge domain state and propagates it to the etcd bd state.
func (c *BDConfigurator) propagateBdDetailsToStatus(bdID uint32, bdName string) error {
	stateMsg := BridgeDomainStateMessage{}
	_, _, found := c.bdIndexes.LookupName(bdID)
	if !found {
		// If bridge domain does not exist in mapping, the lookup treats it as a removed bridge domain,
		// and ID in message is set to 0. Name has to be passed further in order
		// to be able to construct the key to remove the status from ETCD.
		stateMsg.Message = &l2ba.BridgeDomainDetails{
			BdID: 0,
		}
		stateMsg.Name = bdName
	} else {
		// Put current state data to status message.
		req := &l2ba.BridgeDomainDump{
			BdID: bdID,
		}
		reqContext := c.vppChan.SendRequest(req)
		msg := &l2ba.BridgeDomainDetails{}
		if err := reqContext.ReceiveReply(msg); err != nil {
			return errors.Errorf("BD status propagation: failed to dump bridge domains: %s", err)
		}
		stateMsg.Message = msg
		stateMsg.Name = bdName
	}

	// Propagate bridge domain state information to the bridge domain state updater.
	c.notificationChan <- stateMsg

	return nil
}

// ResolveCreatedInterface looks for bridge domain this interface is assigned to and sets it up.
func (c *BDConfigurator) ResolveCreatedInterface(ifName string, ifIdx uint32) error {
	// Find bridge domain where the interface should be assigned
	bdIdx, bd, bdIf, found := c.bdIndexes.LookupBdForInterface(ifName)
	if !found {
		return nil
	}
	configuredIf, err := c.bdHandler.SetInterfaceToBridgeDomain(bd.Name, bdIdx, bdIf, c.ifIndexes)
	if err != nil {
		return errors.Errorf("error while assigning registered interface %s to bridge domain %s: %v",
			ifName, bd.Name, err)
	}

	// Refresh metadata. Skip if resolved interface already exists
	_, bdMeta, found := c.bdIndexes.LookupIdx(bd.Name)
	if !found {
		return errors.Errorf("unable to get list of configured interfaces from %s", bd.Name)
	}
	var isUpdated bool
	for _, metaIf := range bdMeta.ConfiguredInterfaces {
		if metaIf == configuredIf {
			isUpdated = true
		}
	}
	if !isUpdated {
		c.bdIndexes.UpdateMetadata(bd.Name, l2idx.NewBDMetadata(bd, append(bdMeta.ConfiguredInterfaces, configuredIf)))
		c.log.Debugf("Bridge domain %s metadata updated", bd.Name)
	}

	// Push to bridge domain state.
	if err := c.propagateBdDetailsToStatus(bdIdx, bd.Name); err != nil {
		return err
	}

	return nil
}

// ResolveDeletedInterface is called by VPP if an interface is removed.
func (c *BDConfigurator) ResolveDeletedInterface(ifName string) error {
	// Find bridge domain the interface should be removed from
	bdIdx, bd, _, found := c.bdIndexes.LookupBdForInterface(ifName)
	if !found {
		return nil
	}

	// If interface belonging to a bridge domain is removed, VPP handles internal bridge domain update itself.
	// However, the etcd operational state and bridge domain metadata still needs to be updated to reflect changed VPP state.
	_, bdMeta, found := c.bdIndexes.LookupIdx(bd.Name)
	if !found {
		return errors.Errorf("unable to get list of configured interfaces for bridge domain %v", bd.Name)
	}
	for i, configuredIf := range bdMeta.ConfiguredInterfaces {
		if configuredIf == ifName {
			bdMeta.ConfiguredInterfaces = append(bdMeta.ConfiguredInterfaces[:i], bdMeta.ConfiguredInterfaces[i+1:]...)
			break
		}
	}
	c.bdIndexes.UpdateMetadata(bd.Name, l2idx.NewBDMetadata(bd, bdMeta.ConfiguredInterfaces))
	c.log.Debugf("Bridge domain %s metadata updated", bd.Name)
	err := c.propagateBdDetailsToStatus(bdIdx, bd.Name)
	if err != nil {
		return err
	}

	return nil
}

// The goal of the validation is to ensure that bridge domain does not contain more than one BVI interface
func (c *BDConfigurator) vppValidateBridgeDomainBVI(newBdConfig, oldBdConfig *l2.BridgeDomains_BridgeDomain) (bool, bool) {
	recreate := calculateBdParamsDiff(newBdConfig, oldBdConfig)
	if recreate {
		c.log.Debugf("Bridge domain %s base params changed, needs to be recreated", newBdConfig.Name)
	}
	if len(newBdConfig.Interfaces) == 0 {
		c.log.Debugf("Bridge domain %s does not contain any interface", newBdConfig.Name)
		return true, recreate
	}
	var bviCount int
	for _, bdInterface := range newBdConfig.Interfaces {
		if bdInterface.BridgedVirtualInterface {
			bviCount++
		}
	}
	if bviCount == 0 {
		c.log.Debugf("Bridge domain %s does not contain any bvi interface", newBdConfig.Name)
		return true, recreate
	} else if bviCount == 1 {
		return true, recreate
	} else {
		c.log.Warnf("Bridge domain %s contains more than one BVI interface", newBdConfig.Name)
		return false, recreate
	}
}

// Compares all base bridge domain params. Returns true if there is a difference, false otherwise.
func calculateBdParamsDiff(newBdConfig, oldBdConfig *l2.BridgeDomains_BridgeDomain) bool {
	if oldBdConfig == nil {
		// nothing to compare
		return false
	}

	if newBdConfig.ArpTermination != oldBdConfig.ArpTermination {
		return true
	}
	if newBdConfig.Flood != oldBdConfig.Flood {
		return true
	}
	if newBdConfig.Forward != oldBdConfig.Forward {
		return true
	}
	if newBdConfig.Learn != oldBdConfig.Learn {
		return true
	}
	if newBdConfig.MacAge != oldBdConfig.MacAge {
		return true
	}
	if newBdConfig.UnknownUnicastFlood != oldBdConfig.UnknownUnicastFlood {
		return true
	}
	return false
}

// Returns lists of interfaces which will be set or unset to bridge domain
// Unset:
//	* all interfaces which are no longer part of the bridge domain
//	* interface which will be set as BVI (in case the BVI was changed)
//	* interface which will be set as non-BVI
// Set:
// 	* all new interfaces added to bridge domain
//  * interface which was a BVI before
//  * interface which will be the new BVI
func (c *BDConfigurator) calculateIfaceDiff(newIfaces, oldIfaces []*l2.BridgeDomains_BridgeDomain_Interfaces) (toSet, toUnset []*l2.BridgeDomains_BridgeDomain_Interfaces) {
	// Find BVI interfaces (it may not be configured)
	var oldBVI, newBVI *l2.BridgeDomains_BridgeDomain_Interfaces
	for _, newIface := range newIfaces {
		if newIface.BridgedVirtualInterface {
			newBVI = newIface
			break
		}
	}
	for _, oldIface := range oldIfaces {
		if oldIface.BridgedVirtualInterface {
			oldBVI = oldIface
			break
		}
	}

	// If BVI was set/unset in general or the BVI interface was changed, pass the knowledge to the diff
	// resolution
	var bviChanged bool
	if (oldBVI == nil && newBVI != nil) || (oldBVI != nil && newBVI == nil) || (oldBVI != nil && newBVI != nil && oldBVI.Name != newBVI.Name) {
		bviChanged = true
	}

	// Resolve interfaces to unset
	for _, oldIface := range oldIfaces {
		var exists bool
		for _, newIface := range newIfaces {
			if oldIface.Name == newIface.Name && oldIface.SplitHorizonGroup == newIface.SplitHorizonGroup {
				exists = true
			}
		}
		// Unset interface as an obsolete one
		if !exists {
			toUnset = append(toUnset, oldIface)
			continue
		}
		if bviChanged {
			// unset deprecated BVI interface
			if oldBVI != nil && oldBVI.Name == oldIface.Name {
				toUnset = append(toUnset, oldIface)
				continue
			}
			// unset non-BVI interface which will be subsequently set as BVI
			if newBVI != nil && newBVI.Name == oldIface.Name {
				toUnset = append(toUnset, oldIface)
			}
		}
	}

	// Resolve interfaces to set
	for _, newIface := range newIfaces {
		var exists bool
		for _, oldIface := range oldIfaces {
			if newIface.Name == oldIface.Name && newIface.SplitHorizonGroup == oldIface.SplitHorizonGroup {
				exists = true
			}
		}
		// Set newly added interface
		if !exists {
			toSet = append(toSet, newIface)
			continue
		}
		if bviChanged {
			// Set non-BVI interface which was BVI before
			if oldBVI != nil && oldBVI.Name == newIface.Name {
				toSet = append(toSet, newIface)
				continue
			}
			// Set new BVI interface
			if newBVI != nil && newBVI.Name == newIface.Name {
				toSet = append(toSet, newIface)
			}
		}
	}

	return toSet, toUnset
}

// Recalculate configured interfaces according to output of binary API calls.
// - current is a list of interfaces present on the vpp before (read from old metadata)
// - added is a list of new configured interfaces
// - removed is a list of un-configured interfaces
// Note: resulting list of interfaces may NOT correspond with the one in bridge domain configuration.
func reckonInterfaces(current []string, added []string, removed []string) []string {
	for _, delItem := range removed {
		for i, currItem := range current {
			if currItem == delItem {
				current = append(current[:i], current[i+1:]...)
				break
			}
		}
	}
	return append(current, added...)
}

// resolve diff of ARP entries
func calculateARPDiff(newARPs, oldARPs []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry) (toAdd, toRemove []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry) {
	// Resolve ARPs to add
	for _, newARP := range newARPs {
		var exists bool
		for _, oldARP := range oldARPs {
			if newARP.IpAddress == oldARP.IpAddress && newARP.PhysAddress == oldARP.PhysAddress {
				exists = true
			}
		}
		if !exists {
			toAdd = append(toAdd, newARP)
		}
	}
	// Resolve ARPs to remove
	for _, oldARP := range oldARPs {
		var exists bool
		for _, newARP := range newARPs {
			if oldARP.IpAddress == newARP.IpAddress && oldARP.PhysAddress == newARP.PhysAddress {
				exists = true
			}
		}
		if !exists {
			toRemove = append(toRemove, oldARP)
		}
	}

	return toAdd, toRemove
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *BDConfigurator) LogError(err error) error {
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
