// SPDX-License-Identifier:Apache-2.0

package config

import (
	"fmt"

	"github.com/pkg/errors"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/ipfamily"
)

type Validate func(ClusterResources) error

func ValidationFor(bgpImpl string) Validate {
	switch bgpImpl {
	case "frr":
		return DiscardNativeOnly
	case "native":
		return DiscardFRROnly
	}
	return DontValidate
}

// DiscardFRROnly returns an error if the current configFile contains
// any options that are available only in the FRR implementation.
func DiscardFRROnly(c ClusterResources) error {
	for _, p := range c.Peers {
		if p.Spec.BFDProfile != "" {
			return fmt.Errorf("peer %s has bfd-profile set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.KeepaliveTime.Duration != 0 {
			return fmt.Errorf("peer %s has keepalive-time set on native bgp mode", p.Spec.Address)
		}
		if p.Spec.VRFName != "" {
			return fmt.Errorf("peer %s has vrf set on native bgp mode", p.Spec.Address)
		}
	}
	if len(c.BFDProfiles) > 0 {
		return errors.New("bfd profiles section set")
	}
	// Only IPv4 BGP advertisements are supported in native mode.
	if err := findIPv6BGPAdvertisement(c); err != nil {
		return err
	}
	// Only legacy type communities are supported in native mode.
	return findNonLegacyCommunity(c)
}

// findIPv6BGPAdvertisement checks for IPv6 addresses. If it finds at least one IPv6 BGP advertisement, it will throw
// and error as it's not supported in native mode.
func findIPv6BGPAdvertisement(c ClusterResources) error {
	if len(c.BGPAdvs) == 0 {
		return nil
	}
	for _, p := range c.Pools {
		for _, cidr := range p.Spec.Addresses {
			nets, err := ParseCIDR(cidr)
			if err != nil {
				return fmt.Errorf("invalid CIDR %q in pool %q: %s", cidr, p.Name, err)
			}
			for _, n := range nets {
				if n.IP.To4() == nil {
					return fmt.Errorf("pool %q has ipv6 CIDR %s, native bgp mode does not support ipv6", p.Name, n)
				}
			}
		}
	}
	return nil
}

// findNonLegacyCommunity returns an error if it can find a non legacy community. If a community string can not be
// parsed, the string will be ignored.
func findNonLegacyCommunity(c ClusterResources) error {
	for _, p := range c.LegacyAddressPools {
		for _, adv := range p.Spec.BGPAdvertisements {
			for _, cs := range adv.Communities {
				c, err := community.New(cs)
				if err != nil {
					// Skip aliases.
					continue
				}
				if !community.IsLegacy(c) {
					return fmt.Errorf("native BGP mode only supports legacy communities, legacy address pool %q "+
						"has non legacy community %q", p.Name, cs)
				}
			}
		}
	}
	for _, adv := range c.BGPAdvs {
		for _, cs := range adv.Spec.Communities {
			c, err := community.New(cs)
			if err != nil {
				// Skip aliases.
				continue
			}
			if !community.IsLegacy(c) {
				return fmt.Errorf("native BGP mode only supports legacy communities, BGP advertisement %q "+
					"has non legacy community %q", adv.Name, cs)
			}
		}
	}
	for _, co := range c.Communities {
		for _, cs := range co.Spec.Communities {
			c, err := community.New(cs.Value)
			if err != nil {
				// Skip it if we cannot parse it - the purpose of this very verification is not to make sure that
				// a string can be parsed or not.
				continue
			}
			if !community.IsLegacy(c) {
				return fmt.Errorf("native BGP mode only supports legacy communities, BGP community CR %q "+
					"has non legacy community %q", co.Name, cs)
			}
		}
	}
	return nil
}

// DontValidate is a Validate function that always returns
// success.
func DontValidate(c ClusterResources) error {
	return nil
}

// DiscardNativeOnly returns an error if the current configFile contains
// any options that are available only in the native implementation.
func DiscardNativeOnly(c ClusterResources) error {
	if len(c.Peers) > 1 {
		peerAddr := make(map[string]bool)
		routerID := c.Peers[0].Spec.RouterID
		peer0 := peerAddressKey(c.Peers[0].Spec)
		peerAddr[peer0] = true
		for _, p := range c.Peers[1:] {
			if p.Spec.RouterID != routerID {
				return fmt.Errorf("peer %s has RouterID different from %s, in FRR mode all RouterID must be equal", p.Spec.RouterID, c.Peers[0].Spec.RouterID)
			}
			peerKey := peerAddressKey(p.Spec)
			if _, ok := peerAddr[peerKey]; ok {
				return fmt.Errorf("peer %s already exists, FRR mode doesn't support duplicate BGPPeers", p.Spec.Address)
			}
			peerAddr[peerKey] = true
		}
	}
	return nil
}

// validateConfig is meant to validate all the inter-dependencies of a parsed configuration.
// In this case, we ensure that bfd echo is not enabled on a v6 pool.
func validateConfig(cfg *Config) error {
	for _, p := range cfg.Pools.ByName {
		containsV6 := false
		for _, cidr := range p.CIDR {
			if ipfamily.ForCIDR(cidr) == ipfamily.IPv6 {
				containsV6 = true
				break
			}
		}
		if !containsV6 { // we only care about v6 advertisements
			continue
		}
		for _, a := range p.BGPAdvertisements {
			if len(a.Peers) == 0 { // all peers
				for _, peer := range cfg.Peers {
					if hasBFDEcho(peer, cfg.BFDProfiles) {
						return fmt.Errorf("pool %s has bgpadvertisement %s which references peer %s which has bfd echo enabled, which is not possible", p.Name, a.Name, peer.Name)
					}
				}
				continue
			}
			for _, peerName := range a.Peers {
				if peer, ok := cfg.Peers[peerName]; ok {
					if hasBFDEcho(peer, cfg.BFDProfiles) {
						return fmt.Errorf("pool %s has bgpadvertisement %s which references peer %s which has bfd echo enabled, which is not possible", p.Name, a.Name, peer.Name)
					}
				}
			}
		}
	}
	return nil
}

func hasBFDEcho(peer *Peer, bfdProfiles map[string]*BFDProfile) bool {
	profile, ok := bfdProfiles[peer.BFDProfile]
	if !ok {
		return false
	}
	if profile.EchoMode {
		return true
	}
	return false
}

func peerAddressKey(peer metallbv1beta2.BGPPeerSpec) string {
	return fmt.Sprintf("%s-%s", peer.Address, peer.VRFName)
}
