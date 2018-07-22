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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
)

// used in showRoute() to determine the width of each column
var (
	columnWidthPrefix  = 20
	columnWidthNextHop = 20
	columnWidthAsPath  = 20
	columnWidthLabel   = 10
)

func updateColumnWidth(nlri, nexthop, aspath, label string) {
	if prefixLen := len(nlri); columnWidthPrefix < prefixLen {
		columnWidthPrefix = prefixLen
	}
	if columnWidthNextHop < len(nexthop) {
		columnWidthNextHop = len(nexthop)
	}
	if columnWidthAsPath < len(aspath) {
		columnWidthAsPath = len(aspath)
	}
	if columnWidthLabel < len(label) {
		columnWidthLabel = len(label)
	}
}

func getNeighbors(vrf string) (neighbors, error) {
	if vrf != "" {
		n, err := client.ListNeighborByVRF(vrf)
		return neighbors(n), err
	} else if t := neighborsOpts.Transport; t != "" {
		switch t {
		case "ipv4":
			n, err := client.ListNeighborByTransport(bgp.AFI_IP)
			return neighbors(n), err
		case "ipv6":
			n, err := client.ListNeighborByTransport(bgp.AFI_IP6)
			return neighbors(n), err
		default:
			return nil, fmt.Errorf("invalid transport: %s", t)
		}
	}
	n, err := client.ListNeighbor()
	return neighbors(n), err
}

func getASN(p *config.Neighbor) string {
	asn := "*"
	if p.State.PeerAs > 0 {
		asn = fmt.Sprint(p.State.PeerAs)
	}
	return asn
}

func showNeighbors(vrf string) error {
	m, err := getNeighbors(vrf)
	if err != nil {
		return err
	}
	if globalOpts.Json {
		j, _ := json.Marshal(m)
		fmt.Println(string(j))
		return nil
	}

	if globalOpts.Quiet {
		for _, p := range m {
			fmt.Println(p.State.NeighborAddress)
		}
		return nil
	}
	maxaddrlen := 0
	maxaslen := 2
	maxtimelen := len("Up/Down")
	timedelta := []string{}

	sort.Sort(m)

	now := time.Now()
	for _, n := range m {
		if i := len(n.Config.NeighborInterface); i > maxaddrlen {
			maxaddrlen = i
		} else if j := len(n.State.NeighborAddress); j > maxaddrlen {
			maxaddrlen = j
		}
		if l := len(getASN(n)); l > maxaslen {
			maxaslen = l
		}
		timeStr := "never"
		if n.Timers.State.Uptime != 0 {
			t := int64(n.Timers.State.Downtime)
			if n.State.SessionState == config.SESSION_STATE_ESTABLISHED {
				t = int64(n.Timers.State.Uptime)
			}
			timeStr = formatTimedelta(int64(now.Sub(time.Unix(int64(t), 0)).Seconds()))
		}
		if len(timeStr) > maxtimelen {
			maxtimelen = len(timeStr)
		}
		timedelta = append(timedelta, timeStr)
	}

	format := "%-" + fmt.Sprint(maxaddrlen) + "s" + " %" + fmt.Sprint(maxaslen) + "s" + " %" + fmt.Sprint(maxtimelen) + "s"
	format += " %-11s |%9s %9s\n"
	fmt.Printf(format, "Peer", "AS", "Up/Down", "State", "#Received", "Accepted")
	formatFsm := func(admin config.AdminState, fsm config.SessionState) string {
		switch admin {
		case config.ADMIN_STATE_DOWN:
			return "Idle(Admin)"
		case config.ADMIN_STATE_PFX_CT:
			return "Idle(PfxCt)"
		}

		switch fsm {
		case config.SESSION_STATE_IDLE:
			return "Idle"
		case config.SESSION_STATE_CONNECT:
			return "Connect"
		case config.SESSION_STATE_ACTIVE:
			return "Active"
		case config.SESSION_STATE_OPENSENT:
			return "Sent"
		case config.SESSION_STATE_OPENCONFIRM:
			return "Confirm"
		case config.SESSION_STATE_ESTABLISHED:
			return "Establ"
		default:
			return string(fsm)
		}
	}

	for i, n := range m {
		neigh := n.State.NeighborAddress
		if n.Config.NeighborInterface != "" {
			neigh = n.Config.NeighborInterface
		}
		fmt.Printf(format, neigh, getASN(n), timedelta[i], formatFsm(n.State.AdminState, n.State.SessionState), fmt.Sprint(n.State.AdjTable.Received), fmt.Sprint(n.State.AdjTable.Accepted))
	}

	return nil
}

