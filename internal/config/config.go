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
	Protocol Proto
	Peers    []struct {
		MyASN    uint32 `yaml:"my-asn"`
		ASN      uint32 `yaml:"peer-asn"`
		Addr     string `yaml:"peer-address"`
		Port     uint16 `yaml:"peer-port"`
		HoldTime string `yaml:"hold-time"`
	}
	Communities map[string]string
	Pools       []struct {
		Name           string
		CIDR           []string
		AvoidBuggyIPs  bool `yaml:"avoid-buggy-ips"`
		AutoAssign     *bool `yaml:"auto-assign"`
		Advertisements []struct {
			AggregationLength *int `yaml:"aggregation-length"`
			LocalPref         *uint32
			Communities       []string
		}
	} `yaml:"address-pools"`
}

// Config is a parsed MetalLB configuration.
type Config struct {
	// Protocol that MetalLB should use, supported values "arp", "bgp" and "rip".
	Protocol Proto
	// Routers that MetalLB should peer with.
	Peers []*Peer
	// Address pools from which to allocate load balancer IPs.
	Pools map[string]*Pool
}

type Proto string

const (
	ProtoARP Proto = "arp"
	ProtoBGP Proto = "bgp"
	ProtoRIP Proto = "rip"
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
	Advertisements []*Advertisement
}

// Advertisement describes one translation from an IP address to a BGP advertisement.
type Advertisement struct {
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
	if err := yaml.Unmarshal([]byte(bs), &raw); err != nil {
		return nil, fmt.Errorf("could not parse config: %s", err)
	}

	cfg := &Config{
		Protocol: ProtoBGP,
		Pools:    map[string]*Pool{},
	}
	switch raw.Protocol {
	case "arp":
		cfg.Protocol = ProtoARP
	case "bgp":
		cfg.Protocol = ProtoBGP
	case "rip":
		cfg.Protocol = ProtoRIP
	case "":
		// Not set default to BGP.
	default:
		return nil, fmt.Errorf("wrong value for protocol %s", raw.Protocol)
	}

	for i, p := range raw.Peers {
		if p.MyASN == 0 {
			return nil, fmt.Errorf("peer #%d missing local ASN", i+1)
		}
		if p.ASN == 0 {
			return nil, fmt.Errorf("peer #%d missing peer ASN", i+1)
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
		cfg.Peers = append(cfg.Peers, &Peer{
			MyASN:    p.MyASN,
			ASN:      p.ASN,
			Addr:     ip,
			Port:     port,
			HoldTime: holdTime,
		})
	}

	communities := map[string]uint32{}
	for n, v := range raw.Communities {
		c, err := parseCommunity(v)
		if err != nil {
			return nil, fmt.Errorf("parsing community %q: %s", n, err)
		}
		communities[n] = c
	}

	var allCIDRs []*net.IPNet
	for i, p := range raw.Pools {
		if p.Name == "" {
			return nil, fmt.Errorf("address pool #%d is missing name", i+1)
		}
		if _, ok := cfg.Pools[p.Name]; ok {
			return nil, fmt.Errorf("duplicate pool definition for %q", p.Name)
		}

		autoAssign := true
		if p.AutoAssign != nil {
			autoAssign = *p.AutoAssign
		}
		pool := &Pool{
			AvoidBuggyIPs: p.AvoidBuggyIPs,
			AutoAssign:    autoAssign,
		}
		cfg.Pools[p.Name] = pool

		for _, cidr := range p.CIDR {
			_, n, err := net.ParseCIDR(cidr)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q in pool %q", cidr, p.Name)
			}
			for _, m := range allCIDRs {
				if cidrsOverlap(n, m) {
					return nil, fmt.Errorf("CIDR %q in pool %q overlaps with already defined CIDR %q", n, p.Name, m)
				}
			}
			pool.CIDR = append(pool.CIDR, n)
			allCIDRs = append(allCIDRs, n)
		}

		for _, ad := range p.Advertisements {
			// TODO: ipv6 support :(
			agLen := 32
			if ad.AggregationLength != nil {
				agLen = *ad.AggregationLength
			}
			if agLen > 32 {
				return nil, fmt.Errorf("invalid aggregation length %q in pool %q", ad.AggregationLength, p.Name)
			}
			for _, cidr := range pool.CIDR {
				o, _ := cidr.Mask.Size()
				if agLen < o {
					return nil, fmt.Errorf("invalid aggregation length %d in pool %q: prefix %q in this pool is more specific than the aggregation length", ad.AggregationLength, p.Name, cidr)
				}
			}

			comms := map[uint32]bool{}
			for _, c := range ad.Communities {
				if v, ok := communities[c]; ok {
					comms[v] = true
				} else {
					v, err := parseCommunity(c)
					if err != nil {
						return nil, fmt.Errorf("invalid community %q in advertisement of pool %q: %s", c, p.Name, err)
					}
					comms[v] = true
				}
			}

			localPref := uint32(0)
			if ad.LocalPref != nil {
				localPref = *ad.LocalPref
			}

			pool.Advertisements = append(pool.Advertisements, &Advertisement{
				AggregationLength: agLen,
				LocalPref:         localPref,
				Communities:       comms,
			})
		}
	}

	return cfg, nil
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
	al, _ := a.Mask.Size()
	bl, _ := b.Mask.Size()
	if al == bl && a.IP.Equal(b.IP) {
		return true
	}
	if al > bl && b.Contains(a.IP) {
		return true
	}
	if bl > al && a.Contains(b.IP) {
		return true
	}
	return false
}
