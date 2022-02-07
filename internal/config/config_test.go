// SPDX-License-Identifier:Apache-2.0

package config

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/labels"
)

func selector(s string) labels.Selector {
	ret, err := labels.Parse(s)
	if err != nil {
		panic(err)
	}
	return ret
}

func ipnet(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func TestParse(t *testing.T) {
	tests := []struct {
		desc string
		raw  string
		want *Config
	}{
		{
			desc: "empty config",
			raw:  "",
			want: &Config{
				Pools:       map[string]*Pool{},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "invalid yaml",
			raw:  "foo:<>$@$2r24j90",
		},

		{
			desc: "config using all features",
			raw: `
peers:
- my-asn: 42
  peer-asn: 142
  peer-address: 1.2.3.4
  peer-port: 1179
  hold-time: 180s
  router-id: 10.20.30.40
  source-address: 10.20.30.40
  ebgp-multihop: true
- my-asn: 100
  peer-asn: 200
  peer-address: 2.3.4.5
  ebgp-multihop: false
  node-selectors:
  - match-labels:
      foo: bar
    match-expressions:
      - {key: bar, operator: In, values: [quux]}
bgp-communities:
  bar: 64512:1234
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 10.20.0.0/16
  - 10.50.0.0/24
  avoid-buggy-ips: true
  auto-assign: false
  bgp-advertisements:
  - aggregation-length: 32
    localpref: 100
    communities: ["bar", "1234:2345"]
  - aggregation-length: 24
    aggregation-length-v6: 64
- name: pool2
  protocol: bgp
  addresses:
  - 30.0.0.0/8
- name: pool3
  protocol: layer2
  addresses:
  - 40.0.0.0/25
  - 40.0.0.150-40.0.0.200
  - 40.0.0.210 - 40.0.0.240
  - 40.0.0.250 - 40.0.0.250
- name: pool4
  protocol: layer2
  addresses:
  - 2001:db8::/64
`,
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           142,
						Addr:          net.ParseIP("1.2.3.4"),
						SrcAddr:       net.ParseIP("10.20.30.40"),
						Port:          1179,
						HoldTime:      180 * time.Second,
						KeepaliveTime: 60 * time.Second,
						RouterID:      net.ParseIP("10.20.30.40"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  true,
					},
					{
						MyASN:         100,
						ASN:           200,
						Addr:          net.ParseIP("2.3.4.5"),
						Port:          179,
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{selector("bar in (quux),foo=bar")},
						EBGPMultiHop:  false,
					},
				},
				Pools: map[string]*Pool{
					"pool1": {
						Protocol:      BGP,
						CIDR:          []*net.IPNet{ipnet("10.20.0.0/16"), ipnet("10.50.0.0/24")},
						AvoidBuggyIPs: true,
						AutoAssign:    false,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities: map[uint32]bool{
									0xfc0004d2: true,
									0x04D20929: true,
								},
							},
							{
								AggregationLength:   24,
								AggregationLengthV6: 64,
								Communities:         map[uint32]bool{},
							},
						},
					},
					"pool2": {
						Protocol:   BGP,
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/8")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
							},
						},
					},
					"pool3": {
						Protocol: Layer2,
						CIDR: []*net.IPNet{
							ipnet("40.0.0.0/25"),
							ipnet("40.0.0.150/31"),
							ipnet("40.0.0.152/29"),
							ipnet("40.0.0.160/27"),
							ipnet("40.0.0.192/29"),
							ipnet("40.0.0.200/32"),
							ipnet("40.0.0.210/31"),
							ipnet("40.0.0.212/30"),
							ipnet("40.0.0.216/29"),
							ipnet("40.0.0.224/28"),
							ipnet("40.0.0.240/32"),
							ipnet("40.0.0.250/32"),
						},
						AutoAssign: true,
					},
					"pool4": {
						Protocol:   Layer2,
						CIDR:       []*net.IPNet{ipnet("2001:db8::/64")},
						AutoAssign: true,
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "peer-only",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
`,
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						Port:          179,
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools:       map[string]*Pool{},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "invalid peer-address",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.400
`,
		},

		{
			desc: "invalid my-asn",
			raw: `
peers:
- peer-asn: 42
  peer-address: 1.2.3.4
`,
		},

		{
			desc: "invalid peer-asn",
			raw: `
peers:
- my-asn: 42
  peer-address: 1.2.3.4
`,
		},

		{
			desc: "invalid ebgp-multihop",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  ebgp-multihop: true
`,
		},

		{
			desc: "invalid hold time (wrong format)",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  hold-time: foo
`,
		},

		{
			desc: "invalid hold time (too short)",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  hold-time: 1s
`,
		},
		{
			desc: "invalid router ID",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  router-id: oh god how do I BGP
`,
		},

		{
			desc: "empty node selector (select everything)",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
`,
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						Port:          179,
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools:       map[string]*Pool{},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "invalid label node selector shape",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  node-selectors:
  - match-labels:
      foo:
        bar: baz
`,
		},

		{
			desc: "invalid expression node selector (missing key)",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  node-selectors:
  - match-expressions:
    - operator: In
      values: [foo, bar]
`,
		},

		{
			desc: "invalid expression node selector (missing operator)",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  node-selectors:
  - match-expressions:
    - key: foo
      values: [foo, bar]
`,
		},

		{
			desc: "invalid expression node selector (invalid operator)",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  node-selectors:
  - match-expressions:
    - key: foo
      operator: Surrounds
      values: [foo, bar]
`,
		},

		{
			desc: "invalid router ID",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  router-id: oh god how do I BGP
`,
		},

		{
			desc: "duplicate peers",
			raw: `
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
`,
		},

		{
			desc: "no pool name",
			raw: `
address-pools:
-
`,
		},

		{
			desc: "address pool with no addresses",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
`,
		},

		{
			desc: "address pool with no protocol",
			raw: `
address-pools:
- name: pool1
`,
		},

		{
			desc: "address pool with unknown protocol",
			raw: `
address-pools:
- name: pool1
  protocol: babel
`,
		},

		{
			desc: "invalid pool CIDR",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 100.200.300.400/24
`,
		},

		{
			desc: "invalid pool CIDR prefix length",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 1.2.3.0/33
`,
		},

		{
			desc: "invalid pool CIDR, first address of the range is after the second",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 1.2.3.10-1.2.3.1
`,
		},

		{
			desc: "simple advertisement",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses: ["1.2.3.0/24"]
  bgp-advertisements:
  -
`,
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						Protocol:   BGP,
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "advertisement with default BGP settings",
			raw: `
address-pools:
- name: pool1
  addresses: ["1.2.3.0/24"]
  protocol: bgp
`,
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						Protocol:   BGP,
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "bad aggregation length (too long)",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  bgp-advertisements:
  - aggregation-length: 33
`,
		},

		{
			desc: "bad aggregation length (incompatible with CIDR)",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 10.20.30.40/24
  - 1.2.3.0/28
  bgp-advertisements:
  - aggregation-length: 26
`,
		},

		{
			desc: "aggregation length by range",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 3.3.3.2-3.3.3.254
  bgp-advertisements:
  - aggregation-length: 26
`,
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						Protocol:   BGP,
						AutoAssign: true,
						CIDR: []*net.IPNet{
							ipnet("3.3.3.2/31"),
							ipnet("3.3.3.4/30"),
							ipnet("3.3.3.8/29"),
							ipnet("3.3.3.16/28"),
							ipnet("3.3.3.32/27"),
							ipnet("3.3.3.64/26"),
							ipnet("3.3.3.128/26"),
							ipnet("3.3.3.192/27"),
							ipnet("3.3.3.224/28"),
							ipnet("3.3.3.240/29"),
							ipnet("3.3.3.248/30"),
							ipnet("3.3.3.252/31"),
							ipnet("3.3.3.254/32"),
						},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   26,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "aggregation length by range, too wide",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 3.3.3.2-3.3.3.254
  bgp-advertisements:
  - aggregation-length: 24
`,
		},

		{
			desc: "bad community literal (wrong format)",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  bgp-advertisements:
  - communities: ["1234"]
`,
		},

		{
			desc: "bad community literal (asn part doesn't fit)",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  bgp-advertisements:
  - communities: ["99999999:1"]
`,
		},

		{
			desc: "bad community literal (community# part doesn't fit)",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  bgp-advertisements:
  - communities: ["1:99999999"]
`,
		},

		{
			desc: "bad community ref (unknown ref)",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  bgp-advertisements:
  - communities: ["flarb"]
`,
		},

		{
			desc: "bad community ref (ref asn doesn't fit)",
			raw: `
bgp-communities:
  flarb: 99999999:1
address-pools:
- name: pool1
  protocol: bgp
  bgp-advertisements:
  - communities: ["flarb"]
`,
		},

		{
			desc: "bad community ref (ref community# doesn't fit)",
			raw: `
bgp-communities:
  flarb: 1:99999999
address-pools:
- name: pool1
  protocol: bgp
  bgp-advertisements:
  - communities: ["flarb"]
`,
		},

		{
			desc: "duplicate pool definition",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
- name: pool1
  protocol: bgp
- name: pool2
  protocol: bgp
`,
		},

		{
			desc: "duplicate CIDRs",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 10.0.0.0/8
- name: pool2
  protocol: bgp
  addresses:
  - 10.0.0.0/8
`,
		},

		{
			desc: "overlapping CIDRs",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 10.0.0.0/8
- name: pool2
  protocol: bgp
  addresses:
  - 10.0.0.0/16
`,
		},

		{
			desc: "BGP advertisements in layer2 pool",
			raw: `
address-pools:
- name: pool1
  protocol: layer2
  addresses:
  - 10.0.0.0/16
  bgp-advertisements:
  - communities: ["flarb"]
`,
		},
		{
			desc: "Session with default BFD Profile",
			raw: `
address-pools:
- name: pool1
  addresses: ["1.2.3.0/24"]
  protocol: bgp
bfd-profiles:
- name: default
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  bfd-profile: default
`,
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						Port:          179,
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{labels.Everything()},
						BFDProfile:    "default",
					},
				},
				Pools: map[string]*Pool{
					"pool1": {
						Protocol:   BGP,
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{
					"default": {
						Name: "default",
					},
				},
			},
		},
		{
			desc: "Peer with non existing BFD Profile",
			raw: `
address-pools:
- name: pool1
  addresses: ["1.2.3.0/24"]
  protocol: bgp
bfd-profiles:
- name: default
peers:
- my-asn: 42
  peer-asn: 42
  peer-address: 1.2.3.4
  bfd-profile: zzz
`,
		},
		{
			desc: "Multiple BFD Profiles with the same name",
			raw: `
address-pools:
- name: pool1
  addresses: ["1.2.3.0/24"]
  protocol: bgp
bfd-profiles:
- name: default
- name: foo
- name: foo
`,
		},
		{
			desc: "Session with nondefault BFD Profile",
			raw: `
address-pools:
- name: pool1
  addresses: ["1.2.3.0/24"]
  protocol: bgp
bfd-profiles:
- name: nondefault
  receive-interval: 50
  transmit-interval: 51
  detect-multiplier: 52
  echo-interval: 54
  echo-mode: true
  passive-mode: true
  minimum-ttl: 55
`,
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						Protocol:   BGP,
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{
					"nondefault": {
						Name:             "nondefault",
						ReceiveInterval:  uint32Ptr(50),
						DetectMultiplier: uint32Ptr(52),
						TransmitInterval: uint32Ptr(51),
						EchoInterval:     uint32Ptr(54),
						MinimumTTL:       uint32Ptr(55),
						EchoMode:         true,
						PassiveMode:      true,
					},
				},
			},
		},
		{
			desc: "BFD Profile with too low receive interval",
			raw: `
bfd-profiles:
- name: default
  receive-interval: 2
`,
		},
		{
			desc: "BFD Profile with too high range receive interval",
			raw: `
bfd-profiles:
- name: default
  receive-interval: 90000
`,
		},
		{
			desc: "Duplicate bgp advertisment definition",
			raw: `
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 10.20.0.0/16
  bgp-advertisements:
  - aggregation-length: 26
  - aggregation-length: 26
`,
		},
		{
			desc: "Duplicate communities definition",
			raw: `
bgp-communities:
  bar: 64512:1234
address-pools:
- name: pool1
  protocol: bgp
  addresses:
  - 10.20.0.0/16
  bgp-advertisements:
  - communities: ["bar", "bar"]
`,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, err := Parse([]byte(test.raw), DontValidate)
			if err != nil && test.want != nil {
				t.Errorf("%q: parse failed: %s", test.desc, err)
				return
			}
			if test.want == nil && err == nil {
				t.Errorf("%q: parse unexpectedly succeeded", test.desc)
				return
			}
			selectorComparer := cmp.Comparer(func(x, y labels.Selector) bool {
				if x == nil {
					return y == nil
				}
				if y == nil {
					return x == nil
				}
				// Nothing() and Everything() have the same string
				// representation, stupidly. So, compare explicitly for
				// Nothing.
				if x == labels.Nothing() {
					return y == labels.Nothing()
				}
				if y == labels.Nothing() {
					return x == labels.Nothing()
				}
				return x.String() == y.String()
			})
			if diff := cmp.Diff(test.want, got, selectorComparer); diff != "" {
				t.Errorf("%q: parse returned wrong result (-want, +got)\n%s", test.desc, diff)
			}
		})
	}
}

func uint32Ptr(n uint32) *uint32 {
	return &n
}
