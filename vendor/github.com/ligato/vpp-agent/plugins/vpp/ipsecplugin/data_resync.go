//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package ipsecplugin

import (
	"github.com/go-errors/errors"
	"github.com/ligato/vpp-agent/plugins/vpp/ipsecplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/ipsec"
)

// IP address of unset field
const defaultIP = "0.0.0.0"

// Resync writes missing IPSec configs to the VPP and removes obsolete ones.
func (c *IPSecConfigurator) Resync(nbSpds []*ipsec.SecurityPolicyDatabases_SPD, nbSas []*ipsec.SecurityAssociations_SA, nbTunnels []*ipsec.TunnelInterfaces_Tunnel) error {
	c.clearMapping()

	// Read VPP IPSec data
	vppSas, err := c.ipSecHandler.DumpIPSecSA()
	if err != nil {
		return errors.Errorf("IPSec resync: failed to dump security associations: %v", err)
	}
	vppSpds, err := c.ipSecHandler.DumpIPSecSPD()
	if err != nil {
		return errors.Errorf("IPSec resync: failed to dump security policy databases: %v", err)
	}
	vppTunnels, err := c.ipSecHandler.DumpIPSecTunnelInterfaces()
	if err != nil {
		return errors.Errorf("IPSec resync: failed to dump tunnel interfaces: %v", err)
	}

	// Remove all security policy databases before manipulating security associations.
	// TODO since IPSec interface dump is missing, all SPDs will be removed since diff cannot be calculated
	for _, vppSpdDetails := range vppSpds {
		// First register all policy entries
		vppSpd := vppSpdDetails.Spd
		for _, spdPolicyEntry := range vppSpdDetails.Spd.PolicyEntries {
			// Find ID for given policy
			meta, ok := vppSpdDetails.PolicyMeta[spdPolicyEntry.Sa]
			if !ok {
				c.log.Warnf("Metadata for SPD gen name %s not found", spdPolicyEntry.Sa)
				continue
			}
			vppSpd.Name = "<unknown>"
			c.spdIndexes.RegisterName(vppSpd.Name, meta.SpdID, nil)
		}
		if err := c.DeleteSPD(vppSpd); err != nil {
			return errors.Errorf("IPSec resync: failed to remove VPP SPD (sp: %s): %v", vppSpd.Name, err)
		}
		c.log.Debugf("IPSec resync: removed VPP SPD %s", vppSpd.Name)
	}

	// Resolve security associations
	if err := c.synchronizeSA(vppSas, nbSas); err != nil {
		return err
	}

	// Configure all NB SPDs
	for _, nbSpd := range nbSpds {
		if err := c.ConfigureSPD(nbSpd); err != nil {
			return errors.Errorf("IPSec resync: failed to configure VPP SPD: %v", err)
		}
		c.log.Debugf("IPSec resync: configured VPP SPD %s", nbSpd.Name)
	}

	// Sync tunnel interfaces
	resolvedVppIfs := make(map[uint32]string)
	for _, nbTun := range nbTunnels {
		var correlated bool
		for _, vppTun := range vppTunnels {
			if nbTun.Name == vppTun.Tunnel.Name {
				if c.isIfModified(nbTun, vppTun.Tunnel) {
					if err := c.ModifyTunnel(vppTun.Tunnel, nbTun); err != nil {
						return errors.Errorf("IPSec resync: failed to modify tunnel interface %s: %v", nbTun.Name, err)
					}
				} else {
					// Update metadata for tunnel interface. Tunnel was registered during interface plugin resync.
					c.ifIndexes.UpdateMetadata(nbTun.Name, &interfaces.Interfaces_Interface{
						Name:        nbTun.Name,
						Enabled:     nbTun.Enabled,
						IpAddresses: nbTun.IpAddresses,
						Vrf:         nbTun.Vrf,
					})
				}
				correlated = true
				resolvedVppIfs[vppTun.Meta.SwIfIndex] = vppTun.Tunnel.Name
			}
		}
		if !correlated {
			if err := c.ConfigureTunnel(nbTun); err != nil {
				return errors.Errorf("IPSec resync: failed to configure tunnel interface %s: %v", nbTun.Name, err)
			}
		}
	}
	// Remove obsolete tunnels
	for _, vppTun := range vppTunnels {
		_, ok := resolvedVppIfs[vppTun.Meta.SwIfIndex]
		if !ok {
			if err := c.DeleteTunnel(vppTun.Tunnel); err != nil {
				return errors.Errorf("IPSec resync: failed to remove tunnel interface %s: %v", vppTun.Tunnel.Name, err)
			}
		}
	}

	c.log.Debug("IPSec resync done")
	return nil
}

