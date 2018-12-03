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
	"strings"

	"github.com/go-errors/errors"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// Resync writes missing BDs to the VPP and removes obsolete ones.
func (c *BDConfigurator) Resync(nbBDs []*l2.BridgeDomains_BridgeDomain) error {
	// Re-initialize cache
	c.clearMapping()

	// Dump current state of the VPP bridge domains
	vppBDs, err := c.bdHandler.DumpBridgeDomains()
	if err != nil {
		return errors.Errorf("bridge domain resyc error: failed to dump bridge domains: %v", err)
	}

	// Correlate with NB config
	for vppBDIdx, vppBD := range vppBDs {
		// tag is bridge domain name (unique identifier)
		tag := vppBD.Bd.Name
		// Find NB bridge domain with the same name
		var nbBD *l2.BridgeDomains_BridgeDomain
		for _, nbBDConfig := range nbBDs {
			if tag == nbBDConfig.Name {
				nbBD = nbBDConfig
				break
			}
		}
		// NB config does not exist, VPP bridge domain is obsolete
		if nbBD == nil {
			if err := c.deleteBridgeDomain(&l2.BridgeDomains_BridgeDomain{
				Name: tag,
			}, vppBDIdx); err != nil {
				return errors.Errorf("bridge domain resync error: failed to remove bridge domain %s: %v", tag, err)
			}
		} else {
			// Bridge domain exists, validate
			valid, recreate := c.vppValidateBridgeDomainBVI(nbBD, &l2.BridgeDomains_BridgeDomain{
				Name:                tag,
				Learn:               vppBD.Bd.Learn,
				Flood:               vppBD.Bd.Flood,
				Forward:             vppBD.Bd.Forward,
				UnknownUnicastFlood: vppBD.Bd.UnknownUnicastFlood,
				ArpTermination:      vppBD.Bd.ArpTermination,
				MacAge:              vppBD.Bd.MacAge,
			})
			if !valid {
				return errors.Errorf("bridge domain resync error: bridge domain %s config is invalid", tag)
			}
			if recreate {
				// Internal bridge domain parameters changed, cannot be modified and will be recreated
				if err := c.deleteBridgeDomain(&l2.BridgeDomains_BridgeDomain{
					Name: tag,
				}, vppBDIdx); err != nil {
					return errors.Errorf("bridge domain resync error: failed to remove bridge domain %s: %v", tag, err)
				}
				if err := c.ConfigureBridgeDomain(nbBD); err != nil {
					return errors.Errorf("bridge domain resync error: failed to create bridge domain %s: %v", tag, err)
				}
				continue
			}

			// todo currently it is not possible to dump interfaces. In order to prevent BD removal, unset all available interfaces
			// Dump all interfaces
			interfaceMap, err := c.ifHandler.DumpInterfaces()
			if err != nil {
				return errors.Errorf("bridge domain resync error: failed to dump interfaces: %v", err)
			}
			// Prepare a list of interface objects with proper name
			var interfacesToUnset []*l2.BridgeDomains_BridgeDomain_Interfaces
			for _, iface := range interfaceMap {
				interfacesToUnset = append(interfacesToUnset, &l2.BridgeDomains_BridgeDomain_Interfaces{
					Name: iface.Interface.Name,
				})
			}
			// Remove interfaces from bridge domain. Attempt to unset interface which does not belong to the bridge domain
			// does not cause an error
			if _, err := c.bdHandler.UnsetInterfacesFromBridgeDomain(nbBD.Name, vppBDIdx, interfacesToUnset, c.ifIndexes); err != nil {
				return errors.Errorf("bridge domain resync error: failed to unset interfaces from bridge domain %s: %v",
					nbBD.Name, err)
			}
			// Set all new interfaces to the bridge domain
			// todo there is no need to calculate diff from configured interfaces, because currently all available interfaces are set here
			configuredIfs, err := c.bdHandler.SetInterfacesToBridgeDomain(nbBD.Name, vppBDIdx, nbBD.Interfaces, c.ifIndexes)
			if err != nil {
				return errors.Errorf("bridge domain resync error: failed to set interfaces from bridge domain %s: %v",
					nbBD.Name, err)
			}

			// todo VPP does not support ARP dump, they can be only added at this time
			// Resolve new ARP entries
			for _, arpEntry := range nbBD.ArpTerminationTable {
				if err := c.bdHandler.VppAddArpTerminationTableEntry(vppBDIdx, arpEntry.PhysAddress, arpEntry.IpAddress); err != nil {
					return errors.Errorf("bridge domain resync error: failed to add arp termination entry (MAC %s) to bridge domain %s: %v",
						arpEntry.PhysAddress, nbBD.Name, err)
				}
			}

			// Register bridge domain
			c.bdIndexes.RegisterName(nbBD.Name, c.bdIDSeq, l2idx.NewBDMetadata(nbBD, configuredIfs))
			c.bdIDSeq++
			c.log.Debugf("Bridge domain resync: %s registered", tag)
		}
	}

	// Configure new bridge domains
	for _, newBD := range nbBDs {
		_, _, found := c.bdIndexes.LookupIdx(newBD.Name)
		if !found {
			if err := c.ConfigureBridgeDomain(newBD); err != nil {
				return errors.Errorf("bridge domain resync error: failed to configure %s: %v", newBD.Name, err)
			}
		}
	}

	return nil
}

