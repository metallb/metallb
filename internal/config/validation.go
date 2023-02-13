// SPDX-License-Identifier:Apache-2.0

package config

import (
	"fmt"

	"github.com/pkg/errors"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
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
	if len(c.BGPAdvs) == 0 {
		return nil
	}
	// we check for ipv6 addresses if we have at least one bgp advertisement, as it's
	// not supported in native mode.
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
	for _, p := range c.Peers {
		for _, p1 := range c.Peers[1:] {
			if p.Spec.MyASN != p1.Spec.MyASN &&
				p.Spec.VRFName == p1.Spec.VRFName {
				return fmt.Errorf("peer %s has myAsn different from %s, in FRR mode all myAsn must be equal for the same VRF", p.Spec.Address, p1.Spec.Address)
			}

		}
	}
	return nil
}

func peerAddressKey(peer metallbv1beta2.BGPPeerSpec) string {
	return fmt.Sprintf("%s-%s", peer.Address, peer.VRFName)
}
