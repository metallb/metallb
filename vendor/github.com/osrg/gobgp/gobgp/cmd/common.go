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
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	cli "github.com/osrg/gobgp/client"
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

const (
	CMD_GLOBAL         = "global"
	CMD_NEIGHBOR       = "neighbor"
	CMD_POLICY         = "policy"
	CMD_RIB            = "rib"
	CMD_ADD            = "add"
	CMD_DEL            = "del"
	CMD_ALL            = "all"
	CMD_SET            = "set"
	CMD_LOCAL          = "local"
	CMD_ADJ_IN         = "adj-in"
	CMD_ADJ_OUT        = "adj-out"
	CMD_RESET          = "reset"
	CMD_SOFT_RESET     = "softreset"
	CMD_SOFT_RESET_IN  = "softresetin"
	CMD_SOFT_RESET_OUT = "softresetout"
	CMD_SHUTDOWN       = "shutdown"
	CMD_ENABLE         = "enable"
	CMD_DISABLE        = "disable"
	CMD_PREFIX         = "prefix"
	CMD_ASPATH         = "as-path"
	CMD_COMMUNITY      = "community"
	CMD_EXTCOMMUNITY   = "ext-community"
	CMD_IMPORT         = "import"
	CMD_EXPORT         = "export"
	CMD_IN             = "in"
	CMD_MONITOR        = "monitor"
	CMD_MRT            = "mrt"
	CMD_DUMP           = "dump"
	CMD_INJECT         = "inject"
	CMD_RPKI           = "rpki"
	CMD_RPKI_TABLE     = "table"
	CMD_RPKI_SERVER    = "server"
	CMD_VRF            = "vrf"
	CMD_ACCEPTED       = "accepted"
	CMD_REJECTED       = "rejected"
	CMD_STATEMENT      = "statement"
	CMD_CONDITION      = "condition"
	CMD_ACTION         = "action"
	CMD_UPDATE         = "update"
	CMD_ROTATE         = "rotate"
	CMD_BMP            = "bmp"
	CMD_LARGECOMMUNITY = "large-community"
	CMD_SUMMARY        = "summary"
	CMD_VALIDATION     = "validation"
)

var subOpts struct {
	AddressFamily string `short:"a" long:"address-family" description:"specifying an address family"`
}

var neighborsOpts struct {
	Reason    string `short:"r" long:"reason" description:"specifying communication field on Cease NOTIFICATION message with Administrative Shutdown subcode"`
	Transport string `short:"t" long:"transport" description:"specifying a transport protocol"`
}

var conditionOpts struct {
	Prefix       string `long:"prefix" description:"specifying a prefix set name of policy"`
	Neighbor     string `long:"neighbor" description:"specifying a neighbor set name of policy"`
	AsPath       string `long:"aspath" description:"specifying an as set name of policy"`
	Community    string `long:"community" description:"specifying a community set name of policy"`
	ExtCommunity string `long:"extcommunity" description:"specifying a extended community set name of policy"`
	AsPathLength string `long:"aspath-len" description:"specifying an as path length of policy (<operator>,<numeric>)"`
}

var actionOpts struct {
	RouteAction         string `long:"route-action" description:"specifying a route action of policy (accept | reject)"`
	CommunityAction     string `long:"community" description:"specifying a community action of policy"`
	MedAction           string `long:"med" description:"specifying a med action of policy"`
	AsPathPrependAction string `long:"as-prepend" description:"specifying a as-prepend action of policy"`
	NexthopAction       string `long:"next-hop" description:"specifying a next-hop action of policy"`
}

var mrtOpts struct {
	OutputDir   string
	FileFormat  string
	Filename    string `long:"filename" description:"MRT file name"`
	RecordCount int64  `long:"count" description:"Number of records to inject"`
	RecordSkip  int64  `long:"skip" description:"Number of records to skip before injecting"`
	QueueSize   int    `long:"batch-size" description:"Maximum number of updates to keep queued"`
	Best        bool   `long:"only-best" description:"only keep best path routes"`
	SkipV4      bool   `long:"no-ipv4" description:"Skip importing IPv4 routes"`
	SkipV6      bool   `long:"no-ipv4" description:"Skip importing IPv6 routes"`
	NextHop     net.IP `long:"nexthop" description:"Rewrite nexthop"`
}

func formatTimedelta(d int64) string {
	u := uint64(d)
	neg := d < 0
	if neg {
		u = -u
	}
	secs := u % 60
	u /= 60
	mins := u % 60
	u /= 60
	hours := u % 24
	days := u / 24

	if days == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
	} else {
		return fmt.Sprintf("%dd ", days) + fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
	}
}

func cidr2prefix(cidr string) string {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return cidr
	}
	var buffer bytes.Buffer
	for i := 0; i < len(n.IP); i++ {
		buffer.WriteString(fmt.Sprintf("%08b", n.IP[i]))
	}
	ones, _ := n.Mask.Size()
	return buffer.String()[:ones]
}

