// SPDX-License-Identifier:Apache-2.0

package config

import (
	"fmt"

	"github.com/pkg/errors"
)

type Validate func(ClusterResources) error

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
	}
	if len(c.BFDProfiles) > 0 {
		return errors.New("bfd profiles section set")
	}
	for _, p := range c.Pools {
		if p.Spec.Protocol == string(BGP) {
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
		myAsn := c.Peers[0].Spec.MyASN
		routerID := c.Peers[0].Spec.RouterID
		peerAddr[c.Peers[0].Spec.Address] = true
		for _, p := range c.Peers[1:] {
			if p.Spec.RouterID != routerID {
				return fmt.Errorf("peer %s has RouterID different from %s, in FRR mode all RouterID must be equal", p.Spec.RouterID, c.Peers[0].Spec.RouterID)
			}
			if p.Spec.MyASN != myAsn {
				return fmt.Errorf("peer %s has myAsn different from %s, in FRR mode all myAsn must be equal", p.Spec.Address, c.Peers[0].Spec.Address)
			}
			if _, ok := peerAddr[p.Spec.Address]; ok {
				return fmt.Errorf("peer %s already exists, FRR mode doesn't support duplicate BGPPeers", p.Spec.Address)
			}
			peerAddr[p.Spec.Address] = true
		}
	}
	return nil
}