// Correlates security associations, removes obsolete and configures new ones
func (c *IPSecConfigurator) synchronizeSA(vppSAs []*vppcalls.IPSecSaDetails, nbSAs []*ipsec.SecurityAssociations_SA) error {
	for _, nbSa := range nbSAs {
		var found bool
		c.log.Debugf("looking for SA %s in the VPP", nbSa.Name)
		// Look for VPP security association
		for _, vppSaDetails := range vppSAs {
			vppSa := vppSaDetails.Sa
			if nbSa.GetSpi() != vppSa.GetSpi() {
				c.log.Debugf("SA comparison: different SPI (nb: %d vs vpp: %d)", nbSa.GetSpi(), vppSa.GetSpi())
				continue
			}
			if nbSa.GetCryptoKey() != vppSa.GetCryptoKey() {
				c.log.Debugf("SA comparison: different crypto key (nb: %s vs vpp: %s)", nbSa.GetCryptoKey(), vppSa.GetCryptoKey())
				continue
			}
			if nbSa.GetCryptoAlg() != vppSa.GetCryptoAlg() {
				c.log.Debugf("SA comparison: different crypto alg (nb: %v vs vpp: %v)", nbSa.GetEnableUdpEncap(), vppSa.GetEnableUdpEncap())
				continue
			}
			if nbSa.GetIntegKey() != vppSa.GetIntegKey() {
				c.log.Debugf("SA comparison: different integ key (nb: %s vs vpp: %s)", nbSa.GetIntegKey(), vppSa.GetIntegKey())
				continue
			}
			if nbSa.GetIntegAlg() != vppSa.GetIntegAlg() {
				c.log.Debugf("SA comparison: different integ alg (nb: %d vs vpp: %d)", nbSa.GetIntegAlg(), vppSa.GetIntegAlg())
				continue
			}
			if nbSa.GetTunnelSrcAddr() == "" && vppSa.GetTunnelSrcAddr() != defaultIP {
				c.log.Debugf("SA comparison: tunnel src IP not set for nb, but is %s for vpp)", vppSa.GetTunnelSrcAddr())
				continue
			} else if nbSa.GetTunnelSrcAddr() != "" && nbSa.GetTunnelSrcAddr() != vppSa.GetTunnelSrcAddr() {
				c.log.Debugf("SA comparison: different tunnel src IP (nb: %s vs vpp: %s)", nbSa.GetTunnelSrcAddr(), vppSa.GetTunnelSrcAddr())
				continue
			}
			if nbSa.GetTunnelDstAddr() == "" && vppSa.GetTunnelDstAddr() != defaultIP {
				c.log.Debugf("SA comparison: tunnel dst IP not set for nb, but is %s for vpp)", vppSa.GetTunnelSrcAddr())
				continue
			} else if nbSa.GetTunnelDstAddr() != "" && nbSa.GetTunnelDstAddr() != vppSa.GetTunnelDstAddr() {
				c.log.Debugf("SA comparison: different tunnel dst IP (nb: %s vs vpp: %s)", nbSa.GetTunnelDstAddr(), vppSa.GetTunnelDstAddr())
				continue
			}
			if nbSa.GetUseAntiReplay() != vppSa.GetUseAntiReplay() {
				c.log.Debugf("SA comparison: different use anti replay (nb: %v vs vpp: %v)", nbSa.GetUseAntiReplay(), vppSa.GetTunnelDstAddr())
				continue
			}
			if nbSa.GetUseEsn() != vppSa.GetUseEsn() {
				c.log.Debugf("SA comparison: different use ESN (nb: %v vs vpp: %v)", nbSa.GetUseEsn(), vppSa.GetUseEsn())
				continue
			}
			if nbSa.GetEnableUdpEncap() != vppSa.GetEnableUdpEncap() {
				c.log.Debugf("SA comparison: different enable UDP encap (nb: %v vs vpp: %v)", nbSa.GetEnableUdpEncap(), vppSa.GetEnableUdpEncap())
				continue
			}
			if nbSa.GetProtocol() != vppSa.GetProtocol() {
				c.log.Debugf("SA comparison: different protocol (nb: %d vs vpp: %d)", nbSa.GetProtocol(), vppSa.GetProtocol())
				continue
			}
			found = true
			vppSa.Name = nbSa.Name // So it can be identified
			break
		}
		if !found {
			if err := c.ConfigureSA(nbSa); err != nil {
				return errors.Errorf("IPSec resync: failed to configure VPP SA %s: %v", nbSa.Name, err)
			}
			c.log.Debugf("IPSec resync: configured VPP SA %s", nbSa.Name)
		} else {
			c.saIndexes.RegisterName(nbSa.Name, c.saIndexSeq, nil)
			c.saIndexSeq++
			c.log.Debugf("SA %s registered without additional changes", nbSa.Name)
		}
	}

	for _, vppSaDetails := range vppSAs {
		vppSa := vppSaDetails.Sa
		// Remove all without name
		if vppSa.Name == "" {
			vppSa.Name = "<unknown>"
			c.saIndexes.RegisterName(vppSa.Name, vppSaDetails.Meta.SaID, nil)
			if err := c.DeleteSA(vppSa); err != nil {
				return errors.Errorf("IPSec resync: failed to remove VPP SA (sp: %d): %v", vppSa.Spi, err)
			}
			c.log.Debugf("IPSec resync: removed VPP SA (spi: %d)", vppSa.Spi)
		}
	}

	return nil
}