func showNeighbor(args []string) error {
	p, e := client.GetNeighbor(args[0], true)
	if e != nil {
		return e
	}
	if globalOpts.Json {
		j, _ := json.Marshal(p)
		fmt.Println(string(j))
		return nil
	}

	fmt.Printf("BGP neighbor is %s, remote AS %s", p.State.NeighborAddress, getASN(p))

	if p.RouteReflector.Config.RouteReflectorClient {
		fmt.Printf(", route-reflector-client\n")
	} else if p.RouteServer.Config.RouteServerClient {
		fmt.Printf(", route-server-client\n")
	} else {
		fmt.Printf("\n")
	}

	id := "unknown"
	if p.State.RemoteRouterId != "" {
		id = p.State.RemoteRouterId
	}
	fmt.Printf("  BGP version 4, remote router ID %s\n", id)
	fmt.Printf("  BGP state = %s", p.State.SessionState)
	if p.Timers.State.Uptime > 0 {
		fmt.Printf(", up for %s\n", formatTimedelta(int64(p.Timers.State.Uptime)-time.Now().Unix()))
	} else {
		fmt.Print("\n")
	}
	fmt.Printf("  BGP OutQ = %d, Flops = %d\n", p.State.Queues.Output, p.State.Flops)
	fmt.Printf("  Hold time is %d, keepalive interval is %d seconds\n", int(p.Timers.State.NegotiatedHoldTime), int(p.Timers.State.KeepaliveInterval))
	fmt.Printf("  Configured hold time is %d, keepalive interval is %d seconds\n", int(p.Timers.Config.HoldTime), int(p.Timers.Config.KeepaliveInterval))

	elems := make([]string, 0, 3)
	if as := p.AsPathOptions.Config.AllowOwnAs; as > 0 {
		elems = append(elems, fmt.Sprintf("Allow Own AS: %d", as))
	}
	switch p.Config.RemovePrivateAs {
	case config.REMOVE_PRIVATE_AS_OPTION_ALL:
		elems = append(elems, "Remove private AS: all")
	case config.REMOVE_PRIVATE_AS_OPTION_REPLACE:
		elems = append(elems, "Remove private AS: replace")
	}
	if p.AsPathOptions.Config.ReplacePeerAs {
		elems = append(elems, "Replace peer AS: enabled")
	}

	fmt.Printf("  %s\n", strings.Join(elems, ", "))

	fmt.Printf("  Neighbor capabilities:\n")
	caps := capabilities{}
	lookup := func(val bgp.ParameterCapabilityInterface, l capabilities) bgp.ParameterCapabilityInterface {
		for _, v := range l {
			if v.Code() == val.Code() {
				if v.Code() == bgp.BGP_CAP_MULTIPROTOCOL {
					lhs := v.(*bgp.CapMultiProtocol).CapValue
					rhs := val.(*bgp.CapMultiProtocol).CapValue
					if lhs == rhs {
						return v
					}
					continue
				}
				return v
			}
		}
		return nil
	}
	for _, c := range p.State.LocalCapabilityList {
		caps = append(caps, c)
	}
	for _, c := range p.State.RemoteCapabilityList {
		if lookup(c, caps) == nil {
			caps = append(caps, c)
		}
	}

	sort.Sort(caps)

	firstMp := true

	for _, c := range caps {
		support := ""
		if m := lookup(c, p.State.LocalCapabilityList); m != nil {
			support += "advertised"
		}
		if lookup(c, p.State.RemoteCapabilityList) != nil {
			if len(support) != 0 {
				support += " and "
			}
			support += "received"
		}

		switch c.Code() {
		case bgp.BGP_CAP_MULTIPROTOCOL:
			if firstMp {
				fmt.Printf("    %s:\n", c.Code())
				firstMp = false
			}
			m := c.(*bgp.CapMultiProtocol).CapValue
			fmt.Printf("        %s:\t%s\n", m, support)
		case bgp.BGP_CAP_GRACEFUL_RESTART:
			fmt.Printf("    %s:\t%s\n", c.Code(), support)
			grStr := func(g *bgp.CapGracefulRestart) string {
				str := ""
				if len(g.Tuples) > 0 {
					str += fmt.Sprintf("restart time %d sec", g.Time)
				}
				if g.Flags&0x08 > 0 {
					if len(str) > 0 {
						str += ", "
					}
					str += "restart flag set"
				}
				if g.Flags&0x04 > 0 {
					if len(str) > 0 {
						str += ", "
					}
					str += "notification flag set"
				}

				if len(str) > 0 {
					str += "\n"
				}
				for _, t := range g.Tuples {
					str += fmt.Sprintf("	    %s", bgp.AfiSafiToRouteFamily(t.AFI, t.SAFI))
					if t.Flags == 0x80 {
						str += ", forward flag set"
					}
					str += "\n"
				}
				return str
			}
			if m := lookup(c, p.State.LocalCapabilityList); m != nil {
				g := m.(*bgp.CapGracefulRestart)
				if s := grStr(g); len(s) > 0 {
					fmt.Printf("        Local: %s", s)
				}
			}
			if m := lookup(c, p.State.RemoteCapabilityList); m != nil {
				g := m.(*bgp.CapGracefulRestart)
				if s := grStr(g); len(s) > 0 {
					fmt.Printf("        Remote: %s", s)
				}
			}
		case bgp.BGP_CAP_LONG_LIVED_GRACEFUL_RESTART:
			fmt.Printf("    %s:\t%s\n", c.Code(), support)
			grStr := func(g *bgp.CapLongLivedGracefulRestart) string {
				var str string
				for _, t := range g.Tuples {
					str += fmt.Sprintf("	    %s, restart time %d sec", bgp.AfiSafiToRouteFamily(t.AFI, t.SAFI), t.RestartTime)
					if t.Flags == 0x80 {
						str += ", forward flag set"
					}
					str += "\n"
				}
				return str
			}
			if m := lookup(c, p.State.LocalCapabilityList); m != nil {
				g := m.(*bgp.CapLongLivedGracefulRestart)
				if s := grStr(g); len(s) > 0 {
					fmt.Printf("        Local:\n%s", s)
				}
			}
			if m := lookup(c, p.State.RemoteCapabilityList); m != nil {
				g := m.(*bgp.CapLongLivedGracefulRestart)
				if s := grStr(g); len(s) > 0 {
					fmt.Printf("        Remote:\n%s", s)
				}
			}
		case bgp.BGP_CAP_EXTENDED_NEXTHOP:
			fmt.Printf("    %s:\t%s\n", c.Code(), support)
			exnhStr := func(e *bgp.CapExtendedNexthop) string {
				lines := make([]string, 0, len(e.Tuples))
				for _, t := range e.Tuples {
					var nhafi string
					switch int(t.NexthopAFI) {
					case bgp.AFI_IP:
						nhafi = "ipv4"
					case bgp.AFI_IP6:
						nhafi = "ipv6"
					default:
						nhafi = fmt.Sprintf("%d", t.NexthopAFI)
					}
					line := fmt.Sprintf("nlri: %s, nexthop: %s", bgp.AfiSafiToRouteFamily(t.NLRIAFI, uint8(t.NLRISAFI)), nhafi)
					lines = append(lines, line)
				}
				return strings.Join(lines, "\n")
			}
			if m := lookup(c, p.State.LocalCapabilityList); m != nil {
				e := m.(*bgp.CapExtendedNexthop)
				if s := exnhStr(e); len(s) > 0 {
					fmt.Printf("        Local:  %s\n", s)
				}
			}
			if m := lookup(c, p.State.RemoteCapabilityList); m != nil {
				e := m.(*bgp.CapExtendedNexthop)
				if s := exnhStr(e); len(s) > 0 {
					fmt.Printf("        Remote: %s\n", s)
				}
			}
		case bgp.BGP_CAP_ADD_PATH:
			fmt.Printf("    %s:\t%s\n", c.Code(), support)
			if m := lookup(c, p.State.LocalCapabilityList); m != nil {
				fmt.Println("      Local:")
				for _, item := range m.(*bgp.CapAddPath).Tuples {
					fmt.Printf("         %s:\t%s\n", item.RouteFamily, item.Mode)
				}
			}
			if m := lookup(c, p.State.RemoteCapabilityList); m != nil {
				fmt.Println("      Remote:")
				for _, item := range m.(*bgp.CapAddPath).Tuples {
					fmt.Printf("         %s:\t%s\n", item.RouteFamily, item.Mode)
				}
			}
		default:
			fmt.Printf("    %s:\t%s\n", c.Code(), support)
		}
	}
	fmt.Print("  Message statistics:\n")
	fmt.Print("                         Sent       Rcvd\n")
	fmt.Printf("    Opens:         %10d %10d\n", p.State.Messages.Sent.Open, p.State.Messages.Received.Open)
	fmt.Printf("    Notifications: %10d %10d\n", p.State.Messages.Sent.Notification, p.State.Messages.Received.Notification)
	fmt.Printf("    Updates:       %10d %10d\n", p.State.Messages.Sent.Update, p.State.Messages.Received.Update)
	fmt.Printf("    Keepalives:    %10d %10d\n", p.State.Messages.Sent.Keepalive, p.State.Messages.Received.Keepalive)
	fmt.Printf("    Route Refresh: %10d %10d\n", p.State.Messages.Sent.Refresh, p.State.Messages.Received.Refresh)
	fmt.Printf("    Discarded:     %10d %10d\n", p.State.Messages.Sent.Discarded, p.State.Messages.Received.Discarded)
	fmt.Printf("    Total:         %10d %10d\n", p.State.Messages.Sent.Total, p.State.Messages.Received.Total)
	fmt.Print("  Route statistics:\n")
	fmt.Printf("    Advertised:    %10d\n", p.State.AdjTable.Advertised)
	fmt.Printf("    Received:      %10d\n", p.State.AdjTable.Received)
	fmt.Printf("    Accepted:      %10d\n", p.State.AdjTable.Accepted)
	first := true
	for _, afisafi := range p.AfiSafis {
		if afisafi.PrefixLimit.Config.MaxPrefixes > 0 {
			if first {
				fmt.Println("  Prefix Limits:")
				first = false
			}
			fmt.Printf("    %s:\tMaximum prefixes allowed %d", afisafi.Config.AfiSafiName, afisafi.PrefixLimit.Config.MaxPrefixes)
			if afisafi.PrefixLimit.Config.ShutdownThresholdPct > 0 {
				fmt.Printf(", Threshold for warning message %d%%\n", afisafi.PrefixLimit.Config.ShutdownThresholdPct)
			} else {
				fmt.Printf("\n")
			}
		}
	}
	return nil
}

