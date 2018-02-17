// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

type ExtCommType int

const (
	ACCEPT ExtCommType = iota
	DISCARD
	RATE
	REDIRECT
	MARK
	ACTION
	RT
	ENCAP
	ESI_LABEL
	ROUTER_MAC
	DEFAULT_GATEWAY
	VALID
	NOT_FOUND
	INVALID
)

var ExtCommNameMap = map[ExtCommType]string{
	ACCEPT:          "accept",
	DISCARD:         "discard",
	RATE:            "rate-limit",
	REDIRECT:        "redirect",
	MARK:            "mark",
	ACTION:          "action",
	RT:              "rt",
	ENCAP:           "encap",
	ESI_LABEL:       "esi-label",
	ROUTER_MAC:      "router-mac",
	DEFAULT_GATEWAY: "default-gateway",
	VALID:           "valid",
	NOT_FOUND:       "not-found",
	INVALID:         "invalid",
}

var ExtCommValueMap = map[string]ExtCommType{
	ExtCommNameMap[ACCEPT]:          ACCEPT,
	ExtCommNameMap[DISCARD]:         DISCARD,
	ExtCommNameMap[RATE]:            RATE,
	ExtCommNameMap[REDIRECT]:        REDIRECT,
	ExtCommNameMap[MARK]:            MARK,
	ExtCommNameMap[ACTION]:          ACTION,
	ExtCommNameMap[RT]:              RT,
	ExtCommNameMap[ENCAP]:           ENCAP,
	ExtCommNameMap[ESI_LABEL]:       ESI_LABEL,
	ExtCommNameMap[ROUTER_MAC]:      ROUTER_MAC,
	ExtCommNameMap[DEFAULT_GATEWAY]: DEFAULT_GATEWAY,
	ExtCommNameMap[VALID]:           VALID,
	ExtCommNameMap[NOT_FOUND]:       NOT_FOUND,
	ExtCommNameMap[INVALID]:         INVALID,
}

func rateLimitParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	exp := regexp.MustCompile(fmt.Sprintf("^(%s|(%s) (\\d+)(\\.(\\d+))?)( as (\\d+))?$", ExtCommNameMap[DISCARD], ExtCommNameMap[RATE]))
	elems := exp.FindStringSubmatch(strings.Join(args, " "))
	if len(elems) != 8 {
		return nil, fmt.Errorf("invalid rate-limit")
	}
	var rate float32
	var as uint64
	if elems[2] == ExtCommNameMap[RATE] {
		f, err := strconv.ParseFloat(elems[3]+elems[4], 32)
		if err != nil {
			return nil, err
		}
		rate = float32(f)
	}
	if elems[7] != "" {
		var err error
		as, err = strconv.ParseUint(elems[7], 10, 16)
		if err != nil {
			return nil, err
		}
	}
	return []bgp.ExtendedCommunityInterface{bgp.NewTrafficRateExtended(uint16(as), rate)}, nil
}

func redirectParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 2 || args[0] != ExtCommNameMap[REDIRECT] {
		return nil, fmt.Errorf("invalid redirect")
	}
	rt, err := bgp.ParseRouteTarget(strings.Join(args[1:], " "))
	if err != nil {
		return nil, err
	}
	switch rt.(type) {
	case *bgp.TwoOctetAsSpecificExtended:
		r := rt.(*bgp.TwoOctetAsSpecificExtended)
		return []bgp.ExtendedCommunityInterface{bgp.NewRedirectTwoOctetAsSpecificExtended(r.AS, r.LocalAdmin)}, nil
	case *bgp.IPv4AddressSpecificExtended:
		r := rt.(*bgp.IPv4AddressSpecificExtended)
		return []bgp.ExtendedCommunityInterface{bgp.NewRedirectIPv4AddressSpecificExtended(r.IPv4.String(), r.LocalAdmin)}, nil
	case *bgp.FourOctetAsSpecificExtended:
		r := rt.(*bgp.FourOctetAsSpecificExtended)
		return []bgp.ExtendedCommunityInterface{bgp.NewRedirectFourOctetAsSpecificExtended(r.AS, r.LocalAdmin)}, nil
	case *bgp.IPv6AddressSpecificExtended:
		r := rt.(*bgp.IPv6AddressSpecificExtended)
		return []bgp.ExtendedCommunityInterface{bgp.NewRedirectIPv6AddressSpecificExtended(r.IPv6.String(), r.LocalAdmin)}, nil
	}
	return nil, fmt.Errorf("invalid redirect")
}

func markParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 2 || args[0] != ExtCommNameMap[MARK] {
		return nil, fmt.Errorf("invalid mark")
	}
	dscp, err := strconv.ParseUint(args[1], 10, 8)
	if err != nil {
		return nil, fmt.Errorf("invalid mark")
	}
	return []bgp.ExtendedCommunityInterface{bgp.NewTrafficRemarkExtended(uint8(dscp))}, nil
}

func actionParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 2 || args[0] != ExtCommNameMap[ACTION] {
		return nil, fmt.Errorf("invalid action")
	}
	sample := false
	terminal := false
	switch args[1] {
	case "sample":
		sample = true
	case "terminal":
		terminal = true
	case "terminal-sample", "sample-terminal":
		sample = true
		terminal = true
	default:
		return nil, fmt.Errorf("invalid action")
	}
	return []bgp.ExtendedCommunityInterface{bgp.NewTrafficActionExtended(terminal, sample)}, nil
}

func rtParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 2 || args[0] != ExtCommNameMap[RT] {
		return nil, fmt.Errorf("invalid rt")
	}
	exts := make([]bgp.ExtendedCommunityInterface, 0, len(args[1:]))
	for _, arg := range args[1:] {
		rt, err := bgp.ParseRouteTarget(arg)
		if err != nil {
			return nil, err
		}
		exts = append(exts, rt)
	}
	return exts, nil
}

func encapParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 2 || args[0] != ExtCommNameMap[ENCAP] {
		return nil, fmt.Errorf("invalid encap")
	}
	var typ bgp.TunnelType
	switch args[1] {
	case "l2tpv3":
		typ = bgp.TUNNEL_TYPE_L2TP3
	case "gre":
		typ = bgp.TUNNEL_TYPE_GRE
	case "ip-in-ip":
		typ = bgp.TUNNEL_TYPE_IP_IN_IP
	case "vxlan":
		typ = bgp.TUNNEL_TYPE_VXLAN
	case "nvgre":
		typ = bgp.TUNNEL_TYPE_NVGRE
	case "mpls":
		typ = bgp.TUNNEL_TYPE_MPLS
	case "mpls-in-gre":
		typ = bgp.TUNNEL_TYPE_MPLS_IN_GRE
	case "vxlan-gre":
		typ = bgp.TUNNEL_TYPE_VXLAN_GRE
	default:
		return nil, fmt.Errorf("invalid encap type")
	}
	o := bgp.NewOpaqueExtended(true)
	o.SubType = bgp.EC_SUBTYPE_ENCAPSULATION
	o.Value = &bgp.EncapExtended{TunnelType: typ}
	return []bgp.ExtendedCommunityInterface{o}, nil
}

func esiLabelParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 2 || args[0] != ExtCommNameMap[ESI_LABEL] {
		return nil, fmt.Errorf("invalid esi-label")
	}
	label, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		return nil, err
	}
	isSingleActive := false
	if len(args) > 2 {
		switch args[2] {
		case "single-active":
			isSingleActive = true
		case "all-active":
			// isSingleActive = false
		default:
			return nil, fmt.Errorf("invalid esi-label")
		}
	}
	o := &bgp.ESILabelExtended{
		Label:          uint32(label),
		IsSingleActive: isSingleActive,
	}
	return []bgp.ExtendedCommunityInterface{o}, nil
}

func routerMacParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 2 || args[0] != ExtCommNameMap[ROUTER_MAC] {
		return nil, fmt.Errorf("invalid router's mac")
	}
	hw, err := net.ParseMAC(args[1])
	if err != nil {
		return nil, err
	}
	o := &bgp.RouterMacExtended{Mac: hw}
	return []bgp.ExtendedCommunityInterface{o}, nil
}

func defaultGatewayParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 1 || args[0] != ExtCommNameMap[DEFAULT_GATEWAY] {
		return nil, fmt.Errorf("invalid default-gateway")
	}
	o := &bgp.OpaqueExtended{
		IsTransitive: true,
		Value:        &bgp.DefaultGatewayExtended{},
		SubType:      bgp.EC_SUBTYPE_DEFAULT_GATEWAY,
	}
	return []bgp.ExtendedCommunityInterface{o}, nil
}

func validationParser(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("invalid validation state")
	}
	var typ bgp.ValidationState
	switch args[0] {
	case "valid":
		typ = bgp.VALIDATION_STATE_VALID
	case "not-found":
		typ = bgp.VALIDATION_STATE_NOT_FOUND
	case "invalid":
		typ = bgp.VALIDATION_STATE_INVALID
	default:
		return nil, fmt.Errorf("invalid validation state")
	}
	o := bgp.NewOpaqueExtended(true)
	o.SubType = bgp.EC_SUBTYPE_ORIGIN_VALIDATION
	o.Value = &bgp.ValidationExtended{Value: typ}
	return []bgp.ExtendedCommunityInterface{o}, nil
}

var ExtCommParserMap = map[ExtCommType]func([]string) ([]bgp.ExtendedCommunityInterface, error){
	ACCEPT:          nil,
	DISCARD:         rateLimitParser,
	RATE:            rateLimitParser,
	REDIRECT:        redirectParser,
	MARK:            markParser,
	ACTION:          actionParser,
	RT:              rtParser,
	ENCAP:           encapParser,
	ESI_LABEL:       esiLabelParser,
	ROUTER_MAC:      routerMacParser,
	DEFAULT_GATEWAY: defaultGatewayParser,
	VALID:           validationParser,
	NOT_FOUND:       validationParser,
	INVALID:         validationParser,
}

func ParseExtendedCommunities(args []string) ([]bgp.ExtendedCommunityInterface, error) {
	idxs := make([]struct {
		t ExtCommType
		i int
	}, 0, len(ExtCommNameMap))
	for idx, v := range args {
		if t, ok := ExtCommValueMap[v]; ok {
			idxs = append(idxs, struct {
				t ExtCommType
				i int
			}{t, idx})
		}
	}
	exts := make([]bgp.ExtendedCommunityInterface, 0, len(idxs))
	for i, idx := range idxs {
		var a []string
		f := ExtCommParserMap[idx.t]
		if i < len(idxs)-1 {
			a = args[:idxs[i+1].i-idx.i]
			args = args[(idxs[i+1].i - idx.i):]
		} else {
			a = args
			args = nil
		}
		if f == nil {
			continue
		}
		ext, err := f(a)
		if err != nil {
			return nil, err
		}
		exts = append(exts, ext...)
	}
	if len(args) > 0 {
		return nil, fmt.Errorf("failed to parse %v", args)
	}
	return exts, nil
}

func ParseFlowSpecArgs(rf bgp.RouteFamily, args []string) (bgp.AddrPrefixInterface, []string, error) {
	// Format:
	// match <rule>... [then <action>...] [rd <rd>] [rt <rt>...]
	req := 3 // match <key1> <arg1> [<key2> <arg2>...]
	if len(args) < req {
		return nil, nil, fmt.Errorf("%d args required at least, but got %d", req, len(args))
	}
	m := extractReserved(args, []string{"match", "then", "rd", "rt"})
	if len(m["match"]) == 0 {
		return nil, nil, fmt.Errorf("specify filtering rules with keyword 'match'")
	}

	var rd bgp.RouteDistinguisherInterface
	extcomms := m["then"]
	switch rf {
	case bgp.RF_FS_IPv4_VPN, bgp.RF_FS_IPv6_VPN, bgp.RF_FS_L2_VPN:
		if len(m["rd"]) == 0 {
			return nil, nil, fmt.Errorf("specify rd")
		}
		var err error
		if rd, err = bgp.ParseRouteDistinguisher(m["rd"][0]); err != nil {
			return nil, nil, fmt.Errorf("invalid rd: %s", m["rd"][0])
		}
		if len(m["rt"]) > 0 {
			extcomms = append(extcomms, "rt")
			extcomms = append(extcomms, m["rt"]...)
		}
	default:
		if len(m["rd"]) > 0 {
			return nil, nil, fmt.Errorf("cannot specify rd for %s", rf.String())
		}
		if len(m["rt"]) > 0 {
			return nil, nil, fmt.Errorf("cannot specify rt for %s", rf.String())
		}
	}

	rules, err := bgp.ParseFlowSpecComponents(rf, strings.Join(m["match"], " "))
	if err != nil {
		return nil, nil, err
	}

	var nlri bgp.AddrPrefixInterface
	switch rf {
	case bgp.RF_FS_IPv4_UC:
		nlri = bgp.NewFlowSpecIPv4Unicast(rules)
	case bgp.RF_FS_IPv6_UC:
		nlri = bgp.NewFlowSpecIPv6Unicast(rules)
	case bgp.RF_FS_IPv4_VPN:
		nlri = bgp.NewFlowSpecIPv4VPN(rd, rules)
	case bgp.RF_FS_IPv6_VPN:
		nlri = bgp.NewFlowSpecIPv6VPN(rd, rules)
	case bgp.RF_FS_L2_VPN:
		nlri = bgp.NewFlowSpecL2VPN(rd, rules)
	default:
		return nil, nil, fmt.Errorf("invalid route family")
	}

	return nlri, extcomms, nil
}

