// SPDX-License-Identifier:Apache-2.0

package config

import (
	"fmt"

	"github.com/pkg/errors"
)

type Validate func(*configFile) error

// DiscardFRROnly returns an error if the current configFile contains
// any options that are available only in the FRR implementation.
func DiscardFRROnly(c *configFile) error {
	for _, p := range c.Peers {
		if p.BFDProfile != "" {
			return fmt.Errorf("peer %s has bfd-profile set on native bgp mode", p.Addr)
		}
		if p.KeepaliveTime != "" {
			return fmt.Errorf("peer %s has keepalive-time set on native bgp mode", p.Addr)
		}
	}
	if len(c.BFDProfiles) > 0 {
		return errors.New("bfd profiles section set")
	}
	for _, p := range c.Pools {
		if p.Protocol == BGP {
			for _, cidr := range p.Addresses {
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
func DontValidate(c *configFile) error {
	return nil
}

// DiscardNativeOnly returns an error if the current configFile contains
// any options that are available only in the native implementation.
func DiscardNativeOnly(c *configFile) error {
	if len(c.Peers) > 1 {
		peerAddr := make(map[string]bool)
		myAsn := c.Peers[0].MyASN
		routerID := c.Peers[0].RouterID
		peerAddr[c.Peers[0].Addr] = true
		for _, p := range c.Peers[1:] {
			if p.RouterID != routerID {
				return fmt.Errorf("peer %s has RouterID different from %s, in FRR mode all RouterID must be equal", p.RouterID, c.Peers[0].RouterID)
			}
			if p.MyASN != myAsn {
				return fmt.Errorf("peer %s has myAsn different from %s, in FRR mode all myAsn must be equal", p.Addr, c.Peers[0].Addr)
			}
			if _, ok := peerAddr[p.Addr]; ok {
				return fmt.Errorf("peer %s already exists, FRR mode doesn't support duplicate BGPPeers", p.Addr)
			}
			peerAddr[p.Addr] = true
		}
	}
	return nil
}
