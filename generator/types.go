// SPDX-License-Identifier:Apache-2.0

package main

type configFile struct {
	Peers          []peer
	BGPCommunities map[string]string `json:"bgp-communities"`
	Pools          []addressPool     `json:"address-pools"`
	BFDProfiles    []bfdProfile      `json:"bfd-profiles"`
}

type peer struct {
	MyASN         uint32         `json:"my-asn"`
	ASN           uint32         `json:"peer-asn"`
	Addr          string         `json:"peer-address"`
	SrcAddr       string         `json:"source-address"`
	Port          uint16         `json:"peer-port"`
	HoldTime      string         `json:"hold-time"`
	KeepaliveTime string         `json:"keepalive-time"`
	RouterID      string         `json:"router-id"`
	NodeSelectors []nodeSelector `json:"node-selectors"`
	Password      string         `json:"password"`
	BFDProfile    string         `json:"bfd-profile"`
	EBGPMultiHop  bool           `json:"ebgp-multihop"`
}

type nodeSelector struct {
	MatchLabels      map[string]string      `json:"match-labels"`
	MatchExpressions []selectorRequirements `json:"match-expressions"`
}

type selectorRequirements struct {
	Key      string
	Operator string
	Values   []string
}

type addressPool struct {
	Protocol          Proto              `json:"protocol"`
	Name              string             `json:"name"`
	Addresses         []string           `json:"addresses"`
	AutoAssign        *bool              `json:"auto-assign"`
	AvoidBuggyIPs     *bool              `json:"avoid-buggy-ips"`
	BGPAdvertisements []bgpAdvertisement `json:"bgp-advertisements"`
}

// Proto holds the protocol we are speaking.
type Proto string

// MetalLB supported protocols.
const (
	BGP    Proto = "bgp"
	Layer2 Proto = "layer2"
)

type bgpAdvertisement struct {
	AggregationLength   *int32   `json:"aggregation-length"`
	AggregationLengthV6 *int32   `json:"aggregation-length-v6"`
	LocalPref           uint32   `json:"localpref"`
	Communities         []string `json:"communities"`
}

type bfdProfile struct {
	Name             string  `json:"name"`
	ReceiveInterval  *uint32 `json:"receive-interval"`
	TransmitInterval *uint32 `json:"transmit-interval"`
	DetectMultiplier *uint32 `json:"detect-multiplier"`
	EchoInterval     *uint32 `json:"echo-interval"`
	EchoMode         bool    `json:"echo-mode"`
	PassiveMode      bool    `json:"passive-mode"`
	MinimumTTL       *uint32 `json:"minimum-ttl"`
}