type AsPathFormat struct{}

func getPathSymbolString(p *table.Path, idx int, showBest bool) string {
	symbols := ""
	if p.IsStale() {
		symbols += "S"
	}
	switch p.ValidationStatus() {
	case config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND:
		symbols += "N"
	case config.RPKI_VALIDATION_RESULT_TYPE_VALID:
		symbols += "V"
	case config.RPKI_VALIDATION_RESULT_TYPE_INVALID:
		symbols += "I"
	}
	if showBest {
		if idx == 0 && !p.IsNexthopInvalid {
			symbols += "*>"
		} else {
			symbols += "* "
		}
	}
	return symbols
}

func getPathAttributeString(p *table.Path) string {
	s := make([]string, 0)
	for _, a := range p.GetPathAttrs() {
		switch a.GetType() {
		case bgp.BGP_ATTR_TYPE_NEXT_HOP, bgp.BGP_ATTR_TYPE_MP_REACH_NLRI, bgp.BGP_ATTR_TYPE_AS_PATH, bgp.BGP_ATTR_TYPE_AS4_PATH:
			continue
		default:
			s = append(s, a.String())
		}
	}
	switch n := p.GetNlri().(type) {
	case *bgp.EVPNNLRI:
		// We print non route key fields like path attributes.
		switch route := n.RouteTypeData.(type) {
		case *bgp.EVPNMacIPAdvertisementRoute:
			s = append(s, fmt.Sprintf("[ESI: %s]", route.ESI.String()))
		case *bgp.EVPNIPPrefixRoute:
			s = append(s, fmt.Sprintf("[ESI: %s]", route.ESI.String()))
			if route.GWIPAddress != nil {
				s = append(s, fmt.Sprintf("[GW: %s]", route.GWIPAddress.String()))
			}
		}
	}
	return fmt.Sprint(s)
}

