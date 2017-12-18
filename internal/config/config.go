// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config // import "go.universe.tf/metallb/internal/config"

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
)

// configFile is the configuration as parsed out of the ConfigMap,
// without validation or useful high level types.
type configFile struct {
	Peers          []peer
	BGPCommunities map[string]string `yaml:"bgp-communities"`
	Pools          []addressPool     `yaml:"address-pools"`
}

type peer struct {
	MyASN    uint32 `yaml:"my-asn"`
	ASN      uint32 `yaml:"peer-asn"`
	Addr     string `yaml:"peer-address"`
	Port     uint16 `yaml:"peer-port"`
	HoldTime string `yaml:"hold-time"`
}

type addressPool struct {
	Protocol          Proto
	Name              string
	CIDR              []string
	AvoidBuggyIPs     bool               `yaml:"avoid-buggy-ips"`
	AutoAssign        *bool              `yaml:"auto-assign"`
	BGPAdvertisements []bgpAdvertisement `yaml:"bgp-advertisements"`
	ARPNetwork        string             `yaml:"arp-network"`
}

type bgpAdvertisement struct {
	AggregationLength *int `yaml:"aggregation-length"`
	LocalPref         *uint32
	Communities       []string
}

// Config is a parsed MetalLB configuration.
type Config struct {
	// Routers that MetalLB should peer with.
	Peers []*Peer
	// Address pools from which to allocate load balancer IPs.
	Pools map[string]*Pool
}

// Proto holds the protocol we are speaking.
type Proto string

// MetalLB supported protocols.
const (
	ARP Proto = "arp"
	BGP       = "bgp"
)

// Peer is the configuration of a BGP peering session.
type Peer struct {
	// AS number to use for the local end of the session.
	MyASN uint32
	// AS number to expect from the remote end of the session.
	ASN uint32
	// Address to dial when establishing the session.
	Addr net.IP
	// Port to dial when establishing the session.
	Port uint16
	// Requested BGP hold time, per RFC4271.
	HoldTime time.Duration
	// TODO: more BGP session settings
}

// Pool is the configuration of an IP address pool.
type Pool struct {
	// Protocol for this pool, supported values: "arp" and "bgp".
	Protocol Proto
	// The addresses that are part of this pool, expressed as CIDR
	// prefixes. config.Parse guarantees that these are
	// non-overlapping, both within and between pools.
	CIDR []*net.IPNet
	// Some buggy consumer devices mistakenly drop IPv4 traffic for IP
	// addresses ending in .0 or .255, due to poor implementations of
	// smurf protection. This setting marks such addresses as
	// unusable, for maximum compatibility with ancient parts of the
	// internet.
	AvoidBuggyIPs bool
	// If false, prevents IP addresses to be automatically assigned
	// from this pool.
	AutoAssign bool
	// When an IP is allocated from this pool, how should it be
	// translated into BGP announcements?
	BGPAdvertisements []*BGPAdvertisement
	// The L2 network that contains this ARP-advertised pool. Used to
	// make sure we don't allocate the network's base or broadcast
	// addresses.
	ARPNetwork *net.IPNet
}

// BGPAdvertisement describes one translation from an IP address to a BGP advertisement.
type BGPAdvertisement struct {
	// Roll up the IP address into a CIDR prefix of this
	// length. Optional, defaults to 32 (i.e. no aggregation) if not
	// specified.
	AggregationLength int
	// Value of the LOCAL_PREF BGP path attribute. Used only when
	// advertising to IBGP peers (i.e. Peer.MyASN == Peer.ASN).
	LocalPref uint32
	// Value of the COMMUNITIES path attribute.
	Communities map[uint32]bool
}

func parseHoldTime(ht string) (time.Duration, error) {
	if ht == "" {
		return 90 * time.Second, nil
	}
	d, err := time.ParseDuration(ht)
	if err != nil {
		return 0, fmt.Errorf("invalid hold time %q: %s", ht, err)
	}
	rounded := time.Duration(int(d.Seconds())) * time.Second
	if rounded != 0 && rounded < 3*time.Second {
		return 0, fmt.Errorf("invalid hold time %q: must be 0 or >=3s", ht)
	}
	return rounded, nil
}