func ParseEvpnEthernetAutoDiscoveryArgs(args []string) (bgp.AddrPrefixInterface, []string, error) {
	// Format:
	// esi <esi> etag <etag> label <label> rd <rd> [rt <rt>...] [encap <encap type>] [esi-label <esi-label> [single-active | all-active]]
	req := 8
	if len(args) < req {
		return nil, nil, fmt.Errorf("%d args required at least, but got %d", req, len(args))
	}
	m := extractReserved(args, []string{"esi", "etag", "label", "rd", "rt", "encap", "esi-label"})
	for _, f := range []string{"esi", "etag", "label", "rd"} {
		for len(m[f]) == 0 {
			return nil, nil, fmt.Errorf("specify %s", f)
		}
	}

	esi, err := bgp.ParseEthernetSegmentIdentifier(m["esi"])
	if err != nil {
		return nil, nil, err
	}

	e, err := strconv.ParseUint(m["etag"][0], 10, 32)
	if err != nil {
		return nil, nil, err
	}
	etag := uint32(e)

	l, err := strconv.ParseUint(m["label"][0], 10, 32)
	if err != nil {
		return nil, nil, err
	}
	label := uint32(l)

	rd, err := bgp.ParseRouteDistinguisher(m["rd"][0])
	if err != nil {
		return nil, nil, err
	}

	extcomms := make([]string, 0)
	if len(m["rt"]) > 0 {
		extcomms = append(extcomms, "rt")
		extcomms = append(extcomms, m["rt"]...)
	}
	if len(m["encap"]) > 0 {
		extcomms = append(extcomms, "encap", m["encap"][0])
	}
	if len(m["esi-label"]) > 0 {
		extcomms = append(extcomms, "esi-label")
		extcomms = append(extcomms, m["esi-label"]...)
	}

	r := &bgp.EVPNEthernetAutoDiscoveryRoute{
		RD:    rd,
		ESI:   esi,
		ETag:  etag,
		Label: label,
	}
	return bgp.NewEVPNNLRI(bgp.EVPN_ROUTE_TYPE_ETHERNET_AUTO_DISCOVERY, 0, r), extcomms, nil
}

func ParseEvpnMacAdvArgs(args []string) (bgp.AddrPrefixInterface, []string, error) {
	// Format:
	// <mac address> <ip address> [esi <esi>] etag <etag> label <label> rd <rd> [rt <rt>...] [encap <encap type>] [default-gateway]
	// or
	// <mac address> <ip address> <etag> [esi <esi>] label <label> rd <rd> [rt <rt>...] [encap <encap type>] [default-gateway]
	// or
	// <mac address> <ip address> <etag> <label> [esi <esi>] rd <rd> [rt <rt>...] [encap <encap type>] [default-gateway]
	req := 6
	if len(args) < req {
		return nil, nil, fmt.Errorf("%d args required at least, but got %d", req, len(args))
	}
	m := extractReserved(args, []string{"esi", "etag", "label", "rd", "rt", "encap"})
	if len(m[""]) < 2 {
		return nil, nil, fmt.Errorf("specify mac and ip address")
	}
	macStr := m[""][0]
	ipStr := m[""][1]
	eTagStr := ""
	labelStr := ""
	if len(m[""]) == 2 {
		if len(m["etag"]) == 0 || len(m["label"]) == 0 {
			return nil, nil, fmt.Errorf("specify etag and label")
		}
		eTagStr = m["etag"][0]
		labelStr = m["label"][0]
	} else if len(m[""]) == 3 {
		if len(m["label"]) == 0 {
			return nil, nil, fmt.Errorf("specify label")
		}
		eTagStr = m[""][2]
		labelStr = m["label"][0]
	} else {
		eTagStr = m[""][2]
		labelStr = m[""][3]
	}
	if len(m["rd"]) == 0 {
		return nil, nil, fmt.Errorf("specify rd")
	}

	mac, err := net.ParseMAC(macStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid mac address: %s", macStr)
	}

	ip := net.ParseIP(ipStr)
	ipLen := 0
	if ip == nil {
		return nil, nil, fmt.Errorf("invalid ip address: %s", ipStr)
	} else if ip.IsUnspecified() {
		ip = nil
	} else if ip.To4() != nil {
		ipLen = net.IPv4len * 8
	} else {
		ipLen = net.IPv6len * 8
	}

	esi, err := bgp.ParseEthernetSegmentIdentifier(m["esi"])
	if err != nil {
		return nil, nil, err
	}

	eTag, err := strconv.ParseUint(eTagStr, 10, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid etag: %s: %s", eTagStr, err)
	}

	var labels []uint32
	for _, l := range strings.SplitN(labelStr, ",", 2) {
		label, err := strconv.ParseUint(l, 10, 32)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid label: %s: %s", labelStr, err)
		}
		labels = append(labels, uint32(label))
	}

	rd, err := bgp.ParseRouteDistinguisher(m["rd"][0])
	if err != nil {
		return nil, nil, err
	}

	extcomms := make([]string, 0)
	if len(m["rt"]) > 0 {
		extcomms = append(extcomms, "rt")
		extcomms = append(extcomms, m["rt"]...)
	}
	if len(m["encap"]) > 0 {
		extcomms = append(extcomms, "encap", m["encap"][0])
	}
	for _, a := range args {
		if a == "default-gateway" {
			extcomms = append(extcomms, "default-gateway")
			break
		}
	}

	r := &bgp.EVPNMacIPAdvertisementRoute{
		RD:               rd,
		ESI:              esi,
		MacAddressLength: 48,
		MacAddress:       mac,
		IPAddressLength:  uint8(ipLen),
		IPAddress:        ip,
		Labels:           labels,
		ETag:             uint32(eTag),
	}
	return bgp.NewEVPNNLRI(bgp.EVPN_ROUTE_TYPE_MAC_IP_ADVERTISEMENT, 0, r), extcomms, nil
}

func ParseEvpnMulticastArgs(args []string) (bgp.AddrPrefixInterface, []string, error) {
	// Format:
	// <ip address> etag <etag> rd <rd> [rt <rt>...] [encap <encap type>]
	// or
	// <ip address> <etag> rd <rd> [rt <rt>...] [encap <encap type>]
	req := 4
	if len(args) < req {
		return nil, nil, fmt.Errorf("%d args required at least, but got %d", req, len(args))
	}
	m := extractReserved(args, []string{"etag", "rd", "rt", "encap"})
	if len(m[""]) < 1 {
		return nil, nil, fmt.Errorf("specify ip address")
	}
	ipStr := m[""][0]
	eTagStr := ""
	if len(m[""]) == 1 {
		if len(m["etag"]) == 0 {
			return nil, nil, fmt.Errorf("specify etag")
		}
		eTagStr = m["etag"][0]
	} else {
		eTagStr = m[""][1]
	}
	if len(m["rd"]) == 0 {
		return nil, nil, fmt.Errorf("specify rd")
	}

	ip := net.ParseIP(ipStr)
	ipLen := 0
	if ip == nil {
		return nil, nil, fmt.Errorf("invalid ip address: %s", ipStr)
	} else if ip.IsUnspecified() {
		ip = nil
	} else if ip.To4() != nil {
		ipLen = net.IPv4len * 8
	} else {
		ipLen = net.IPv6len * 8
	}

	eTag, err := strconv.ParseUint(eTagStr, 10, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid etag: %s: %s", eTagStr, err)
	}

	rd, err := bgp.ParseRouteDistinguisher(m["rd"][0])
	if err != nil {
		return nil, nil, err
	}

	extcomms := make([]string, 0)
	if len(m["rt"]) > 0 {
		extcomms = append(extcomms, "rt")
		extcomms = append(extcomms, m["rt"]...)
	}
	if len(m["encap"]) > 0 {
		extcomms = append(extcomms, "encap", m["encap"][0])
	}

	r := &bgp.EVPNMulticastEthernetTagRoute{
		RD:              rd,
		IPAddressLength: uint8(ipLen),
		IPAddress:       ip,
		ETag:            uint32(eTag),
	}
	return bgp.NewEVPNNLRI(bgp.EVPN_INCLUSIVE_MULTICAST_ETHERNET_TAG, 0, r), extcomms, nil
}

