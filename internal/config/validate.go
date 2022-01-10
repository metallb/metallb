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
		for _, a := range p.BGPAdvertisements {
			if a.AggregationLengthV6 != nil {
				return fmt.Errorf("pool %s has aggregation-lenght-v6 set on native bgp mode", p.Name)
			}
		}
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
	if len(c.Peers) > 0 {
		myAsn := c.Peers[0].MyASN
		for _, p := range c.Peers {
			if p.RouterID != "" {
				return fmt.Errorf("peer %s has routerid set on frr mode", p.Addr)
			}
			if p.MyASN != myAsn {
				return fmt.Errorf("peer %s has myAsn different from %s, in FRR mode all myAsn must be equal", p.Addr, c.Peers[0].Addr)
			}
		}
	}
	return nil
}
