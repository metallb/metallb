// SPDX-License-Identifier:Apache-2.0

package config

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		desc     string
		config   *configFile
		mustFail bool
	}{
		{
			desc: "peer with bfd profile",
			config: &configFile{
				Peers: []peer{
					{
						Addr:       "1.2.3.4",
						BFDProfile: "foo",
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "bfd profile set",
			config: &configFile{
				Peers: []peer{
					{
						Addr: "1.2.3.4",
					},
				},
				BFDProfiles: []bfdProfile{
					{
						Name: "default",
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "v6 address",
			config: &configFile{
				Pools: []addressPool{
					{
						Protocol:  BGP,
						Addresses: []string{"2001:db8::/64"},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "keepalive time",
			config: &configFile{
				Peers: []peer{
					{
						KeepaliveTime: "1s",
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "aggregation v6",
			config: &configFile{
				Pools: []addressPool{
					{
						Name: "foo",
						BGPAdvertisements: []bgpAdvertisement{
							{
								AggregationLengthV6: intPtr(47),
							},
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "should pass",
			config: &configFile{
				Peers: []peer{
					{
						Addr: "1.2.3.4",
					},
				},
			},
			mustFail: false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := DiscardFRROnly(test.config)
			if test.mustFail && err == nil {
				t.Fatalf("Expected error for %s", test.desc)
			}
			if !test.mustFail && err != nil {
				t.Fatalf("Not expected error %s for %s", err, test.desc)
			}
		})
	}
}

func TestValidateFRR(t *testing.T) {
	tests := []struct {
		desc     string
		config   *configFile
		mustFail bool
	}{
		{
			desc: "peer with routerid",
			config: &configFile{
				Peers: []peer{
					{
						Addr:     "1.2.3.4",
						RouterID: "1.2.3.4",
					},
				},
			},
		},
		{
			desc: "routerid set, one different",
			config: &configFile{
				Peers: []peer{
					{
						Addr:     "1.2.3.4",
						RouterID: "1.2.3.4",
					},
					{
						Addr:     "1.2.3.5",
						RouterID: "1.2.3.4",
					},
					{
						Addr:     "1.2.3.6",
						RouterID: "1.2.3.5",
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "routerid set, one not set",
			config: &configFile{
				Peers: []peer{
					{
						Addr:     "1.2.3.4",
						RouterID: "1.2.3.4",
					},
					{
						Addr:     "1.2.3.5",
						RouterID: "1.2.3.4",
					},
					{
						Addr: "1.2.3.6",
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "bfd profile set",
			config: &configFile{
				Peers: []peer{
					{
						Addr: "1.2.3.4",
					},
				},
				BFDProfiles: []bfdProfile{
					{
						Name: "default",
					},
				},
			},
		},
		{
			desc: "myAsn set, all equals",
			config: &configFile{
				Peers: []peer{
					{
						Addr:  "1.2.3.4",
						MyASN: 123,
					},
					{
						Addr:  "1.2.3.5",
						MyASN: 123,
					},
					{
						Addr:  "1.2.3.6",
						MyASN: 123,
					},
				},
			},
		},
		{
			desc: "myAsn set, one different",
			config: &configFile{
				Peers: []peer{
					{
						Addr:  "1.2.3.4",
						MyASN: 123,
					},
					{
						Addr:  "1.2.3.5",
						MyASN: 123,
					},
					{
						Addr:  "1.2.3.6",
						MyASN: 125,
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "duplicate BGPPeer address",
			config: &configFile{
				Peers: []peer{
					{
						Addr: "1.2.3.4",
					},
					{
						Addr: "1.2.3.4",
					},
				},
			},
			mustFail: true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := DiscardNativeOnly(test.config)
			if test.mustFail && err == nil {
				t.Fatalf("Expected error for %s", test.desc)
			}
			if !test.mustFail && err != nil {
				t.Fatalf("Not expected error %s for %s", err, test.desc)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