func ParseEvpnEthernetSegmentArgs(args []string) (bgp.AddrPrefixInterface, []string, error) {
	// Format:
	// <ip address> esi <esi> rd <rd> [rt <rt>...] [encap <encap type>]
	req := 5
	if len(args) < req {
		return nil, nil, fmt.Errorf("%d args required at least, but got %d", req, len(args))
	}
	m := extractReserved(args, []string{"esi", "rd", "rt", "encap"})
	if len(m[""]) < 1 {
		return nil, nil, fmt.Errorf("specify ip address")
	}
	for _, f := range []string{"esi", "rd"} {
		for len(m[f]) == 0 {
			return nil, nil, fmt.Errorf("specify %s", f)
		}
	}

	ip := net.ParseIP(m[""][0])
	ipLen := 0
	if ip == nil {
		return nil, nil, fmt.Errorf("invalid ip address: %s", m[""][0])
	} else if ip.IsUnspecified() {
		ip = nil
	} else if ip.To4() != nil {
		ipLen = net.IPv4len * 8
	} else {
		ipLen = net.IPv6len * 8
	}

	esi, err := bgp.ParseEthernetSegmentIdentifier(m["esi"])
	if err != nil {
		return nil, nil, err
	}

	rd, err := bgp.ParseRouteDistinguisher(m["rd"][0])
	if err != nil {
		return nil, nil, err
	}

	extcomms := make([]string, 0)
	if len(m["rt"]) > 0 {
		extcomms = append(extcomms, "rt")
		extcomms = append(extcomms, m["rt"]...)
	}
	if len(m["encap"]) > 0 {
		extcomms = append(extcomms, "encap", m["encap"][0])
	}

	r := &bgp.EVPNEthernetSegmentRoute{
		RD:              rd,
		ESI:             esi,
		IPAddressLength: uint8(ipLen),
		IPAddress:       ip,
	}
	return bgp.NewEVPNNLRI(bgp.EVPN_ETHERNET_SEGMENT_ROUTE, 0, r), extcomms, nil
}

func ParseEvpnIPPrefixArgs(args []string) (bgp.AddrPrefixInterface, []string, error) {
	// Format:
	// <ip prefix> [gw <gateway>] [esi <esi>] etag <etag> [label <label>] rd <rd> [rt <rt>...] [encap <encap type>]
	req := 5
	if len(args) < req {
		return nil, nil, fmt.Errorf("%d args required at least, but got %d", req, len(args))
	}
	m := extractReserved(args, []string{"gw", "esi", "etag", "label", "rd", "rt", "encap", "router-mac"})
	if len(m[""]) < 1 {
		return nil, nil, fmt.Errorf("specify prefix")
	}
	for _, f := range []string{"etag", "rd"} {
		for len(m[f]) == 0 {
			return nil, nil, fmt.Errorf("specify %s", f)
		}
	}

	_, nw, err := net.ParseCIDR(m[""][0])
	if err != nil {
		return nil, nil, err
	}
	ones, _ := nw.Mask.Size()

	var gw net.IP
	if len(m["gw"]) > 0 {
		gw = net.ParseIP(m["gw"][0])
	}

	rd, err := bgp.ParseRouteDistinguisher(m["rd"][0])
	if err != nil {
		return nil, nil, err
	}

	esi, err := bgp.ParseEthernetSegmentIdentifier(m["esi"])
	if err != nil {
		return nil, nil, err
	}

	e, err := strconv.ParseUint(m["etag"][0], 10, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid etag: %s: %s", m["etag"][0], err)
	}
	etag := uint32(e)

	var label uint32
	if len(m["label"]) > 0 {
		e, err := strconv.ParseUint(m["label"][0], 10, 32)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid label: %s: %s", m["label"][0], err)
		}
		label = uint32(e)
	}

	extcomms := make([]string, 0)
	if len(m["rt"]) > 0 {
		extcomms = append(extcomms, "rt")
		extcomms = append(extcomms, m["rt"]...)
	}
	if len(m["encap"]) > 0 {
		extcomms = append(extcomms, "encap", m["encap"][0])
	}
	if len(m["router-mac"]) > 0 {
		extcomms = append(extcomms, "router-mac", m["router-mac"][0])
	}

	r := &bgp.EVPNIPPrefixRoute{
		RD:             rd,
		ESI:            esi,
		ETag:           etag,
		IPPrefixLength: uint8(ones),
		IPPrefix:       nw.IP,
		GWIPAddress:    gw,
		Label:          label,
	}
	return bgp.NewEVPNNLRI(bgp.EVPN_IP_PREFIX, 0, r), extcomms, nil
}

func ParseEvpnArgs(args []string) (bgp.AddrPrefixInterface, []string, error) {
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("lack of args. need 1 but %d", len(args))
	}
	subtype := args[0]
	args = args[1:]
	switch subtype {
	case "a-d":
		return ParseEvpnEthernetAutoDiscoveryArgs(args)
	case "macadv":
		return ParseEvpnMacAdvArgs(args)
	case "multicast":
		return ParseEvpnMulticastArgs(args)
	case "esi":
		return ParseEvpnEthernetSegmentArgs(args)
	case "prefix":
		return ParseEvpnIPPrefixArgs(args)
	}
	return nil, nil, fmt.Errorf("invalid subtype. expect [macadv|multicast|prefix] but %s", subtype)
}

func extractOrigin(args []string) ([]string, bgp.PathAttributeInterface, error) {
	typ := bgp.BGP_ORIGIN_ATTR_TYPE_INCOMPLETE
	for idx, arg := range args {
		if arg == "origin" && len(args) > (idx+1) {
			switch args[idx+1] {
			case "igp":
				typ = bgp.BGP_ORIGIN_ATTR_TYPE_IGP
			case "egp":
				typ = bgp.BGP_ORIGIN_ATTR_TYPE_EGP
			case "incomplete":
			default:
				return nil, nil, fmt.Errorf("invalid origin type. expect [igp|egp|incomplete] but %s", args[idx+1])
			}
			args = append(args[:idx], args[idx+2:]...)
			break
		}
	}
	return args, bgp.NewPathAttributeOrigin(uint8(typ)), nil
}