// Parse loads and validates a Config from bs.
func Parse(bs []byte) (*Config, error) {
	var raw configFile
	if err := yaml.Unmarshal(bs, &raw); err != nil {
		return nil, fmt.Errorf("could not parse config: %s", err)
	}

	cfg := &Config{Pools: map[string]*Pool{}}
	for i, p := range raw.Peers {
		peer, err := parsePeer(p)
		if err != nil {
			return nil, fmt.Errorf("parsing peer #%d: %s", i+1, err)
		}
		cfg.Peers = append(cfg.Peers, peer)
	}

	communities := map[string]uint32{}
	for n, v := range raw.BGPCommunities {
		c, err := parseCommunity(v)
		if err != nil {
			return nil, fmt.Errorf("parsing community %q: %s", n, err)
		}
		communities[n] = c
	}

	var allCIDRs []*net.IPNet
	for i, p := range raw.Pools {
		pool, err := parseAddressPool(p, communities)
		if err != nil {
			return nil, fmt.Errorf("parsing address pool #%d: %s", i+1, err)
		}

		// Check that the pool isn't already defined
		if cfg.Pools[p.Name] != nil {
			return nil, fmt.Errorf("duplicate definition of pool %q", p.Name)
		}

		// Check that all specified CIDR ranges are non-overlapping.
		for _, cidr := range pool.CIDR {
			for _, m := range allCIDRs {
				if cidrsOverlap(cidr, m) {
					return nil, fmt.Errorf("CIDR %q in pool %q overlaps with already defined CIDR %q", cidr, p.Name, m)
				}
			}
			allCIDRs = append(allCIDRs, cidr)
		}

		cfg.Pools[p.Name] = pool
	}

	return cfg, nil
}

func parsePeer(p peer) (*Peer, error) {
	if p.MyASN == 0 {
		return nil, errors.New("missing local ASN")
	}
	if p.ASN == 0 {
		return nil, errors.New("missing peer ASN")
	}
	ip := net.ParseIP(p.Addr)
	if ip == nil {
		return nil, fmt.Errorf("invalid peer IP %q", p.Addr)
	}
	holdTime, err := parseHoldTime(p.HoldTime)
	if err != nil {
		return nil, err
	}
	port := uint16(179)
	if p.Port != 0 {
		port = p.Port
	}
	return &Peer{
		MyASN:    p.MyASN,
		ASN:      p.ASN,
		Addr:     ip,
		Port:     port,
		HoldTime: holdTime,
	}, nil
}

func parseAddressPool(p addressPool, bgpCommunities map[string]uint32) (*Pool, error) {
	if p.Name == "" {
		return nil, errors.New("missing pool name")
	}

	ret := &Pool{
		Protocol:      p.Protocol,
		AvoidBuggyIPs: p.AvoidBuggyIPs,
		AutoAssign:    true,
	}

	if p.AutoAssign != nil {
		ret.AutoAssign = *p.AutoAssign
	}

	if len(p.CIDR) == 0 {
		return nil, errors.New("pool has no prefixes defined")
	}
	for _, cidr := range p.CIDR {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q in pool %q", cidr, p.Name)
		}
		ret.CIDR = append(ret.CIDR, n)
	}

	switch ret.Protocol {
	case ARP:
		if len(p.BGPAdvertisements) > 0 {
			return nil, errors.New("cannot have bgp-advertisements configuration element in an ARP address pool")
		}

		arpNet, err := parseARPNetwork(p.ARPNetwork, ret.CIDR)
		if err != nil {
			return nil, fmt.Errorf("parsing ARP network: %s", err)
		}
		ret.ARPNetwork = arpNet
	case BGP:
		if p.ARPNetwork != "" {
			return nil, errors.New("cannot have arp-network configuration element in a BGP address pool")
		}

		ads, err := parseBGPAdvertisements(p.BGPAdvertisements, ret.CIDR, bgpCommunities)
		if err != nil {
			return nil, fmt.Errorf("parsing BGP communities: %s", err)
		}
		ret.BGPAdvertisements = ads

	case "":
		return nil, errors.New("address pool is missing the protocol field")
	default:
		return nil, fmt.Errorf("unknown protocol %q", ret.Protocol)
	}

	return ret, nil
}

