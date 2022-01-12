// SPDX-License-Identifier:Apache-2.0

package config

// Proto holds the protocol we are speaking.
type Proto string

// MetalLB supported protocols.
const (
	BGP    Proto = "bgp"
	Layer2 Proto = "layer2"
)

// configFile is the configuration as parsed out of the ConfigMap,
// without validation or useful high level types.
type File struct {
	Peers          []Peer            `yaml:"peers,omitempty"`
	BGPCommunities map[string]string `yaml:"bgp-communities,omitempty"`
	Pools          []AddressPool     `yaml:"address-pools,omitempty"`
	BFDProfiles    []BfdProfile      `yaml:"bfd-profiles"`
}

type Peer struct {
	MyASN         uint32         `yaml:"my-asn,omitempty"`
	ASN           uint32         `yaml:"peer-asn,omitempty"`
	Addr          string         `yaml:"peer-address,omitempty"`
	SrcAddr       string         `yaml:"source-address,omitempty"`
	Port          uint16         `yaml:"peer-port,omitempty"`
	HoldTime      string         `yaml:"hold-time,omitempty"`
	KeepaliveTime string         `yaml:"keepalive-time,omitempty"`
	NodeSelectors []NodeSelector `yaml:"node-selectors,omitempty"`
	Password      string         `yaml:"password,omitempty"`
	BFDProfile    string         `yaml:"bfd-profile,omitempty"`
	EBGPMultiHop  bool           `yaml:"ebgp-multihop,omitempty"`
}

type NodeSelector struct {
	MatchLabels      map[string]string      `yaml:"match-labels,omitempty"`
	MatchExpressions []SelectorRequirements `yaml:"match-expressions,omitempty"`
}

type SelectorRequirements struct {
	Key      string   `yaml:"key,omitempty"`
	Operator string   `yaml:"operator,omitempty"`
	Values   []string `yaml:"values,omitempty"`
}

type AddressPool struct {
	Protocol          Proto              `yaml:"protocol,omitempty"`
	Name              string             `yaml:"name,omitempty"`
	Addresses         []string           `yaml:"addresses,omitempty"`
	AvoidBuggyIPs     bool               `yaml:"avoid-buggy-ips,omitempty"`
	AutoAssign        *bool              `yaml:"auto-assign,omitempty"`
	BGPAdvertisements []BgpAdvertisement `yaml:"bgp-advertisements,omitempty"`
}

type BgpAdvertisement struct {
	AggregationLength *int     `yaml:"aggregation-length,omitempty"`
	LocalPref         *uint32  `yaml:"localpref,omitempty"`
	Communities       []string `yaml:"communities,omitempty"`
}

type BfdProfile struct {
	Name             string  `yaml:"name"`
	ReceiveInterval  *uint32 `yaml:"receive-interval,omitempty"`
	TransmitInterval *uint32 `yaml:"transmit-interval,omitempty"`
	DetectMultiplier *uint32 `yaml:"detect-multiplier,omitempty"`
	EchoInterval     *uint32 `yaml:"echo-interval,omitempty"`
	EchoMode         *bool   `yaml:"echo-mode,omitempty"`
	PassiveMode      *bool   `yaml:"passive-mode,omitempty"`
	MinimumTTL       *uint32 `yaml:"minimum-ttl,omitempty"`
}

func BFDProfileWithDefaults(profile BfdProfile, multiHop bool) BfdProfile {
	res := BfdProfile{}
	res.Name = profile.Name
	res.ReceiveInterval = valueWithDefault(profile.ReceiveInterval, 300)
	res.TransmitInterval = valueWithDefault(profile.TransmitInterval, 300)
	res.DetectMultiplier = valueWithDefault(profile.DetectMultiplier, 3)
	res.EchoInterval = valueWithDefault(profile.EchoInterval, 50)
	res.MinimumTTL = valueWithDefault(profile.MinimumTTL, 254)
	res.EchoMode = profile.EchoMode
	res.PassiveMode = profile.PassiveMode

	if multiHop {
		res.EchoMode = boolPtr(false)
		res.EchoInterval = uint32Ptr(50)
	}

	return res
}

func valueWithDefault(v *uint32, def uint32) *uint32 {
	if v != nil {
		return v
	}
	return &def
}

func uint32Ptr(n uint32) *uint32 {
	return &n
}

func boolPtr(b bool) *bool {
	return &b
}