// Returns true if provided tunnel interfaces are different, false if they are equal
func (c *IPSecConfigurator) isIfModified(vppTun, nbTun *ipsec.TunnelInterfaces_Tunnel) bool {
	if nbTun.GetName() != vppTun.GetName() {
		c.log.Debugf("Tunnel comparison: different name (nb: %s vs vpp: %s)", nbTun.GetName(), vppTun.GetName())
		return true
	}
	if nbTun.GetEsn() != vppTun.GetEsn() {
		c.log.Debugf("Tunnel comparison: different esn (nb: %v vs vpp: %v)", nbTun.GetEsn(), vppTun.GetEsn())
		return true
	}
	if nbTun.GetAntiReplay() != vppTun.GetAntiReplay() {
		c.log.Debugf("Tunnel comparison: different anti replay (nb: %v vs vpp: %v)", nbTun.GetAntiReplay(), vppTun.GetAntiReplay())
		return true
	}
	if c.localRemoteIPSpiIsDifferent(nbTun, vppTun) {
		return true
	}
	if nbTun.GetCryptoAlg() != vppTun.GetCryptoAlg() {
		c.log.Debugf("Tunnel comparison: different crypto alg (nb: %v vs vpp: %v)", nbTun.GetCryptoAlg(), vppTun.GetCryptoAlg())
		return true
	}
	if nbTun.GetIntegAlg() != vppTun.GetIntegAlg() {
		c.log.Debugf("Tunnel comparison: different integ alg (nb: %v vs vpp: %v)", nbTun.GetIntegAlg(), vppTun.GetIntegAlg())
		return true
	}
	if len(nbTun.GetIpAddresses()) != len(vppTun.GetIpAddresses()) {
		c.log.Debugf("Tunnel comparison: different IP address count (nb: %v vs vpp: %v)", len(nbTun.GetIpAddresses()), len(vppTun.GetIpAddresses()))
		return true
	}
	if c.ipAddressesAreDifferent(nbTun.GetIpAddresses(), vppTun.GetIpAddresses()) {
		return true
	}
	if nbTun.GetVrf() != vppTun.GetVrf() {
		c.log.Debugf("SA comparison: different protocol (nb: %d vs vpp: %d)", nbTun.GetVrf(), vppTun.GetVrf())
		return true
	}
	return false
}

func (c *IPSecConfigurator) localRemoteIPSpiIsDifferent(nbTun, vppTun *ipsec.TunnelInterfaces_Tunnel) bool {
	if nbTun.GetLocalIp() == vppTun.GetLocalIp() && nbTun.GetLocalSpi() == vppTun.GetLocalSpi() {
		if nbTun.GetRemoteIp() != vppTun.GetRemoteIp() {
			c.log.Debugf("Tunnel comparison: different remote IP (nb: %s vs vpp: %s)", nbTun.GetRemoteIp(), vppTun.GetRemoteIp())
			return true
		}
		if nbTun.GetRemoteSpi() != vppTun.GetRemoteSpi() {
			c.log.Debugf("Tunnel comparison: different remote spi (nb: %d vs vpp: %d)", nbTun.GetRemoteSpi(), vppTun.GetRemoteSpi())
			return true
		}
	} else if nbTun.GetLocalIp() == vppTun.GetRemoteIp() && nbTun.GetLocalSpi() == vppTun.GetRemoteSpi() {
		if nbTun.GetRemoteIp() != vppTun.GetLocalIp() {
			c.log.Debugf("Tunnel comparison: different remote IP (nb: %s vs vpp: %s)", nbTun.GetRemoteIp(), vppTun.GetRemoteIp())
			return true
		}
		if nbTun.GetRemoteSpi() != vppTun.GetLocalSpi() {
			c.log.Debugf("Tunnel comparison: different remote spi (nb: %d vs vpp: %d)", nbTun.GetRemoteSpi(), vppTun.GetRemoteSpi())
			return true
		}
	} else {
		c.log.Debugf("Tunnel comparison: different local ip/spi (nb: %s/%d vs vpp: %s/%d)",
			nbTun.GetLocalIp(), nbTun.GetLocalSpi(), vppTun.GetLocalIp(), vppTun.GetLocalSpi())
		return true
	}
	return false
}

func (c *IPSecConfigurator) ipAddressesAreDifferent(nbIPs, vppIPs []string) bool {
	for _, nbIP := range nbIPs {
		var found bool
		for _, vppIP := range vppIPs {
			if nbIP == vppIP {
				found = true
			}
		}
		if !found {
			c.log.Debugf("SA comparison: IP address %s is missing on vpp tunnel", nbIP)
			return true
		}
	}
	return false
}