func parseBGPAdvertisements(ads []bgpAdvertisement, cidrs []*net.IPNet, communities map[string]uint32) ([]*BGPAdvertisement, error) {
	if len(ads) == 0 {
		return []*BGPAdvertisement{
			{
				AggregationLength: 32,
				LocalPref:         0,
				Communities:       map[uint32]bool{},
			},
		}, nil
	}

	var ret []*BGPAdvertisement
	for _, rawAd := range ads {
		ad := &BGPAdvertisement{
			AggregationLength: 32,
			LocalPref:         0,
			Communities:       map[uint32]bool{},
		}

		if rawAd.AggregationLength != nil {
			ad.AggregationLength = *rawAd.AggregationLength
		}
		if ad.AggregationLength > 32 {
			return nil, fmt.Errorf("invalid aggregation length %q", ad.AggregationLength)
		}
		for _, cidr := range cidrs {
			o, _ := cidr.Mask.Size()
			if ad.AggregationLength < o {
				return nil, fmt.Errorf("invalid aggregation length %d: prefix %q in this pool is more specific than the aggregation length", ad.AggregationLength, cidr)
			}
		}

		if rawAd.LocalPref != nil {
			ad.LocalPref = *rawAd.LocalPref
		}

		for _, c := range rawAd.Communities {
			if v, ok := communities[c]; ok {
				ad.Communities[v] = true
			} else {
				v, err := parseCommunity(c)
				if err != nil {
					return nil, fmt.Errorf("invalid community %q in BGP advertisement: %s", c, err)
				}
				ad.Communities[v] = true
			}
		}

		ret = append(ret, ad)
	}

	return ret, nil
}

func parseARPNetwork(a string, cidr []*net.IPNet) (*net.IPNet, error) {
	var ret *net.IPNet
	if a == "" {
		ret = &net.IPNet{
			IP:   cidr[0].IP.Mask(net.CIDRMask(24, 32)),
			Mask: net.CIDRMask(24, 32),
		}
	} else {
		_, arpNet, err := net.ParseCIDR(a)
		if err != nil {
			return nil, fmt.Errorf("parsing ARP network: %s", err)
		}
		ret = arpNet
	}

	// All CIDRs must be contained within the arp-network.
	for _, cidr := range cidr {
		if !cidrContainsCIDR(ret, cidr) {
			if a == "" {
				// This validation is failing based on a
				// default-selected value. Make the error point this
				// out to better guide the user.
				return nil, fmt.Errorf("pool did not specify an arp-network, and CIDR %q falls outside the inferred arp-network %q (maybe explicitly define arp-network?)", cidr, ret)
			}
			return nil, fmt.Errorf("CIDR %q is not contained within the enclosing ARP network %q", cidr, ret)
		}
	}

	return ret, nil
}

func parseCommunity(c string) (uint32, error) {
	fs := strings.Split(c, ":")
	if len(fs) != 2 {
		return 0, fmt.Errorf("invalid community string %q", c)
	}
	a, err := strconv.ParseUint(fs[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid first section of community %q: %s", fs[0], err)
	}
	b, err := strconv.ParseUint(fs[1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid second section of community %q: %s", fs[0], err)
	}

	return (uint32(a) << 16) + uint32(b), nil
}

func cidrsOverlap(a, b *net.IPNet) bool {
	return cidrContainsCIDR(a, b) || cidrContainsCIDR(b, a)
}

func cidrContainsCIDR(outer, inner *net.IPNet) bool {
	ol, _ := outer.Mask.Size()
	il, _ := inner.Mask.Size()
	if ol == il && outer.IP.Equal(inner.IP) {
		return true
	}
	if ol < il && outer.Contains(inner.IP) {
		return true
	}
	return false
}