// Resync writes missing FIBs to the VPP and removes obsolete ones.
func (c *FIBConfigurator) Resync(nbFIBs []*l2.FibTable_FibEntry) error {
	// Re-initialize cache
	c.clearMapping()

	// Get all FIB entries configured on the VPP
	vppFIBs, err := c.fibHandler.DumpFIBTableEntries()
	if err != nil {
		return errors.Errorf("FIB resync error: failed to dump FIB table: %v", err)
	}

	// Correlate existing config with the NB
	for vppFIBmac, vppFIBdata := range vppFIBs {
		exists, meta := func(nbFIBs []*l2.FibTable_FibEntry) (bool, *l2.FibTable_FibEntry) {
			for _, nbFIB := range nbFIBs {
				// Physical address
				if strings.ToUpper(vppFIBmac) != strings.ToUpper(nbFIB.PhysAddress) {
					continue
				}
				// Bridge domain
				bdIdx, _, found := c.bdIndexes.LookupIdx(nbFIB.BridgeDomain)
				if !found || vppFIBdata.Meta.BdID != bdIdx {
					continue
				}
				// BVI
				if vppFIBdata.Fib.BridgedVirtualInterface != nbFIB.BridgedVirtualInterface {
					continue
				}
				// Interface
				swIdx, _, found := c.ifIndexes.LookupIdx(nbFIB.OutgoingInterface)
				if !found || vppFIBdata.Meta.IfIdx != swIdx {
					continue
				}
				// Is static
				if vppFIBdata.Fib.StaticConfig != nbFIB.StaticConfig {
					continue
				}

				return true, nbFIB
			}
			return false, nil
		}(nbFIBs)

		// Register existing entries, Remove entries missing in NB config (except non-static)
		if exists {
			c.fibIndexes.RegisterName(vppFIBmac, c.fibIndexSeq, meta)
			c.fibIndexSeq++
		} else if vppFIBdata.Fib.StaticConfig {
			// Get appropriate interface/bridge domain names
			ifIdx, _, ifFound := c.ifIndexes.LookupName(vppFIBdata.Meta.IfIdx)
			bdIdx, _, bdFound := c.bdIndexes.LookupName(vppFIBdata.Meta.BdID)
			if !ifFound || !bdFound {
				// FIB entry cannot be removed without these informations and
				// it should be removed by the VPP
				continue
			}

			if err := c.Delete(&l2.FibTable_FibEntry{
				PhysAddress:       vppFIBmac,
				OutgoingInterface: ifIdx,
				BridgeDomain:      bdIdx,
			}, func(callbackErr error) {
				if callbackErr != nil {
					c.log.Error(callbackErr)
				}
			}); err != nil {
				return errors.Errorf("FIB resync error: failed to remove entry %s: %v", vppFIBmac, err)
			}
		}
	}

	// Configure all unregistered FIB entries from NB config
	for _, nbFIB := range nbFIBs {
		_, _, found := c.fibIndexes.LookupIdx(nbFIB.PhysAddress)
		if !found {
			if err := c.Add(nbFIB, func(callbackErr error) {
				if callbackErr != nil {
					c.log.Error(callbackErr)
				}
			}); err != nil {
				return errors.Errorf("FIB resync error: failed to add entry %s: %v", nbFIB.PhysAddress, err)
			}
		}
	}

	c.log.Info("FIB resync done")

	return nil
}

// Resync writes missing XCons to the VPP and removes obsolete ones.
func (c *XConnectConfigurator) Resync(nbXConns []*l2.XConnectPairs_XConnectPair) error {
	// Re-initialize cache
	c.clearMapping()

	// Read cross connects from the VPP
	vppXConns, err := c.xcHandler.DumpXConnectPairs()
	if err != nil {
		return errors.Errorf("XConnect resync error: failed to dump XConnect data: %v", err)
	}

	// Correlate with NB config
	for _, vppXConn := range vppXConns {
		var existsInNB bool
		var rxIfName, txIfName string
		for _, nbXConn := range nbXConns {
			// Find receive and transmit interface
			rxIfName, _, rxIfFound := c.ifIndexes.LookupName(vppXConn.Meta.ReceiveInterfaceSwIfIdx)
			txIfName, _, txIfFound := c.ifIndexes.LookupName(vppXConn.Meta.TransmitInterfaceSwIfIdx)
			if !rxIfFound || !txIfFound {
				continue
			}
			if rxIfName == nbXConn.ReceiveInterface && txIfName == nbXConn.TransmitInterface {
				// NB XConnect correlated with VPP
				c.xcIndexes.RegisterName(nbXConn.ReceiveInterface, c.xcIndexSeq, nbXConn)
				c.xcIndexSeq++
				c.log.Debugf("XConnect resync: %s registered", nbXConn.ReceiveInterface)
				existsInNB = true
			}
		}
		if !existsInNB {
			if err := c.DeleteXConnectPair(&l2.XConnectPairs_XConnectPair{
				ReceiveInterface:  rxIfName,
				TransmitInterface: txIfName,
			}); err != nil {
				return errors.Errorf("XConnect resync error: failed to remove XConnect %s-%s: %v",
					rxIfName, txIfName, err)
			}
		}
	}

	// Configure new XConnect pairs
	for _, nbXConn := range nbXConns {
		_, _, found := c.xcIndexes.LookupIdx(nbXConn.ReceiveInterface)
		if !found {
			if err := c.ConfigureXConnectPair(nbXConn); err != nil {
				return errors.Errorf("XConnect resync error: failed to configure XConnect %s-%s: %v",
					nbXConn.ReceiveInterface, nbXConn.TransmitInterface, err)
			}
		}
	}

	c.log.Debug("XConnect resync done")

	return nil
}