func makeShowRouteArgs(p *table.Path, idx int, now time.Time, showAge, showBest, showLabel bool, showIdentifier bgp.BGPAddPathMode) []interface{} {
	nlri := p.GetNlri()

	// Path Symbols (e.g. "*>")
	args := []interface{}{getPathSymbolString(p, idx, showBest)}

	// Path Identifier
	switch showIdentifier {
	case bgp.BGP_ADD_PATH_RECEIVE:
		args = append(args, fmt.Sprint(nlri.PathIdentifier()))
	case bgp.BGP_ADD_PATH_SEND:
		args = append(args, fmt.Sprint(nlri.PathLocalIdentifier()))
	}

	// NLRI
	args = append(args, nlri)

	// Label
	label := ""
	if showLabel {
		label = p.GetLabelString()
		args = append(args, label)
	}

	// Next Hop
	nexthop := "fictitious"
	if n := p.GetNexthop(); n != nil {
		nexthop = p.GetNexthop().String()
	}
	args = append(args, nexthop)

	// AS_PATH
	aspathstr := p.GetAsString()
	args = append(args, aspathstr)

	// Age
	if showAge {
		args = append(args, formatTimedelta(int64(now.Sub(p.GetTimestamp()).Seconds())))
	}

	// Path Attributes
	pattrstr := getPathAttributeString(p)
	args = append(args, pattrstr)

	updateColumnWidth(nlri.String(), nexthop, aspathstr, label)

	return args
}

func showRoute(destinationList [][]*table.Path, showAge, showBest, showLabel bool, showIdentifier bgp.BGPAddPathMode) {
	var pathStrs [][]interface{}
	now := time.Now()
	for _, pathList := range destinationList {
		for idx, p := range pathList {
			pathStrs = append(pathStrs, makeShowRouteArgs(p, idx, now, showAge, showBest, showLabel, showIdentifier))
		}
	}

	headers := make([]interface{}, 0)
	var format string
	headers = append(headers, "") // Symbols
	format = fmt.Sprintf("%%-3s")
	if showIdentifier != bgp.BGP_ADD_PATH_NONE {
		headers = append(headers, "ID")
		format += "%-3s "
	}
	headers = append(headers, "Network")
	format += fmt.Sprintf("%%-%ds ", columnWidthPrefix)
	if showLabel {
		headers = append(headers, "Labels")
		format += fmt.Sprintf("%%-%ds ", columnWidthLabel)
	}
	headers = append(headers, "Next Hop", "AS_PATH")
	format += fmt.Sprintf("%%-%ds %%-%ds ", columnWidthNextHop, columnWidthAsPath)
	if showAge {
		headers = append(headers, "Age")
		format += "%-10s "
	}
	headers = append(headers, "Attrs")
	format += "%-s\n"

	fmt.Printf(format, headers...)
	for _, pathStr := range pathStrs {
		fmt.Printf(format, pathStr...)
	}
}

func checkOriginAsWasNotShown(p *table.Path, shownAs map[uint32]struct{}) bool {
	asPath := p.GetAsPath().Value
	// the path was generated in internal
	if len(asPath) == 0 {
		return false
	}
	asList := asPath[len(asPath)-1].GetAS()
	origin := asList[len(asList)-1]

	if _, ok := shownAs[origin]; ok {
		return false
	}
	shownAs[origin] = struct{}{}
	return true
}

func showValidationInfo(p *table.Path, shownAs map[uint32]struct{}) error {
	asPath := p.GetAsPath().Value
	if len(asPath) == 0 {
		return fmt.Errorf("The path to %s was locally generated.\n", p.GetNlri().String())
	} else if !checkOriginAsWasNotShown(p, shownAs) {
		return nil
	}

	status := p.Validation().Status
	reason := p.Validation().Reason
	asList := asPath[len(asPath)-1].GetAS()
	origin := asList[len(asList)-1]

	fmt.Printf("Target Prefix: %s, AS: %d\n", p.GetNlri().String(), origin)
	fmt.Printf("  This route is %s", status)
	switch status {
	case config.RPKI_VALIDATION_RESULT_TYPE_INVALID:
		fmt.Printf("  reason: %s\n", reason)
		switch reason {
		case table.RPKI_VALIDATION_REASON_TYPE_AS:
			fmt.Println("  No VRP ASN matches the route origin ASN.")
		case table.RPKI_VALIDATION_REASON_TYPE_LENGTH:
			fmt.Println("  Route Prefix length is greater than the maximum length allowed by VRP(s) matching this route origin ASN.")
		}
	case config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND:
		fmt.Println("\n  No VRP Covers the Route Prefix")
	default:
		fmt.Print("\n\n")
	}

	printVRPs := func(l []*table.ROA) {
		if len(l) == 0 {
			fmt.Println("    No Entry")
		} else {
			var format string
			if ip, _, _ := net.ParseCIDR(p.GetNlri().String()); ip.To4() != nil {
				format = "    %-18s %-6s %-10s\n"
			} else {
				format = "    %-42s %-6s %-10s\n"
			}
			fmt.Printf(format, "Network", "AS", "MaxLen")
			for _, m := range l {
				fmt.Printf(format, m.Prefix, fmt.Sprint(m.AS), fmt.Sprint(m.MaxLen))
			}
		}
	}

	fmt.Println("  Matched VRPs: ")
	printVRPs(p.Validation().Matched)
	fmt.Println("  Unmatched AS VRPs: ")
	printVRPs(p.Validation().UnmatchedAs)
	fmt.Println("  Unmatched Length VRPs: ")
	printVRPs(p.Validation().UnmatchedLength)

	return nil
}