func extractReserved(args, keys []string) map[string][]string {
	m := make(map[string][]string, len(keys))
	var k string
	isReserved := func(s string) bool {
		for _, r := range keys {
			if s == r {
				return true
			}
		}
		return false
	}
	for _, arg := range args {
		if isReserved(arg) {
			k = arg
			m[k] = make([]string, 0, 1)
		} else {
			m[k] = append(m[k], arg)
		}
	}
	return m
}

type neighbors []*config.Neighbor

func (n neighbors) Len() int {
	return len(n)
}

func (n neighbors) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n neighbors) Less(i, j int) bool {
	p1 := n[i].State.NeighborAddress
	p2 := n[j].State.NeighborAddress
	p1Isv4 := !strings.Contains(p1, ":")
	p2Isv4 := !strings.Contains(p2, ":")
	if p1Isv4 != p2Isv4 {
		if p1Isv4 {
			return true
		}
		return false
	}
	addrlen := 128
	if p1Isv4 {
		addrlen = 32
	}
	strings := sort.StringSlice{cidr2prefix(fmt.Sprintf("%s/%d", p1, addrlen)),
		cidr2prefix(fmt.Sprintf("%s/%d", p2, addrlen))}
	return strings.Less(0, 1)
}

type capabilities []bgp.ParameterCapabilityInterface

func (c capabilities) Len() int {
	return len(c)
}

func (c capabilities) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c capabilities) Less(i, j int) bool {
	return c[i].Code() < c[j].Code()
}

type vrfs []*table.Vrf

func (v vrfs) Len() int {
	return len(v)
}

func (v vrfs) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

func (v vrfs) Less(i, j int) bool {
	return v[i].Name < v[j].Name
}

func newClient() *cli.Client {
	var grpcOpts []grpc.DialOption
	if globalOpts.TLS {
		var creds credentials.TransportCredentials
		if globalOpts.CaFile == "" {
			creds = credentials.NewClientTLSFromCert(nil, "")
		} else {
			var err error
			creds, err = credentials.NewClientTLSFromFile(globalOpts.CaFile, "")
			if err != nil {
				exitWithError(err)
			}
		}
		grpcOpts = []grpc.DialOption{
			grpc.WithTimeout(time.Second),
			grpc.WithBlock(),
			grpc.WithTransportCredentials(creds),
		}
	}
	target := net.JoinHostPort(globalOpts.Host, strconv.Itoa(globalOpts.Port))
	client, err := cli.New(target, grpcOpts...)
	if err != nil {
		exitWithError(err)
	}
	return client
}

func addr2AddressFamily(a net.IP) bgp.RouteFamily {
	if a.To4() != nil {
		return bgp.RF_IPv4_UC
	} else if a.To16() != nil {
		return bgp.RF_IPv6_UC
	}
	return bgp.RouteFamily(0)
}

func checkAddressFamily(def bgp.RouteFamily) (bgp.RouteFamily, error) {
	var rf bgp.RouteFamily
	var e error
	switch subOpts.AddressFamily {
	case "ipv4", "v4", "4":
		rf = bgp.RF_IPv4_UC
	case "ipv6", "v6", "6":
		rf = bgp.RF_IPv6_UC
	case "ipv4-l3vpn", "vpnv4", "vpn-ipv4":
		rf = bgp.RF_IPv4_VPN
	case "ipv6-l3vpn", "vpnv6", "vpn-ipv6":
		rf = bgp.RF_IPv6_VPN
	case "ipv4-labeled", "ipv4-labelled", "ipv4-mpls":
		rf = bgp.RF_IPv4_MPLS
	case "ipv6-labeled", "ipv6-labelled", "ipv6-mpls":
		rf = bgp.RF_IPv6_MPLS
	case "evpn":
		rf = bgp.RF_EVPN
	case "encap", "ipv4-encap":
		rf = bgp.RF_IPv4_ENCAP
	case "ipv6-encap":
		rf = bgp.RF_IPv6_ENCAP
	case "rtc":
		rf = bgp.RF_RTC_UC
	case "ipv4-flowspec", "ipv4-flow", "flow4":
		rf = bgp.RF_FS_IPv4_UC
	case "ipv6-flowspec", "ipv6-flow", "flow6":
		rf = bgp.RF_FS_IPv6_UC
	case "ipv4-l3vpn-flowspec", "ipv4vpn-flowspec", "flowvpn4":
		rf = bgp.RF_FS_IPv4_VPN
	case "ipv6-l3vpn-flowspec", "ipv6vpn-flowspec", "flowvpn6":
		rf = bgp.RF_FS_IPv6_VPN
	case "l2vpn-flowspec":
		rf = bgp.RF_FS_L2_VPN
	case "opaque":
		rf = bgp.RF_OPAQUE
	case "":
		rf = def
	default:
		e = fmt.Errorf("unsupported address family: %s", subOpts.AddressFamily)
	}
	return rf, e
}

func printError(err error) {
	if globalOpts.Json {
		j, _ := json.Marshal(struct {
			Error string `json:"error"`
		}{Error: err.Error()})
		fmt.Println(string(j))
	} else {
		fmt.Println(err)
	}
}

func exitWithError(err error) {
	printError(err)
	os.Exit(1)
}
