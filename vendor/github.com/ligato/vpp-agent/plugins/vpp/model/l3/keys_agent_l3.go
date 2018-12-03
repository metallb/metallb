// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package l3

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Prefixes
const (
	// VrfPrefix is the relative key prefix for VRFs.
	VrfPrefix = "vpp/config/v1/vrf/"
	// RoutesPrefix is the relative key prefix for routes.
	RoutesPrefix = VrfPrefix + "{vrf}/fib/{net}/{mask}/{next-hop}"
	// ArpPrefix is the relative key prefix for ARP.
	ArpPrefix = "vpp/config/v1/arp/"
	// ArpEntryPrefix is the relative key prefix for ARP table entries.
	ArpKey = ArpPrefix + "{if}/{ip}"
	// ProxyARPPrefix is the relative key prefix for proxy ARP configuration.
	ProxyARPRangePrefix = "vpp/config/v1/proxyarp/range/"
	// ProxyARPPrefix is the relative key prefix for proxy ARP configuration.
	ProxyARPInterfacePrefix = "vpp/config/v1/proxyarp/interface/"
	// ProxyARPRangePrefix is the relative key prefix for proxy ARP ranges.
	ProxyARPRangeKey = ProxyARPRangePrefix + "{label}"
	// ProxyARPInterfacePrefix is the relative key prefix for proxy ARP-enabled interfaces.
	ProxyARPInterfaceKey = ProxyARPInterfacePrefix + "{label}"
	// IPScanNeighPrefix it the relative key prefix for IP scan neighbor feature
	IPScanNeighPrefix = "vpp/config/v1/ipneigh/"
)

// RouteKey returns the key used in ETCD to store vpp route for vpp instance.
func RouteKey(vrf uint32, dstAddr string, nextHopAddr string) string {
	_, dstNet, _ := net.ParseCIDR(dstAddr)
	dstNetAddr := dstNet.IP.String()
	dstNetMask, _ := dstNet.Mask.Size()
	key := RoutesPrefix
	key = strings.Replace(key, "{vrf}", strconv.Itoa(int(vrf)), 1)
	key = strings.Replace(key, "{net}", dstNetAddr, 1)
	key = strings.Replace(key, "{mask}", strconv.Itoa(dstNetMask), 1)
	key = strings.Replace(key, "{next-hop}", nextHopAddr, 1)
	return key
}

// ParseRouteKey parses VRF label and route address from a route key.
func ParseRouteKey(key string) (isRouteKey bool, vrfIndex string, dstNetAddr string, dstNetMask int, nextHopAddr string) {
	if strings.HasPrefix(key, VrfPrefix) {
		vrfSuffix := strings.TrimPrefix(key, VrfPrefix)
		routeComps := strings.Split(vrfSuffix, "/")
		if len(routeComps) >= 5 && routeComps[1] == "fib" {
			if mask, err := strconv.Atoi(routeComps[3]); err == nil {
				return true, routeComps[0], routeComps[2], mask, routeComps[4]
			}
		} else if len(routeComps) == 4 && routeComps[1] == "fib" {
			if mask, err := strconv.Atoi(routeComps[3]); err == nil {
				return true, routeComps[0], routeComps[2], mask, ""
			}
		}
	}
	return false, "", "", 0, ""
}

// ArpEntryKey returns the key to store ARP entry
func ArpEntryKey(iface, ipAddr string) string {
	key := ArpKey
	key = strings.Replace(key, "{if}", iface, 1)
	key = strings.Replace(key, "{ip}", ipAddr, 1)
	//key = strings.Replace(key, "{mac}", macAddr, 1)
	return key
}

// ParseArpKey parses ARP entry from a key
func ParseArpKey(key string) (iface string, ipAddr string, err error) {
	if strings.HasPrefix(key, ArpPrefix) {
		arpSuffix := strings.TrimPrefix(key, ArpPrefix)
		arpComps := strings.Split(arpSuffix, "/")
		if len(arpComps) == 2 {
			return arpComps[0], arpComps[1], nil
		}
	}
	return "", "", fmt.Errorf("invalid ARP key")
}

// ProxyArpRangeKey returns the key to store Proxy ARP range config
func ProxyArpRangeKey(label string) string {
	key := ProxyARPRangeKey
	key = strings.Replace(key, "{label}", label, 1)
	return key
}

// ProxyArpInterfaceKey returns the key to store Proxy ARP interface config
func ProxyArpInterfaceKey(label string) string {
	key := ProxyARPInterfaceKey
	key = strings.Replace(key, "{label}", label, 1)
	return key
}