func showRibInfo(r, name string) error {
	def := addr2AddressFamily(net.ParseIP(name))
	if r == CMD_GLOBAL {
		def = bgp.RF_IPv4_UC
	}
	family, err := checkAddressFamily(def)
	if err != nil {
		return err
	}

	var info *table.TableInfo
	switch r {
	case CMD_GLOBAL:
		info, err = client.GetRIBInfo(family)
	case CMD_LOCAL:
		info, err = client.GetLocalRIBInfo(name, family)
	case CMD_ADJ_IN:
		info, err = client.GetAdjRIBInInfo(name, family)
	case CMD_ADJ_OUT:
		info, err = client.GetAdjRIBOutInfo(name, family)
	default:
		return fmt.Errorf("invalid resource to show RIB info: %s", r)
	}

	if err != nil {
		return err
	}

	if globalOpts.Json {
		j, _ := json.Marshal(info)
		fmt.Println(string(j))
		return nil
	}
	fmt.Printf("Table %s\n", family)
	fmt.Printf("Destination: %d, Path: %d\n", info.NumDestination, info.NumPath)
	return nil

}

func parseCIDRorIP(str string) (net.IP, *net.IPNet, error) {
	ip, n, err := net.ParseCIDR(str)
	if err == nil {
		return ip, n, nil
	}
	ip = net.ParseIP(str)
	if ip == nil {
		return ip, nil, fmt.Errorf("invalid CIDR/IP")
	}
	return ip, nil, nil
}

func showNeighborRib(r string, name string, args []string) error {
	showBest := false
	showAge := true
	showLabel := false
	showIdentifier := bgp.BGP_ADD_PATH_NONE
	validationTarget := ""

	def := addr2AddressFamily(net.ParseIP(name))
	switch r {
	case CMD_GLOBAL:
		def = bgp.RF_IPv4_UC
		showBest = true
	case CMD_LOCAL:
		showBest = true
	case CMD_ADJ_OUT:
		showAge = false
	case CMD_VRF:
		def = bgp.RF_IPv4_UC
		showBest = true
	}
	family, err := checkAddressFamily(def)
	if err != nil {
		return err
	}
	switch family {
	case bgp.RF_IPv4_MPLS, bgp.RF_IPv6_MPLS, bgp.RF_IPv4_VPN, bgp.RF_IPv6_VPN, bgp.RF_EVPN:
		showLabel = true
	}

	var filter []*table.LookupPrefix
	if len(args) > 0 {
		target := args[0]
		switch family {
		case bgp.RF_EVPN:
			// Uses target as EVPN Route Type string
		default:
			if _, _, err = parseCIDRorIP(target); err != nil {
				return err
			}
		}
		var option table.LookupOption
		args = args[1:]
		for len(args) != 0 {
			if args[0] == "longer-prefixes" {
				option = table.LOOKUP_LONGER
			} else if args[0] == "shorter-prefixes" {
				option = table.LOOKUP_SHORTER
			} else if args[0] == "validation" {
				if r != CMD_ADJ_IN {
					return fmt.Errorf("RPKI information is supported for only adj-in.")
				}
				validationTarget = target
			} else {
				return fmt.Errorf("invalid format for route filtering")
			}
			args = args[1:]
		}
		filter = []*table.LookupPrefix{&table.LookupPrefix{
			Prefix:       target,
			LookupOption: option,
		},
		}
	}

	var rib *table.Table
	switch r {
	case CMD_GLOBAL:
		rib, err = client.GetRIB(family, filter)
	case CMD_LOCAL:
		rib, err = client.GetLocalRIB(name, family, filter)
	case CMD_ADJ_IN, CMD_ACCEPTED, CMD_REJECTED:
		showIdentifier = bgp.BGP_ADD_PATH_RECEIVE
		rib, err = client.GetAdjRIBIn(name, family, filter)
	case CMD_ADJ_OUT:
		showIdentifier = bgp.BGP_ADD_PATH_SEND
		rib, err = client.GetAdjRIBOut(name, family, filter)
	case CMD_VRF:
		rib, err = client.GetVRFRIB(name, family, filter)
	}

	if err != nil {
		return err
	}

	switch r {
	case CMD_LOCAL, CMD_ADJ_IN, CMD_ACCEPTED, CMD_REJECTED, CMD_ADJ_OUT:
		if rib.Info("", 0).NumDestination == 0 {
			peer, err := client.GetNeighbor(name, false)
			if err != nil {
				return err
			}
			if peer.State.SessionState != config.SESSION_STATE_ESTABLISHED {
				return fmt.Errorf("Neighbor %v's BGP session is not established", name)
			}
		}
	}

	if globalOpts.Json {
		d := make(map[string]*table.Destination)
		for _, dst := range rib.GetDestinations() {
			d[dst.GetNlri().String()] = dst
		}
		j, _ := json.Marshal(d)
		fmt.Println(string(j))
		return nil
	}

	if validationTarget != "" {
		// show RPKI validation info
		addr, _, err := net.ParseCIDR(validationTarget)
		if err != nil {
			return err
		}
		var nlri bgp.AddrPrefixInterface
		if addr.To16() == nil {
			nlri, _ = bgp.NewPrefixFromRouteFamily(bgp.AFI_IP, bgp.SAFI_UNICAST, validationTarget)
		} else {
			nlri, _ = bgp.NewPrefixFromRouteFamily(bgp.AFI_IP6, bgp.SAFI_UNICAST, validationTarget)
		}
		d := rib.GetDestination(nlri)
		if d == nil {
			fmt.Println("Network not in table")
			return nil
		}
		shownAs := make(map[uint32]struct{})
		for _, p := range d.GetAllKnownPathList() {
			if err := showValidationInfo(p, shownAs); err != nil {
				return err
			}
		}
	} else {
		// show RIB
		var ds [][]*table.Path
		for _, d := range rib.GetSortedDestinations() {
			var ps []*table.Path
			switch r {
			case CMD_ACCEPTED:
				ps = append(ps, d.GetAllKnownPathList()...)
			case CMD_REJECTED:
				// always nothing
			default:
				ps = d.GetAllKnownPathList()
			}
			ds = append(ds, ps)
		}
		if len(ds) > 0 {
			showRoute(ds, showAge, showBest, showLabel, showIdentifier)
		} else {
			fmt.Println("Network not in table")
		}
	}
	return nil
}

