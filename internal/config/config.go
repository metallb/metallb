package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

// configFile is the configuration as parsed out of the ConfigMap,
// without validation or useful high level types.
type configFile struct {
	Peers []struct {
		MyASN uint32 `yaml:"my-asn"`
		ASN   uint32 `yaml:"peer-asn"`
		Addr  string `yaml:"peer-address"`
	}
	Communities map[string]string
	Pools       []struct {
		Name           string
		CIDR           []string
		Advertisements []struct {
			AggregationLength int `yaml:"aggregation-length"`
			LocalPref         uint32
			Communities       []string
		}
	} `yaml:"address-pools"`
}

type Config struct {
	Peers []Peer
	Pools map[string]*Pool
}

type Peer struct {
	MyASN uint32
	ASN   uint32
	Addr  net.IP
	// TODO: BGP session settings
}

type Pool struct {
	CIDR           []*net.IPNet
	Advertisements []Advertisement
}

type Advertisement struct {
	AggregationLength int
	LocalPref         uint32
	Communities       map[uint32]bool
}

func Parse(bs []byte) (*Config, error) {
	var raw configFile
	if err := yaml.Unmarshal([]byte(bs), &raw); err != nil {
		return nil, fmt.Errorf("could not parse config: %s", err)
	}

	cfg := &Config{
		Pools: map[string]*Pool{},
	}
	for _, p := range raw.Peers {
		ip := net.ParseIP(p.Addr)
		if ip == nil {
			return nil, fmt.Errorf("invalid peer IP %q", p.Addr)
		}
		cfg.Peers = append(cfg.Peers, Peer{
			MyASN: p.MyASN,
			ASN:   p.ASN,
			Addr:  ip,
		})
	}

	communities := map[string]uint32{}
	for n, v := range raw.Communities {
		c, err := parseCommunity(v)
		if err != nil {
			return nil, fmt.Errorf("parsing community %q: %s", n, err)
		}
		if _, ok := communities[n]; ok {
			return nil, fmt.Errorf("duplicate community definition for %q", n)
		}
		communities[n] = c
	}

	var allCIDRs []*net.IPNet
	for _, p := range raw.Pools {
		if _, ok := cfg.Pools[p.Name]; ok {
			return nil, fmt.Errorf("duplicate pool definition for %q", p.Name)
		}
		pool := &Pool{}
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
			if ad.AggregationLength > 32 {
				return nil, fmt.Errorf("invalid aggregation length %q in pool %q", ad.AggregationLength, p.Name)
			}
			comms := map[uint32]bool{}
			for _, c := range ad.Communities {
				if v, ok := communities[c]; ok {
					comms[v] = true
				} else {
					v, err := parseCommunity(c)
					if err != nil {
						return nil, fmt.Errorf("invalid community %q in advertisement of pool %q", c, p.Name)
					}
					comms[v] = true
				}
			}
			pool.Advertisements = append(pool.Advertisements, Advertisement{
				AggregationLength: ad.AggregationLength,
				LocalPref:         ad.LocalPref,
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