func toAs4Value(s string) (uint32, error) {
	if strings.Contains(s, ".") {
		v := strings.Split(s, ".")
		upper, err := strconv.ParseUint(v[0], 10, 16)
		if err != nil {
			return 0, nil
		}
		lower, err := strconv.ParseUint(v[1], 10, 16)
		if err != nil {
			return 0, nil
		}
		return uint32(upper<<16 | lower), nil
	}
	i, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(i), nil
}

func newAsPath(aspath string) (bgp.PathAttributeInterface, error) {
	// For the first step, parses "aspath" into a list of uint32 list.
	// e.g.) "10 20 {30,40} 50" -> [][]uint32{{10, 20}, {30, 40}, {50}}
	exp := regexp.MustCompile("[{}]")
	segments := exp.Split(aspath, -1)
	asPathPrams := make([]bgp.AsPathParamInterface, 0, len(segments))
	exp = regexp.MustCompile(",|\\s+")
	for idx, segment := range segments {
		if segment == "" {
			continue
		}
		nums := exp.Split(segment, -1)
		asNums := make([]uint32, 0, len(nums))
		for _, n := range nums {
			if n == "" {
				continue
			}
			if asn, err := toAs4Value(n); err != nil {
				return nil, err
			} else {
				asNums = append(asNums, uint32(asn))
			}
		}
		// Assumes "idx" is even, the given "segment" is of type AS_SEQUENCE,
		// otherwise AS_SET, because the "segment" enclosed in parentheses is
		// of type AS_SET.
		if idx%2 == 0 {
			asPathPrams = append(asPathPrams, bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, asNums))
		} else {
			asPathPrams = append(asPathPrams, bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SET, asNums))
		}
	}
	return bgp.NewPathAttributeAsPath(asPathPrams), nil
}

func extractAsPath(args []string) ([]string, bgp.PathAttributeInterface, error) {
	for idx, arg := range args {
		if arg == "aspath" && len(args) > (idx+1) {
			attr, err := newAsPath(args[idx+1])
			if err != nil {
				return nil, nil, err
			}
			args = append(args[:idx], args[idx+2:]...)
			return args, attr, nil
		}
	}
	return args, nil, nil
}

func extractNexthop(rf bgp.RouteFamily, args []string) ([]string, string, error) {
	afi, _ := bgp.RouteFamilyToAfiSafi(rf)
	nexthop := "0.0.0.0"
	if afi == bgp.AFI_IP6 {
		nexthop = "::"
	}
	for idx, arg := range args {
		if arg == "nexthop" && len(args) > (idx+1) {
			if net.ParseIP(args[idx+1]) == nil {
				return nil, "", fmt.Errorf("invalid nexthop address")
			}
			nexthop = args[idx+1]
			args = append(args[:idx], args[idx+2:]...)
			break
		}
	}
	return args, nexthop, nil
}

func extractLocalPref(args []string) ([]string, bgp.PathAttributeInterface, error) {
	for idx, arg := range args {
		if arg == "local-pref" && len(args) > (idx+1) {
			metric, err := strconv.ParseUint(args[idx+1], 10, 32)
			if err != nil {
				return nil, nil, err
			}
			args = append(args[:idx], args[idx+2:]...)
			return args, bgp.NewPathAttributeLocalPref(uint32(metric)), nil
		}
	}
	return args, nil, nil
}

func extractMed(args []string) ([]string, bgp.PathAttributeInterface, error) {
	for idx, arg := range args {
		if arg == "med" && len(args) > (idx+1) {
			med, err := strconv.ParseUint(args[idx+1], 10, 32)
			if err != nil {
				return nil, nil, err
			}
			args = append(args[:idx], args[idx+2:]...)
			return args, bgp.NewPathAttributeMultiExitDisc(uint32(med)), nil
		}
	}
	return args, nil, nil
}

func extractCommunity(args []string) ([]string, bgp.PathAttributeInterface, error) {
	for idx, arg := range args {
		if arg == "community" && len(args) > (idx+1) {
			elems := strings.Split(args[idx+1], ",")
			comms := make([]uint32, 0, 1)
			for _, elem := range elems {
				c, err := table.ParseCommunity(elem)
				if err != nil {
					return nil, nil, err
				}
				comms = append(comms, c)
			}
			args = append(args[:idx], args[idx+2:]...)
			return args, bgp.NewPathAttributeCommunities(comms), nil
		}
	}
	return args, nil, nil
}

func extractLargeCommunity(args []string) ([]string, bgp.PathAttributeInterface, error) {
	for idx, arg := range args {
		if arg == "large-community" && len(args) > (idx+1) {
			elems := strings.Split(args[idx+1], ",")
			comms := make([]*bgp.LargeCommunity, 0, 1)
			for _, elem := range elems {
				c, err := bgp.ParseLargeCommunity(elem)
				if err != nil {
					return nil, nil, err
				}
				comms = append(comms, c)
			}
			args = append(args[:idx], args[idx+2:]...)
			return args, bgp.NewPathAttributeLargeCommunities(comms), nil
		}
	}
	return args, nil, nil
}

func extractAigp(args []string) ([]string, bgp.PathAttributeInterface, error) {
	for idx, arg := range args {
		if arg == "aigp" {
			if len(args) < (idx + 3) {
				return nil, nil, fmt.Errorf("invalid aigp format")
			}
			typ := args[idx+1]
			switch typ {
			case "metric":
				metric, err := strconv.ParseUint(args[idx+2], 10, 64)
				if err != nil {
					return nil, nil, err
				}
				aigp := bgp.NewPathAttributeAigp([]bgp.AigpTLV{bgp.NewAigpTLVIgpMetric(uint64(metric))})
				return append(args[:idx], args[idx+3:]...), aigp, nil
			default:
				return nil, nil, fmt.Errorf("unknown aigp type: %s", typ)
			}
		}
	}
	return args, nil, nil
}

func extractAggregator(args []string) ([]string, bgp.PathAttributeInterface, error) {
	for idx, arg := range args {
		if arg == "aggregator" {
			if len(args) < (idx + 1) {
				return nil, nil, fmt.Errorf("invalid aggregator format")
			}
			v := strings.SplitN(args[idx+1], ":", 2)
			if len(v) != 2 {
				return nil, nil, fmt.Errorf("invalid aggregator format")
			}
			as, err := strconv.ParseUint(v[0], 10, 32)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid aggregator format")
			}
			attr := bgp.NewPathAttributeAggregator(uint32(as), net.ParseIP(v[1]).String())
			return append(args[:idx], args[idx+2:]...), attr, nil
		}
	}
	return args, nil, nil
}

func extractRouteDistinguisher(args []string) ([]string, bgp.RouteDistinguisherInterface, error) {
	for idx, arg := range args {
		if arg == "rd" {
			if len(args) < (idx + 1) {
				return nil, nil, fmt.Errorf("invalid rd format")
			}
			rd, err := bgp.ParseRouteDistinguisher(args[idx+1])
			if err != nil {
				return nil, nil, err
			}
			return append(args[:idx], args[idx+2:]...), rd, nil
		}
	}
	return args, nil, nil
}