func resetNeighbor(cmd string, remoteIP string, args []string) error {
	family := bgp.RouteFamily(0)
	if reasonLen := len(neighborsOpts.Reason); reasonLen > bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX {
		return fmt.Errorf("Too long reason for shutdown communication (max %d bytes)", bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX)
	}
	switch cmd {
	case CMD_RESET:
		return client.ResetNeighbor(remoteIP, neighborsOpts.Reason)
	case CMD_SOFT_RESET:
		return client.SoftReset(remoteIP, family)
	case CMD_SOFT_RESET_IN:
		return client.SoftResetIn(remoteIP, family)
	case CMD_SOFT_RESET_OUT:
		return client.SoftResetOut(remoteIP, family)
	}
	return nil
}

func stateChangeNeighbor(cmd string, remoteIP string, args []string) error {
	if reasonLen := len(neighborsOpts.Reason); reasonLen > bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX {
		return fmt.Errorf("Too long reason for shutdown communication (max %d bytes)", bgp.BGP_ERROR_ADMINISTRATIVE_COMMUNICATION_MAX)
	}
	switch cmd {
	case CMD_SHUTDOWN:
		fmt.Printf("WARNING: command `%s` is deprecated. use `%s` instead\n", CMD_SHUTDOWN, CMD_DISABLE)
		return client.ShutdownNeighbor(remoteIP, neighborsOpts.Reason)
	case CMD_ENABLE:
		return client.EnableNeighbor(remoteIP)
	case CMD_DISABLE:
		return client.DisableNeighbor(remoteIP, neighborsOpts.Reason)
	}
	return nil
}

func showNeighborPolicy(remoteIP, policyType string, indent int) error {
	var assignment *table.PolicyAssignment
	var err error

	switch strings.ToLower(policyType) {
	case "import":
		assignment, err = client.GetRouteServerImportPolicy(remoteIP)
	case "export":
		assignment, err = client.GetRouteServerExportPolicy(remoteIP)
	default:
		return fmt.Errorf("invalid policy type: choose from (in|import|export)")
	}

	if err != nil {
		return err
	}

	if globalOpts.Json {
		j, _ := json.Marshal(assignment)
		fmt.Println(string(j))
		return nil
	}

	fmt.Printf("%s policy:\n", strings.Title(policyType))
	fmt.Printf("%sDefault: %s\n", strings.Repeat(" ", indent), assignment.Default.String())
	for _, p := range assignment.Policies {
		fmt.Printf("%sName %s:\n", strings.Repeat(" ", indent), p.Name)
		printPolicy(indent+4, p)
	}
	return nil
}

func extractDefaultAction(args []string) ([]string, table.RouteType, error) {
	for idx, arg := range args {
		if arg == "default" {
			if len(args) < (idx + 2) {
				return nil, table.ROUTE_TYPE_NONE, fmt.Errorf("specify default action [accept|reject]")
			}
			typ := args[idx+1]
			switch strings.ToLower(typ) {
			case "accept":
				return append(args[:idx], args[idx+2:]...), table.ROUTE_TYPE_ACCEPT, nil
			case "reject":
				return append(args[:idx], args[idx+2:]...), table.ROUTE_TYPE_REJECT, nil
			default:
				return nil, table.ROUTE_TYPE_NONE, fmt.Errorf("invalid default action")
			}
		}
	}
	return args, table.ROUTE_TYPE_NONE, nil
}

