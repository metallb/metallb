package main

import (
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/osrg/gobgp/config"
)

func main() {
	b := config.Bgp{
		Global: config.Global{
			Config: config.GlobalConfig{
				As:       12332,
				RouterId: "10.0.0.1",
			},
		},
		Neighbors: []config.Neighbor{
			config.Neighbor{
				Config: config.NeighborConfig{
					PeerAs:          12333,
					AuthPassword:    "apple",
					NeighborAddress: "192.168.177.33",
				},
				AfiSafis: []config.AfiSafi{
					config.AfiSafi{
						Config: config.AfiSafiConfig{
							AfiSafiName: "ipv4-unicast",
						},
					},
					config.AfiSafi{
						Config: config.AfiSafiConfig{
							AfiSafiName: "ipv6-unicast",
						},
					},
				},
				ApplyPolicy: config.ApplyPolicy{

					Config: config.ApplyPolicyConfig{
						ImportPolicyList:    []string{"pd1"},
						DefaultImportPolicy: config.DEFAULT_POLICY_TYPE_ACCEPT_ROUTE,
					},
				},
			},

			config.Neighbor{
				Config: config.NeighborConfig{
					PeerAs:          12334,
					AuthPassword:    "orange",
					NeighborAddress: "192.168.177.32",
				},
			},

			config.Neighbor{
				Config: config.NeighborConfig{
					PeerAs:          12335,
					AuthPassword:    "grape",
					NeighborAddress: "192.168.177.34",
				},
			},
		},
	}

	var buffer bytes.Buffer
	encoder := toml.NewEncoder(&buffer)
	err := encoder.Encode(b)
	if err != nil {
		panic(err)
	}

	err = encoder.Encode(policy())
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v\n", buffer.String())
}

func policy() config.RoutingPolicy {

	ps := config.PrefixSet{
		PrefixSetName: "ps1",
		PrefixList: []config.Prefix{
			config.Prefix{
				IpPrefix:        "10.3.192.0/21",
				MasklengthRange: "21..24",
			}},
	}

	ns := config.NeighborSet{
		NeighborSetName:  "ns1",
		NeighborInfoList: []string{"10.0.0.2"},
	}

	cs := config.CommunitySet{
		CommunitySetName: "community1",
		CommunityList:    []string{"65100:10"},
	}

	ecs := config.ExtCommunitySet{
		ExtCommunitySetName: "ecommunity1",
		ExtCommunityList:    []string{"RT:65001:200"},
	}

	as := config.AsPathSet{
		AsPathSetName: "aspath1",
		AsPathList:    []string{"^65100"},
	}

	bds := config.BgpDefinedSets{
		CommunitySets:    []config.CommunitySet{cs},
		ExtCommunitySets: []config.ExtCommunitySet{ecs},
		AsPathSets:       []config.AsPathSet{as},
	}

	ds := config.DefinedSets{
		PrefixSets:     []config.PrefixSet{ps},
		NeighborSets:   []config.NeighborSet{ns},
		BgpDefinedSets: bds,
	}

	al := config.AsPathLength{
		Operator: "eq",
		Value:    2,
	}

	s := config.Statement{
		Name: "statement1",
		Conditions: config.Conditions{

			MatchPrefixSet: config.MatchPrefixSet{
				PrefixSet:       "ps1",
				MatchSetOptions: config.MATCH_SET_OPTIONS_RESTRICTED_TYPE_ANY,
			},

			MatchNeighborSet: config.MatchNeighborSet{
				NeighborSet:     "ns1",
				MatchSetOptions: config.MATCH_SET_OPTIONS_RESTRICTED_TYPE_ANY,
			},

			BgpConditions: config.BgpConditions{
				MatchCommunitySet: config.MatchCommunitySet{
					CommunitySet:    "community1",
					MatchSetOptions: config.MATCH_SET_OPTIONS_TYPE_ANY,
				},

				MatchExtCommunitySet: config.MatchExtCommunitySet{
					ExtCommunitySet: "ecommunity1",
					MatchSetOptions: config.MATCH_SET_OPTIONS_TYPE_ANY,
				},

				MatchAsPathSet: config.MatchAsPathSet{
					AsPathSet:       "aspath1",
					MatchSetOptions: config.MATCH_SET_OPTIONS_TYPE_ANY,
				},
				AsPathLength: al,
			},
		},
		Actions: config.Actions{
			RouteDisposition: "reject-route",
			BgpActions: config.BgpActions{
				SetCommunity: config.SetCommunity{
					SetCommunityMethod: config.SetCommunityMethod{
						CommunitiesList: []string{"65100:20"},
					},
					Options: "ADD",
				},
				SetMed: "-200",
			},
		},
	}

	pd := config.PolicyDefinition{
		Name:       "pd1",
		Statements: []config.Statement{s},
	}

	p := config.RoutingPolicy{
		DefinedSets:       ds,
		PolicyDefinitions: []config.PolicyDefinition{pd},
	}

	return p
}