func ParsePath(rf bgp.RouteFamily, args []string) (*table.Path, error) {
	var nlri bgp.AddrPrefixInterface
	var extcomms []string
	var err error
	attrs := table.PathAttrs(make([]bgp.PathAttributeInterface, 0, 1))

	fns := []func([]string) ([]string, bgp.PathAttributeInterface, error){
		extractAsPath,
		extractOrigin,
		extractMed,
		extractLocalPref,
		extractCommunity,
		extractAigp,
		extractAggregator,
		extractLargeCommunity,
	}

	for _, fn := range fns {
		var a bgp.PathAttributeInterface
		args, a, err = fn(args)
		if err != nil {
			return nil, err
		}
		if a != nil {
			attrs = append(attrs, a)
		}
	}

	args, nexthop, err := extractNexthop(rf, args)
	if err != nil {
		return nil, err
	}

	switch rf {
	case bgp.RF_IPv4_UC, bgp.RF_IPv6_UC:
		if len(args) < 1 {
			return nil, fmt.Errorf("invalid format")
		}
		ip, nw, err := net.ParseCIDR(args[0])
		if err != nil {
			return nil, err
		}
		ones, _ := nw.Mask.Size()
		if rf == bgp.RF_IPv4_UC {
			if ip.To4() == nil {
				return nil, fmt.Errorf("invalid ipv4 prefix")
			}
			nlri = bgp.NewIPAddrPrefix(uint8(ones), ip.String())
		} else {
			if ip.To16() == nil {
				return nil, fmt.Errorf("invalid ipv6 prefix")
			}
			nlri = bgp.NewIPv6AddrPrefix(uint8(ones), ip.String())
		}

		if len(args) > 2 && args[1] == "identifier" {
			id, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid format")
			}
			nlri.SetPathIdentifier(uint32(id))
			extcomms = args[3:]
		} else {
			extcomms = args[1:]
		}

	case bgp.RF_IPv4_VPN, bgp.RF_IPv6_VPN:
		if len(args) < 5 || args[1] != "label" || args[3] != "rd" {
			return nil, fmt.Errorf("invalid format")
		}
		ip, nw, _ := net.ParseCIDR(args[0])
		ones, _ := nw.Mask.Size()

		label, err := strconv.ParseUint(args[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid format")
		}
		mpls := bgp.NewMPLSLabelStack(uint32(label))

		rd, err := bgp.ParseRouteDistinguisher(args[4])
		if err != nil {
			return nil, err
		}

		extcomms = args[5:]

		if rf == bgp.RF_IPv4_VPN {
			if ip.To4() == nil {
				return nil, fmt.Errorf("invalid ipv4 prefix")
			}
			nlri = bgp.NewLabeledVPNIPAddrPrefix(uint8(ones), ip.String(), *mpls, rd)
		} else {
			if ip.To16() == nil {
				return nil, fmt.Errorf("invalid ipv6 prefix")
			}
			nlri = bgp.NewLabeledVPNIPv6AddrPrefix(uint8(ones), ip.String(), *mpls, rd)
		}
	case bgp.RF_IPv4_MPLS, bgp.RF_IPv6_MPLS:
		if len(args) < 2 {
			return nil, fmt.Errorf("invalid format")
		}

		ip, nw, _ := net.ParseCIDR(args[0])
		ones, _ := nw.Mask.Size()

		mpls, err := bgp.ParseMPLSLabelStack(args[1])
		if err != nil {
			return nil, err
		}

		extcomms = args[2:]

		if rf == bgp.RF_IPv4_MPLS {
			if ip.To4() == nil {
				return nil, fmt.Errorf("invalid ipv4 prefix")
			}
			nlri = bgp.NewLabeledIPAddrPrefix(uint8(ones), ip.String(), *mpls)
		} else {
			if ip.To4() != nil {
				return nil, fmt.Errorf("invalid ipv6 prefix")
			}
			nlri = bgp.NewLabeledIPv6AddrPrefix(uint8(ones), ip.String(), *mpls)
		}
	case bgp.RF_EVPN:
		nlri, extcomms, err = ParseEvpnArgs(args)
	case bgp.RF_FS_IPv4_UC, bgp.RF_FS_IPv4_VPN, bgp.RF_FS_IPv6_UC, bgp.RF_FS_IPv6_VPN, bgp.RF_FS_L2_VPN:
		nlri, extcomms, err = ParseFlowSpecArgs(rf, args)
	case bgp.RF_OPAQUE:
		m := extractReserved(args, []string{"key", "value"})
		if len(m["key"]) != 1 {
			return nil, fmt.Errorf("opaque nlri key missing")
		}
		if len(m["value"]) > 0 {
			nlri = bgp.NewOpaqueNLRI([]byte(m["key"][0]), []byte(m["value"][0]))
		} else {
			nlri = bgp.NewOpaqueNLRI([]byte(m["key"][0]), nil)
		}
	default:
		return nil, fmt.Errorf("Unsupported route family: %s", rf)
	}
	if err != nil {
		return nil, err
	}

	if rf == bgp.RF_IPv4_UC && net.ParseIP(nexthop).To4() != nil {
		attrs = append(attrs, bgp.NewPathAttributeNextHop(nexthop))
	} else {
		mpreach := bgp.NewPathAttributeMpReachNLRI(nexthop, []bgp.AddrPrefixInterface{nlri})
		attrs = append(attrs, mpreach)
	}

	if extcomms != nil && len(extcomms) > 0 {
		extcomms, err := ParseExtendedCommunities(extcomms)
		if err != nil {
			return nil, err
		}
		normalextcomms := make([]bgp.ExtendedCommunityInterface, 0)
		ipv6extcomms := make([]bgp.ExtendedCommunityInterface, 0)
		for _, com := range extcomms {
			switch com.(type) {
			case *bgp.RedirectIPv6AddressSpecificExtended:
				ipv6extcomms = append(ipv6extcomms, com)
			default:
				normalextcomms = append(normalextcomms, com)
			}
		}
		if len(normalextcomms) != 0 {
			p := bgp.NewPathAttributeExtendedCommunities(normalextcomms)
			attrs = append(attrs, p)
		}
		if len(ipv6extcomms) != 0 {
			ip6p := bgp.NewPathAttributeIP6ExtendedCommunities(ipv6extcomms)
			attrs = append(attrs, ip6p)
		}
	}

	sort.Sort(attrs)

	return table.NewPath(nil, nlri, false, attrs, time.Now(), false), nil
}

func showGlobalRib(args []string) error {
	return showNeighborRib(CMD_GLOBAL, "", args)
}