func modNeighborPolicy(remoteIP, policyType, cmdType string, args []string) error {
	assign := &table.PolicyAssignment{
		Name: remoteIP,
	}
	switch strings.ToLower(policyType) {
	case "import":
		assign.Type = table.POLICY_DIRECTION_IMPORT
	case "export":
		assign.Type = table.POLICY_DIRECTION_EXPORT
	}

	usage := fmt.Sprintf("usage: gobgp neighbor %s policy %s %s", remoteIP, policyType, cmdType)
	if remoteIP == "" {
		usage = fmt.Sprintf("usage: gobgp global policy %s %s", policyType, cmdType)
	}

	var err error
	switch cmdType {
	case CMD_ADD, CMD_SET:
		if len(args) < 1 {
			return fmt.Errorf("%s <policy name>... [default {%s|%s}]", usage, "accept", "reject")
		}
		var err error
		var def table.RouteType
		args, def, err = extractDefaultAction(args)
		if err != nil {
			return fmt.Errorf("%s\n%s <policy name>... [default {%s|%s}]", err, usage, "accept", "reject")
		}
		assign.Default = def
	}
	ps := make([]*table.Policy, 0, len(args))
	for _, name := range args {
		ps = append(ps, &table.Policy{Name: name})
	}
	assign.Policies = ps
	switch cmdType {
	case CMD_ADD:
		err = client.AddPolicyAssignment(assign)
	case CMD_SET:
		err = client.ReplacePolicyAssignment(assign)
	case CMD_DEL:
		all := false
		if len(args) == 0 {
			all = true
		}
		err = client.DeletePolicyAssignment(assign, all)
	}
	return err
}

func modNeighbor(cmdType string, args []string) error {
	params := map[string]int{
		"interface": PARAM_SINGLE,
	}
	usage := fmt.Sprintf("usage: gobgp neighbor %s [ <neighbor-address> | interface <neighbor-interface> ]", cmdType)
	if cmdType == CMD_ADD {
		usage += " as <VALUE>"
	} else if cmdType == CMD_UPDATE {
		usage += " [ as <VALUE> ]"
	}
	if cmdType == CMD_ADD || cmdType == CMD_UPDATE {
		params["as"] = PARAM_SINGLE
		params["family"] = PARAM_SINGLE
		params["vrf"] = PARAM_SINGLE
		params["route-reflector-client"] = PARAM_SINGLE
		params["route-server-client"] = PARAM_FLAG
		params["allow-own-as"] = PARAM_SINGLE
		params["remove-private-as"] = PARAM_SINGLE
		params["replace-peer-as"] = PARAM_FLAG
		usage += " [ family <address-families-list> | vrf <vrf-name> | route-reflector-client [<cluster-id>] | route-server-client | allow-own-as <num> | remove-private-as (all|replace) | replace-peer-as ]"
	}

	m, err := extractReserved(args, params)
	if err != nil || (len(m[""]) != 1 && len(m["interface"]) != 1) {
		return fmt.Errorf("%s", usage)
	}

	unnumbered := len(m["interface"]) > 0
	if !unnumbered {
		if _, err := net.ResolveIPAddr("ip", m[""][0]); err != nil {
			return err
		}
	}

	getNeighborAddress := func() (string, error) {
		if unnumbered {
			return config.GetIPv6LinkLocalNeighborAddress(m["interface"][0])
		}
		return m[""][0], nil
	}

	getNeighborConfig := func() (*config.Neighbor, error) {
		addr, err := getNeighborAddress()
		if err != nil {
			return nil, err
		}
		var peer *config.Neighbor
		switch cmdType {
		case CMD_ADD, CMD_DEL:
			peer = &config.Neighbor{}
			if unnumbered {
				peer.Config.NeighborInterface = m["interface"][0]
			} else {
				peer.Config.NeighborAddress = addr
			}
			peer.State.NeighborAddress = addr
		case CMD_UPDATE:
			peer, err = client.GetNeighbor(addr)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid command: %s", cmdType)
		}
		return peer, nil
	}

	updateNeighborConfig := func(peer *config.Neighbor) error {
		if len(m["as"]) > 0 {
			as, err := strconv.ParseUint(m["as"][0], 10, 32)
			if err != nil {
				return err
			}
			peer.Config.PeerAs = uint32(as)
		}
		if len(m["family"]) == 1 {
			peer.AfiSafis = make([]config.AfiSafi, 0) // for the case of CMD_UPDATE
			for _, family := range strings.Split(m["family"][0], ",") {
				afiSafiName := config.AfiSafiType(family)
				if afiSafiName.ToInt() == -1 {
					return fmt.Errorf("invalid family value: %s", family)
				}
				peer.AfiSafis = append(peer.AfiSafis, config.AfiSafi{Config: config.AfiSafiConfig{AfiSafiName: afiSafiName}})
			}
		}
		if len(m["vrf"]) == 1 {
			peer.Config.Vrf = m["vrf"][0]
		}
		if option, ok := m["route-reflector-client"]; ok {
			peer.RouteReflector.Config = config.RouteReflectorConfig{
				RouteReflectorClient: true,
			}
			if len(option) == 1 {
				peer.RouteReflector.Config.RouteReflectorClusterId = config.RrClusterIdType(option[0])
			}
		}
		if _, ok := m["route-server-client"]; ok {
			peer.RouteServer.Config = config.RouteServerConfig{
				RouteServerClient: true,
			}
		}
		if option, ok := m["allow-own-as"]; ok {
			as, err := strconv.ParseUint(option[0], 10, 8)
			if err != nil {
				return err
			}
			peer.AsPathOptions.Config.AllowOwnAs = uint8(as)
		}
		if option, ok := m["remove-private-as"]; ok {
			switch option[0] {
			case "all":
				peer.Config.RemovePrivateAs = config.REMOVE_PRIVATE_AS_OPTION_ALL
			case "replace":
				peer.Config.RemovePrivateAs = config.REMOVE_PRIVATE_AS_OPTION_REPLACE
			default:
				return fmt.Errorf("invalid remove-private-as value: all or replace")
			}
		}
		if _, ok := m["replace-peer-as"]; ok {
			peer.AsPathOptions.Config.ReplacePeerAs = true
		}
		return nil
	}

	n, err := getNeighborConfig()
	if err != nil {
		return err
	}

	switch cmdType {
	case CMD_ADD:
		if err := updateNeighborConfig(n); err != nil {
			return err
		}
		return client.AddNeighbor(n)
	case CMD_DEL:
		return client.DeleteNeighbor(n)
	case CMD_UPDATE:
		if err := updateNeighborConfig(n); err != nil {
			return err
		}
		_, err := client.UpdateNeighbor(n, true)
		return err
	}
	return nil
}