func modPath(resource string, name, modtype string, args []string) error {
	rf, err := checkAddressFamily(bgp.RF_IPv4_UC)
	if err != nil {
		return err
	}

	path, err := ParsePath(rf, args)
	if err != nil {
		cmdstr := "global"
		if resource == CMD_VRF {
			cmdstr = fmt.Sprintf("vrf %s", name)
		}
		rdHelpMsgFmt := `
    <RD> : xxx:yyy, xxx.xxx.xxx.xxx:yyy, xxx.xxx:yyy`
		ss := make([]string, 0, len(bgp.ProtocolNameMap))
		for _, v := range bgp.ProtocolNameMap {
			ss = append(ss, v)
		}
		sort.SliceStable(ss, func(i, j int) bool { return ss[i] < ss[j] })
		ss = append(ss, "<DEC_NUM>")
		ipProtocols := strings.Join(ss, ", ")
		ss = make([]string, 0, len(bgp.TCPFlagNameMap))
		for _, v := range bgp.TCPSortedFlags {
			ss = append(ss, bgp.TCPFlagNameMap[v])
		}
		tcpFlags := strings.Join(ss, ", ")
		ss = make([]string, 0, len(bgp.EthernetTypeNameMap))
		for _, v := range bgp.EthernetTypeNameMap {
			ss = append(ss, v)
		}
		sort.SliceStable(ss, func(i, j int) bool { return ss[i] < ss[j] })
		ss = append(ss, "<DEC_NUM>")
		etherTypes := strings.Join(ss, ", ")
		helpErrMap := map[bgp.RouteFamily]error{}
		helpErrMap[bgp.RF_IPv4_UC] = fmt.Errorf("usage: %s rib %s <PREFIX> [identifier <VALUE>] [origin { igp | egp | incomplete }] [aspath <VALUE>] [nexthop <ADDRESS>] [med <VALUE>] [local-pref <VALUE>] [community <VALUE>] [aigp metric <METRIC>] [large-community <VALUE> ] -a ipv4", cmdstr, modtype)
		helpErrMap[bgp.RF_IPv6_UC] = fmt.Errorf("usage: %s rib %s <PREFIX> [identifier <VALUE>] [origin { igp | egp | incomplete }] [aspath <VALUE>] [nexthop <ADDRESS>] [med <VALUE>] [local-pref <VALUE>] [community <VALUE>] [aigp metric <METRIC>] [large-community <VALUE> ] -a ipv6", cmdstr, modtype)
		fsHelpMsgFmt := fmt.Sprintf(`err: %s
usage: %s rib -a %%s %s%%s match <MATCH> then <THEN>%%s%%s%%s
    <THEN> : { %s |
               %s |
               %s <RATE> [as <AS>] |
               %s <RT> |
               %s <DEC_NUM> |
               %s { sample | terminal | sample-terminal } }...
    <RT> : xxx:yyy, xxx.xxx.xxx.xxx:yyy, xxxx::xxxx:yyy, xxx.xxx:yyy`,
			err,
			cmdstr,
			// <address family>
			modtype,
			// "" or " rd <RD>"
			// "" or " [rt <RT>]"
			// <help message for RD>
			// <MATCH>
			ExtCommNameMap[ACCEPT],
			ExtCommNameMap[DISCARD],
			ExtCommNameMap[RATE],
			ExtCommNameMap[REDIRECT],
			ExtCommNameMap[MARK],
			ExtCommNameMap[ACTION],
		)
		baseFsMatchExpr := fmt.Sprintf(`
    <MATCH> : { %s <PREFIX> [<OFFSET>] |
                %s <PREFIX> [<OFFSET>] |
                %s <PROTOCOLS>... |
                %s <FRAGMENTS>... |
                %s <TCP_FLAGS>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... %%s}...
    <PROTOCOLS> : [&] [<|<=|>|>=|==|!=] <PROTOCOL>
    <PROTOCOL> : %s
    <FRAGMENTS> : [&] [=|!|!=] <FRAGMENT>
    <FRAGMENT> : dont-fragment, is-fragment, first-fragment, last-fragment, not-a-fragment
    <TCP_FLAGS> : [&] [=|!|!=] <TCP_FLAG>
    <TCP_FLAG> : %s%%s
    <ITEM> : [&] [<|<=|>|>=|==|!=] <DEC_NUM>`,
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_DST_PREFIX],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_SRC_PREFIX],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_IP_PROTO],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_FRAGMENT],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_TCP_FLAG],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_PORT],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_DST_PORT],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_SRC_PORT],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_ICMP_TYPE],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_ICMP_CODE],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_PKT_LEN],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_DSCP],
			// <additional help messages if exists>
			ipProtocols,
			tcpFlags,
			// <additional help messages if exists>
		)
		ipv4FsMatchExpr := fmt.Sprintf(baseFsMatchExpr, "", "")
		ipv6FsMatchExpr := fmt.Sprintf(baseFsMatchExpr, fmt.Sprintf(`|
                %s <ITEM>... `,
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_LABEL]), "")
		l2vpnFsMatchExpr := fmt.Sprintf(baseFsMatchExpr, fmt.Sprintf(`|
                %s <ITEM>... |
                %s <MAC_ADDRESS> |
                %s <MAC_ADDRESS> |
                %s <ETHER_TYPES>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... |
                %s <ITEM>... `,
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_LABEL],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_DST_MAC],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_SRC_MAC],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_ETHERNET_TYPE],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_LLC_DSAP],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_LLC_SSAP],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_LLC_CONTROL],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_SNAP],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_VID],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_COS],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_INNER_VID],
			bgp.FlowSpecNameMap[bgp.FLOW_SPEC_TYPE_INNER_COS]), fmt.Sprintf(`
    <ETHER_TYPES> : [&] [<|<=|>|>=|==|!=] <ETHER_TYPE>
    <ETHER_TYPE> : %s`,
			etherTypes))
		helpErrMap[bgp.RF_FS_IPv4_UC] = fmt.Errorf(fsHelpMsgFmt, "ipv4-flowspec", "", "", "", ipv4FsMatchExpr)
		helpErrMap[bgp.RF_FS_IPv6_UC] = fmt.Errorf(fsHelpMsgFmt, "ipv6-flowspec", "", "", "", ipv6FsMatchExpr)
		helpErrMap[bgp.RF_FS_IPv4_VPN] = fmt.Errorf(fsHelpMsgFmt, "ipv4-l3vpn-flowspec", " rd <RD>", " [rt <RT>]", rdHelpMsgFmt, ipv4FsMatchExpr)
		helpErrMap[bgp.RF_FS_IPv6_VPN] = fmt.Errorf(fsHelpMsgFmt, "ipv6-l3vpn-flowspec", " rd <RD>", " [rt <RT>]", rdHelpMsgFmt, ipv6FsMatchExpr)
		helpErrMap[bgp.RF_FS_L2_VPN] = fmt.Errorf(fsHelpMsgFmt, "l2vpn-flowspec", " rd <RD>", " [rt <RT>]", rdHelpMsgFmt, l2vpnFsMatchExpr)
		helpErrMap[bgp.RF_EVPN] = fmt.Errorf(`usage: %s rib %s { a-d <A-D> | macadv <MACADV> | multicast <MULTICAST> | esi <ESI> | prefix <PREFIX> } -a evpn
    <A-D>       : esi <esi> etag <etag> label <label> rd <rd> [rt <rt>...] [encap <encap type>] [esi-label <esi-label> [single-active | all-active]]
    <MACADV>    : <mac address> <ip address> [esi <esi>] etag <etag> label <label> rd <rd> [rt <rt>...] [encap <encap type>] [default-gateway]
    <MULTICAST> : <ip address> etag <etag> rd <rd> [rt <rt>...] [encap <encap type>]
    <ESI>       : <ip address> esi <esi> rd <rd> [rt <rt>...] [encap <encap type>]
    <PREFIX>    : <ip prefix> [gw <gateway>] [esi <esi>] etag <etag> [label <label>] rd <rd> [rt <rt>...] [encap <encap type>] [router-mac <mac address>]`, cmdstr, modtype)
		helpErrMap[bgp.RF_OPAQUE] = fmt.Errorf(`usage: %s rib %s key <KEY> [value <VALUE>]`, cmdstr, modtype)
		if err, ok := helpErrMap[rf]; ok {
			return err
		}
		return err
	}

	if modtype == CMD_ADD {
		if resource == CMD_VRF {
			_, err = client.AddVRFPath(name, []*table.Path{path})
		} else {
			_, err = client.AddPath([]*table.Path{path})
		}
	} else {
		if resource == CMD_VRF {
			err = client.DeleteVRFPath(name, []*table.Path{path})
		} else {
			err = client.DeletePath([]*table.Path{path})
		}
	}
	return err
}

func showGlobalConfig() error {
	g, err := client.GetServer()
	if err != nil {
		return err
	}
	if globalOpts.Json {
		j, _ := json.Marshal(g)
		fmt.Println(string(j))
		return nil
	}
	fmt.Println("AS:       ", g.Config.As)
	fmt.Println("Router-ID:", g.Config.RouterId)
	if len(g.Config.LocalAddressList) > 0 {
		fmt.Printf("Listening Port: %d, Addresses: %s\n", g.Config.Port, strings.Join(g.Config.LocalAddressList, ", "))
	}
	if g.UseMultiplePaths.Config.Enabled {
		fmt.Printf("Multipath: enabled")
	}
	return nil
}

func modGlobalConfig(args []string) error {
	m := extractReserved(args, []string{"as", "router-id", "listen-port",
		"listen-addresses", "use-multipath"})

	if len(m["as"]) != 1 || len(m["router-id"]) != 1 {
		return fmt.Errorf("usage: gobgp global as <VALUE> router-id <VALUE> [use-multipath] [listen-port <VALUE>] [listen-addresses <VALUE>...]")
	}
	asn, err := strconv.ParseUint(m["as"][0], 10, 32)
	if err != nil {
		return err
	}
	id := net.ParseIP(m["router-id"][0])
	if id.To4() == nil {
		return fmt.Errorf("invalid router-id format")
	}
	var port int64
	if len(m["listen-port"]) > 0 {
		// Note: GlobalConfig.Port is uint32 type, but the TCP/UDP port is
		// 16-bit length.
		port, err = strconv.ParseInt(m["listen-port"][0], 10, 16)
		if err != nil {
			return err
		}
	}
	useMultipath := false
	if _, ok := m["use-multipath"]; ok {
		useMultipath = true
	}
	return client.StartServer(&config.Global{
		Config: config.GlobalConfig{
			As:               uint32(asn),
			RouterId:         id.String(),
			Port:             int32(port),
			LocalAddressList: m["listen-addresses"],
		},
		UseMultiplePaths: config.UseMultiplePaths{
			Config: config.UseMultiplePathsConfig{
				Enabled: useMultipath,
			},
		},
	})
}

func NewGlobalCmd() *cobra.Command {
	globalCmd := &cobra.Command{
		Use: CMD_GLOBAL,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if len(args) != 0 {
				err = modGlobalConfig(args)
			} else {
				err = showGlobalConfig()
			}
			if err != nil {
				exitWithError(err)
			}
		},
	}

	ribCmd := &cobra.Command{
		Use: CMD_RIB,
		Run: func(cmd *cobra.Command, args []string) {
			if err := showGlobalRib(args); err != nil {
				exitWithError(err)
			}
		},
	}

	ribCmd.PersistentFlags().StringVarP(&subOpts.AddressFamily, "address-family", "a", "", "address family")

	for _, v := range []string{CMD_ADD, CMD_DEL} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(cmd *cobra.Command, args []string) {
				err := modPath(CMD_GLOBAL, "", cmd.Use, args)
				if err != nil {
					exitWithError(err)
				}
			},
		}
		ribCmd.AddCommand(cmd)

		if v == CMD_DEL {
			subcmd := &cobra.Command{
				Use: CMD_ALL,
				Run: func(cmd *cobra.Command, args []string) {
					family, err := checkAddressFamily(bgp.RouteFamily(0))
					if err != nil {
						exitWithError(err)
					}
					if err = client.DeletePathByFamily(family); err != nil {
						exitWithError(err)
					}
				},
			}
			cmd.AddCommand(subcmd)
		}
	}

	summaryCmd := &cobra.Command{
		Use: CMD_SUMMARY,
		Run: func(cmd *cobra.Command, args []string) {
			if err := showRibInfo(CMD_GLOBAL, ""); err != nil {
				exitWithError(err)
			}
		},
	}
	ribCmd.AddCommand(summaryCmd)

	policyCmd := &cobra.Command{
		Use: CMD_POLICY,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				exitWithError(fmt.Errorf("usage: gobgp global policy [{ import | export }]"))
			}
			for _, v := range []string{CMD_IMPORT, CMD_EXPORT} {
				if err := showNeighborPolicy("", v, 4); err != nil {
					exitWithError(err)
				}
			}
		},
	}

	for _, v := range []string{CMD_IMPORT, CMD_EXPORT} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(cmd *cobra.Command, args []string) {
				if err := showNeighborPolicy("", cmd.Use, 0); err != nil {
					exitWithError(err)
				}
			},
		}

		for _, w := range []string{CMD_ADD, CMD_DEL, CMD_SET} {
			subcmd := &cobra.Command{
				Use: w,
				Run: func(subcmd *cobra.Command, args []string) {
					err := modNeighborPolicy("", cmd.Use, subcmd.Use, args)
					if err != nil {
						exitWithError(err)
					}
				},
			}
			cmd.AddCommand(subcmd)
		}

		policyCmd.AddCommand(cmd)
	}

	delCmd := &cobra.Command{
		Use: CMD_DEL,
	}

	allCmd := &cobra.Command{
		Use: CMD_ALL,
		Run: func(cmd *cobra.Command, args []string) {
			if err := client.StopServer(); err != nil {
				exitWithError(err)
			}
		},
	}
	delCmd.AddCommand(allCmd)

	globalCmd.AddCommand(ribCmd, policyCmd, delCmd)
	return globalCmd
}