func NewNeighborCmd() *cobra.Command {

	neighborCmdImpl := &cobra.Command{}

	type cmds struct {
		names []string
		f     func(string, string, []string) error
	}

	c := make([]cmds, 0, 3)
	c = append(c, cmds{[]string{CMD_LOCAL, CMD_ADJ_IN, CMD_ADJ_OUT, CMD_ACCEPTED, CMD_REJECTED}, showNeighborRib})
	c = append(c, cmds{[]string{CMD_RESET, CMD_SOFT_RESET, CMD_SOFT_RESET_IN, CMD_SOFT_RESET_OUT}, resetNeighbor})
	c = append(c, cmds{[]string{CMD_SHUTDOWN, CMD_ENABLE, CMD_DISABLE}, stateChangeNeighbor})

	for _, v := range c {
		f := v.f
		for _, name := range v.names {
			c := &cobra.Command{
				Use: name,
				Run: func(cmd *cobra.Command, args []string) {
					addr := ""
					switch name {
					case CMD_RESET, CMD_SOFT_RESET, CMD_SOFT_RESET_IN, CMD_SOFT_RESET_OUT, CMD_SHUTDOWN:
						if args[len(args)-1] == "all" {
							addr = "all"
						}
					}
					if addr == "" {
						peer, err := client.GetNeighbor(args[len(args)-1], false)
						if err != nil {
							exitWithError(err)
						}
						addr = peer.State.NeighborAddress
					}
					err := f(cmd.Use, addr, args[:len(args)-1])
					if err != nil {
						exitWithError(err)
					}
				},
			}
			neighborCmdImpl.AddCommand(c)
			switch name {
			case CMD_LOCAL, CMD_ADJ_IN, CMD_ADJ_OUT:
				n := name
				c.AddCommand(&cobra.Command{
					Use: CMD_SUMMARY,
					Run: func(cmd *cobra.Command, args []string) {
						if err := showRibInfo(n, args[len(args)-1]); err != nil {
							exitWithError(err)
						}
					},
				})
			}
		}
	}

	policyCmd := &cobra.Command{
		Use: CMD_POLICY,
		Run: func(cmd *cobra.Command, args []string) {
			peer, err := client.GetNeighbor(args[0], false)
			if err != nil {
				exitWithError(err)
			}
			remoteIP := peer.State.NeighborAddress
			for _, v := range []string{CMD_IN, CMD_IMPORT, CMD_EXPORT} {
				if err := showNeighborPolicy(remoteIP, v, 4); err != nil {
					exitWithError(err)
				}
			}
		},
	}

	for _, v := range []string{CMD_IN, CMD_IMPORT, CMD_EXPORT} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(cmd *cobra.Command, args []string) {
				peer, err := client.GetNeighbor(args[0], false)
				if err != nil {
					exitWithError(err)
				}
				remoteIP := peer.State.NeighborAddress
				err = showNeighborPolicy(remoteIP, cmd.Use, 0)
				if err != nil {
					exitWithError(err)
				}
			},
		}

		for _, w := range []string{CMD_ADD, CMD_DEL, CMD_SET} {
			subcmd := &cobra.Command{
				Use: w,
				Run: func(subcmd *cobra.Command, args []string) {
					peer, err := client.GetNeighbor(args[len(args)-1], false)
					if err != nil {
						exitWithError(err)
					}
					remoteIP := peer.State.NeighborAddress
					args = args[:len(args)-1]
					if err = modNeighborPolicy(remoteIP, cmd.Use, subcmd.Use, args); err != nil {
						exitWithError(err)
					}
				},
			}
			cmd.AddCommand(subcmd)
		}

		policyCmd.AddCommand(cmd)

	}

	neighborCmdImpl.AddCommand(policyCmd)

	neighborCmd := &cobra.Command{
		Use: CMD_NEIGHBOR,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if len(args) == 0 {
				err = showNeighbors("")
			} else if len(args) == 1 {
				err = showNeighbor(args)
			} else {
				args = append(args[1:], args[0])
				neighborCmdImpl.SetArgs(args)
				err = neighborCmdImpl.Execute()
			}
			if err != nil {
				exitWithError(err)
			}
		},
	}

	for _, v := range []string{CMD_ADD, CMD_DEL, CMD_UPDATE} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(c *cobra.Command, args []string) {
				if err := modNeighbor(c.Use, args); err != nil {
					exitWithError(err)
				}
			},
		}
		neighborCmd.AddCommand(cmd)
	}

	neighborCmd.PersistentFlags().StringVarP(&subOpts.AddressFamily, "address-family", "a", "", "address family")
	neighborCmd.PersistentFlags().StringVarP(&neighborsOpts.Reason, "reason", "", "", "specifying communication field on Cease NOTIFICATION message with Administrative Shutdown subcode")
	neighborCmd.PersistentFlags().StringVarP(&neighborsOpts.Transport, "transport", "t", "", "specifying a transport protocol")
	return neighborCmd
}
