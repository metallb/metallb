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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/table"
	"github.com/spf13/cobra"
)

func formatDefinedSet(head bool, typ string, indent int, list []table.DefinedSet) string {
	if len(list) == 0 {
		return "Nothing defined yet\n"
	}
	buff := bytes.NewBuffer(make([]byte, 0, 64))
	sIndent := strings.Repeat(" ", indent)
	maxNameLen := 0
	for _, s := range list {
		if len(s.Name()) > maxNameLen {
			maxNameLen = len(s.Name())
		}
	}
	if head {
		if len("NAME") > maxNameLen {
			maxNameLen = len("NAME")
		}
	}
	format := fmt.Sprintf("%%-%ds  %%s\n", maxNameLen)
	if head {
		buff.WriteString(fmt.Sprintf(format, "NAME", typ))
	}
	for _, s := range list {
		l := s.List()
		if len(l) == 0 {
			buff.WriteString(fmt.Sprintf(format, s.Name(), ""))
		}
		for i, x := range l {
			if typ == "COMMUNITY" || typ == "EXT-COMMUNITY" || typ == "LARGE-COMMUNITY" {
				exp := regexp.MustCompile("\\^\\^(\\S+)\\$\\$")
				x = exp.ReplaceAllString(x, "$1")
			}
			if i == 0 {
				buff.WriteString(fmt.Sprintf(format, s.Name(), x))
			} else {
				buff.WriteString(fmt.Sprintf(sIndent))
				buff.WriteString(fmt.Sprintf(format, "", x))
			}
		}
	}
	return buff.String()
}

func showDefinedSet(v string, args []string) error {
	var typ table.DefinedType
	switch v {
	case CMD_PREFIX:
		typ = table.DEFINED_TYPE_PREFIX
	case CMD_NEIGHBOR:
		typ = table.DEFINED_TYPE_NEIGHBOR
	case CMD_ASPATH:
		typ = table.DEFINED_TYPE_AS_PATH
	case CMD_COMMUNITY:
		typ = table.DEFINED_TYPE_COMMUNITY
	case CMD_EXTCOMMUNITY:
		typ = table.DEFINED_TYPE_EXT_COMMUNITY
	case CMD_LARGECOMMUNITY:
		typ = table.DEFINED_TYPE_LARGE_COMMUNITY
	default:
		return fmt.Errorf("unknown defined type: %s", v)
	}
	var m table.DefinedSetList
	if len(args) > 0 {
		d, err := client.GetDefinedSetByName(typ, args[0])
		if err != nil {
			return err
		}
		m = table.DefinedSetList([]table.DefinedSet{d})
	} else {
		var err error
		m, err = client.GetDefinedSet(typ)
		if err != nil {
			return err
		}
	}
	if globalOpts.Json {
		j, _ := json.Marshal(m)
		fmt.Println(string(j))
		return nil
	}
	if globalOpts.Quiet {
		if len(args) > 0 {
			fmt.Println(m)
		} else {
			for _, p := range m {
				fmt.Println(p.Name())
			}
		}
		return nil
	}
	sort.Sort(m)
	var output string
	switch v {
	case CMD_PREFIX:
		output = formatDefinedSet(true, "PREFIX", 0, m)
	case CMD_NEIGHBOR:
		output = formatDefinedSet(true, "ADDRESS", 0, m)
	case CMD_ASPATH:
		output = formatDefinedSet(true, "AS-PATH", 0, m)
	case CMD_COMMUNITY:
		output = formatDefinedSet(true, "COMMUNITY", 0, m)
	case CMD_EXTCOMMUNITY:
		output = formatDefinedSet(true, "EXT-COMMUNITY", 0, m)
	case CMD_LARGECOMMUNITY:
		output = formatDefinedSet(true, "LARGE-COMMUNITY", 0, m)
	}
	fmt.Print(output)
	return nil
}

func parsePrefixSet(args []string) (table.DefinedSet, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("empty neighbor set name")
	}
	name := args[0]
	args = args[1:]
	var list []config.Prefix
	if len(args) > 0 {
		mask := ""
		if len(args) > 1 {
			mask = args[1]
		}
		min, max, err := config.ParseMaskLength(args[0], mask)
		if err != nil {
			return nil, err
		}
		prefix := config.Prefix{
			IpPrefix:        args[0],
			MasklengthRange: fmt.Sprintf("%d..%d", min, max),
		}
		list = []config.Prefix{prefix}
	}
	return table.NewPrefixSet(config.PrefixSet{
		PrefixSetName: name,
		PrefixList:    list,
	})
}

func parseNeighborSet(args []string) (table.DefinedSet, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("empty neighbor set name")
	}
	name := args[0]
	args = args[1:]
	for _, arg := range args {
		address := net.ParseIP(arg)
		if address.To4() == nil && address.To16() == nil {
			_, _, err := net.ParseCIDR(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid address or prefix: %s\nplease enter ipv4 or ipv6 format", arg)
			}
		}
	}
	return table.NewNeighborSet(config.NeighborSet{
		NeighborSetName:  name,
		NeighborInfoList: args,
	})
}

func parseAsPathSet(args []string) (table.DefinedSet, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("empty as-path set name")
	}
	name := args[0]
	args = args[1:]
	for _, arg := range args {
		_, err := regexp.Compile(arg)
		if err != nil {
			return nil, err
		}
	}
	return table.NewAsPathSet(config.AsPathSet{
		AsPathSetName: name,
		AsPathList:    args,
	})
}

func parseCommunitySet(args []string) (table.DefinedSet, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("empty community set name")
	}
	name := args[0]
	args = args[1:]
	for _, arg := range args {
		if _, err := table.ParseCommunityRegexp(arg); err != nil {
			return nil, err
		}
	}
	return table.NewCommunitySet(config.CommunitySet{
		CommunitySetName: name,
		CommunityList:    args,
	})
}

func parseExtCommunitySet(args []string) (table.DefinedSet, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("empty ext-community set name")
	}
	name := args[0]
	args = args[1:]
	for _, arg := range args {
		if _, _, err := table.ParseExtCommunityRegexp(arg); err != nil {
			return nil, err
		}
	}
	return table.NewExtCommunitySet(config.ExtCommunitySet{
		ExtCommunitySetName: name,
		ExtCommunityList:    args,
	})
}

func parseLargeCommunitySet(args []string) (table.DefinedSet, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("empty large-community set name")
	}
	name := args[0]
	args = args[1:]
	for _, arg := range args {
		if _, err := table.ParseLargeCommunityRegexp(arg); err != nil {
			return nil, err
		}
	}
	return table.NewLargeCommunitySet(config.LargeCommunitySet{
		LargeCommunitySetName: name,
		LargeCommunityList:    args,
	})
}

func parseDefinedSet(settype string, args []string) (table.DefinedSet, error) {
	switch settype {
	case CMD_PREFIX:
		return parsePrefixSet(args)
	case CMD_NEIGHBOR:
		return parseNeighborSet(args)
	case CMD_ASPATH:
		return parseAsPathSet(args)
	case CMD_COMMUNITY:
		return parseCommunitySet(args)
	case CMD_EXTCOMMUNITY:
		return parseExtCommunitySet(args)
	case CMD_LARGECOMMUNITY:
		return parseLargeCommunitySet(args)
	default:
		return nil, fmt.Errorf("invalid defined set type: %s", settype)
	}
}

var modPolicyUsageFormat = map[string]string{
	CMD_PREFIX:         "usage: policy prefix %s <name> [<prefix> [<mask range>]]",
	CMD_NEIGHBOR:       "usage: policy neighbor %s <name> [<neighbor address>...]",
	CMD_ASPATH:         "usage: policy aspath %s <name> [<regexp>...]",
	CMD_COMMUNITY:      "usage: policy community %s <name> [<regexp>...]",
	CMD_EXTCOMMUNITY:   "usage: policy extcommunity %s <name> [<regexp>...]",
	CMD_LARGECOMMUNITY: "usage: policy large-community %s <name> [<regexp>...]",
}

func modDefinedSet(settype string, modtype string, args []string) error {
	var d table.DefinedSet
	var err error
	if len(args) < 1 {
		return fmt.Errorf(modPolicyUsageFormat[settype], modtype)
	}
	if d, err = parseDefinedSet(settype, args); err != nil {
		return err
	}
	switch modtype {
	case CMD_ADD:
		err = client.AddDefinedSet(d)
	case CMD_DEL:
		all := false
		if len(args) < 2 {
			all = true
		}
		err = client.DeleteDefinedSet(d, all)
	case CMD_SET:
		err = client.ReplaceDefinedSet(d)
	}
	return err
}

func printStatement(indent int, s *table.Statement) {
	sIndent := func(indent int) string {
		return strings.Repeat(" ", indent)
	}
	fmt.Printf("%sStatementName %s:\n", sIndent(indent), s.Name)
	fmt.Printf("%sConditions:\n", sIndent(indent+2))

	ind := sIndent(indent + 4)

	for _, c := range s.Conditions {
		switch t := c.(type) {
		case *table.PrefixCondition:
			fmt.Printf("%sPrefixSet: %s %s\n", ind, t.Option(), t.Name())
		case *table.NeighborCondition:
			fmt.Printf("%sNeighborSet: %s %s\n", ind, t.Option(), t.Name())
		case *table.AsPathCondition:
			fmt.Printf("%sAsPathSet: %s %s\n", ind, t.Option(), t.Name())
		case *table.CommunityCondition:
			fmt.Printf("%sCommunitySet: %s %s\n", ind, t.Option(), t.Name())
		case *table.ExtCommunityCondition:
			fmt.Printf("%sExtCommunitySet: %s %s\n", ind, t.Option(), t.Name())
		case *table.LargeCommunityCondition:
			fmt.Printf("%sLargeCommunitySet: %s %s\n", ind, t.Option(), t.Name())
		case *table.AsPathLengthCondition:
			fmt.Printf("%sAsPathLength: %s\n", ind, t.String())
		case *table.RpkiValidationCondition:
			fmt.Printf("%sRPKI result: %s\n", ind, t.String())
		case *table.RouteTypeCondition:
			fmt.Printf("%sRoute Type: %s\n", ind, t.String())
		}
	}

	fmt.Printf("%sActions:\n", sIndent(indent+2))
	for _, a := range s.ModActions {
		switch t := a.(type) {
		case *table.RoutingAction:
			action := "accept"
			if !t.AcceptRoute {
				action = "reject"
			}
			fmt.Println(ind, action)
		case *table.CommunityAction:
			fmt.Println(ind, "Community: ", t.String())
		case *table.ExtCommunityAction:
			fmt.Println(ind, "ExtCommunity: ", t.String())
		case *table.LargeCommunityAction:
			fmt.Println(ind, "LargeCommunity: ", t.String())
		case *table.MedAction:
			fmt.Println(ind, "MED: ", t.String())
		case *table.LocalPrefAction:
			fmt.Println(ind, "LocalPref: ", t.String())
		case *table.AsPathPrependAction:
			fmt.Println(ind, "ASPathPrepend: ", t.String())
		case *table.NexthopAction:
			fmt.Println(ind, "Nexthop: ", t.String())
		}
	}

	if s.RouteAction != nil && s.RouteAction.(*table.RoutingAction) != nil {
		t := s.RouteAction.(*table.RoutingAction)
		fmt.Println(ind, t.String())
	}

}

func printPolicy(indent int, pd *table.Policy) {
	for _, s := range pd.Statements {
		printStatement(indent, s)
	}
}

func showPolicy(args []string) error {
	policies, err := client.GetPolicy()
	if err != nil {
		return err
	}
	var m table.Policies
	if len(args) > 0 {
		for _, p := range policies {
			if args[0] == p.Name {
				m = append(m, p)
				break
			}
		}
		if len(m) == 0 {
			return fmt.Errorf("not found %s", args[0])
		}
	} else {
		m = policies
	}
	if globalOpts.Json {
		j, _ := json.Marshal(m)
		fmt.Println(string(j))
		return nil
	}
	if globalOpts.Quiet {
		for _, p := range m {
			fmt.Println(p.Name)
		}
		return nil
	}
	sort.Sort(m)
	for _, pd := range m {
		fmt.Printf("Name %s:\n", pd.Name)
		printPolicy(4, pd)
	}
	return nil
}

func showStatement(args []string) error {
	stmts, err := client.GetStatement()
	if err != nil {
		return err
	}
	var m []*table.Statement
	if len(args) > 0 {
		for _, s := range stmts {
			if args[0] == s.Name {
				m = append(m, s)
				break
			}
		}
		if len(m) == 0 {
			return fmt.Errorf("not found %s", args[0])
		}
	} else {
		m = stmts
	}
	if globalOpts.Json {
		j, _ := json.Marshal(m)
		fmt.Println(string(j))
		return nil
	}
	if globalOpts.Quiet {
		for _, s := range m {
			fmt.Println(s.Name)
		}
		return nil
	}
	for _, s := range m {
		printStatement(0, s)
	}
	return nil
}

func modStatement(op string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gobgp policy statement %s <name>", op)
	}
	stmt := &table.Statement{
		Name: args[0],
	}
	var err error
	switch op {
	case CMD_ADD:
		err = client.AddStatement(stmt)
	case CMD_DEL:
		err = client.DeleteStatement(stmt, false)
	default:
		return fmt.Errorf("invalid operation: %s", op)
	}
	return err
}

func modCondition(name, op string, args []string) error {
	stmt := config.Statement{
		Name: name,
	}
	usage := fmt.Sprintf("usage: gobgp policy statement %s %s condition", name, op)
	if len(args) < 1 {
		return fmt.Errorf("%s { prefix | neighbor | as-path | community | ext-community | large-community | as-path-length | rpki | route-type }", usage)
	}
	typ := args[0]
	args = args[1:]
	switch typ {
	case "prefix":
		if len(args) < 1 {
			return fmt.Errorf("%s prefix <set-name> [{ any | invert }]", usage)
		}
		stmt.Conditions.MatchPrefixSet.PrefixSet = args[0]
		if len(args) == 1 {
			break
		}
		switch strings.ToLower(args[1]) {
		case "any":
			stmt.Conditions.MatchPrefixSet.MatchSetOptions = config.MATCH_SET_OPTIONS_RESTRICTED_TYPE_ANY
		case "invert":
			stmt.Conditions.MatchPrefixSet.MatchSetOptions = config.MATCH_SET_OPTIONS_RESTRICTED_TYPE_INVERT
		default:
			return fmt.Errorf("%s prefix <set-name> [{ any | invert }]", usage)
		}
	case "neighbor":
		if len(args) < 1 {
			return fmt.Errorf("%s neighbor <set-name> [{ any | invert }]", usage)
		}
		stmt.Conditions.MatchNeighborSet.NeighborSet = args[0]
		if len(args) == 1 {
			break
		}
		switch strings.ToLower(args[1]) {
		case "any":
			stmt.Conditions.MatchNeighborSet.MatchSetOptions = config.MATCH_SET_OPTIONS_RESTRICTED_TYPE_ANY
		case "invert":
			stmt.Conditions.MatchNeighborSet.MatchSetOptions = config.MATCH_SET_OPTIONS_RESTRICTED_TYPE_INVERT
		default:
			return fmt.Errorf("%s neighbor <set-name> [{ any | invert }]", usage)
		}
	case "as-path":
		if len(args) < 1 {
			return fmt.Errorf("%s as-path <set-name> [{ any | all | invert }]", usage)
		}
		stmt.Conditions.BgpConditions.MatchAsPathSet.AsPathSet = args[0]
		if len(args) == 1 {
			break
		}
		switch strings.ToLower(args[1]) {
		case "any":
			stmt.Conditions.BgpConditions.MatchAsPathSet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_ANY
		case "all":
			stmt.Conditions.BgpConditions.MatchAsPathSet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_ALL
		case "invert":
			stmt.Conditions.BgpConditions.MatchAsPathSet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_INVERT
		default:
			return fmt.Errorf("%s as-path <set-name> [{ any | all | invert }]", usage)
		}
	case "community":
		if len(args) < 1 {
			return fmt.Errorf("%s community <set-name> [{ any | all | invert }]", usage)
		}
		stmt.Conditions.BgpConditions.MatchCommunitySet.CommunitySet = args[0]
		if len(args) == 1 {
			break
		}
		switch strings.ToLower(args[1]) {
		case "any":
			stmt.Conditions.BgpConditions.MatchCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_ANY
		case "all":
			stmt.Conditions.BgpConditions.MatchCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_ALL
		case "invert":
			stmt.Conditions.BgpConditions.MatchCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_INVERT
		default:
			return fmt.Errorf("%s community <set-name> [{ any | all | invert }]", usage)
		}
	case "ext-community":
		if len(args) < 1 {
			return fmt.Errorf("%s ext-community <set-name> [{ any | all | invert }]", usage)
		}
		stmt.Conditions.BgpConditions.MatchExtCommunitySet.ExtCommunitySet = args[0]
		if len(args) == 1 {
			break
		}
		switch strings.ToLower(args[1]) {
		case "any":
			stmt.Conditions.BgpConditions.MatchExtCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_ANY
		case "all":
			stmt.Conditions.BgpConditions.MatchExtCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_ALL
		case "invert":
			stmt.Conditions.BgpConditions.MatchExtCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_INVERT
		default:
			return fmt.Errorf("%s ext-community <set-name> [{ any | all | invert }]", usage)
		}
	case "large-community":
		if len(args) < 1 {
			return fmt.Errorf("%s large-community <set-name> [{ any | all | invert }]", usage)
		}
		stmt.Conditions.BgpConditions.MatchLargeCommunitySet.LargeCommunitySet = args[0]
		if len(args) == 1 {
			break
		}
		switch strings.ToLower(args[1]) {
		case "any":
			stmt.Conditions.BgpConditions.MatchLargeCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_ANY
		case "all":
			stmt.Conditions.BgpConditions.MatchLargeCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_ALL
		case "invert":
			stmt.Conditions.BgpConditions.MatchLargeCommunitySet.MatchSetOptions = config.MATCH_SET_OPTIONS_TYPE_INVERT
		default:
			return fmt.Errorf("%s large-community <set-name> [{ any | all | invert }]", usage)
		}
	case "as-path-length":
		if len(args) < 2 {
			return fmt.Errorf("%s as-path-length <length> { eq | ge | le }", usage)
		}
		length, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		stmt.Conditions.BgpConditions.AsPathLength.Value = uint32(length)
		switch strings.ToLower(args[1]) {
		case "eq":
			stmt.Conditions.BgpConditions.AsPathLength.Operator = config.ATTRIBUTE_COMPARISON_EQ
		case "ge":
			stmt.Conditions.BgpConditions.AsPathLength.Operator = config.ATTRIBUTE_COMPARISON_GE
		case "le":
			stmt.Conditions.BgpConditions.AsPathLength.Operator = config.ATTRIBUTE_COMPARISON_LE
		default:
			return fmt.Errorf("%s as-path-length <length> { eq | ge | le }", usage)
		}
	case "rpki":
		if len(args) < 1 {
			return fmt.Errorf("%s rpki { valid | invalid | not-found }", usage)
		}
		switch strings.ToLower(args[0]) {
		case "valid":
			stmt.Conditions.BgpConditions.RpkiValidationResult = config.RPKI_VALIDATION_RESULT_TYPE_VALID
		case "invalid":
			stmt.Conditions.BgpConditions.RpkiValidationResult = config.RPKI_VALIDATION_RESULT_TYPE_INVALID
		case "not-found":
			stmt.Conditions.BgpConditions.RpkiValidationResult = config.RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND
		default:
			return fmt.Errorf("%s rpki { valid | invalid | not-found }", usage)
		}
	case "route-type":
		err := fmt.Errorf("%s route-type { internal | external | local }", usage)
		if len(args) < 1 {
			return err
		}
		switch strings.ToLower(args[0]) {
		case "internal":
			stmt.Conditions.BgpConditions.RouteType = config.ROUTE_TYPE_INTERNAL
		case "external":
			stmt.Conditions.BgpConditions.RouteType = config.ROUTE_TYPE_EXTERNAL
		case "local":
			stmt.Conditions.BgpConditions.RouteType = config.ROUTE_TYPE_LOCAL
		default:
			return err
		}
	default:
		return fmt.Errorf("%s { prefix | neighbor | as-path | community | ext-community | large-community | as-path-length | rpki | route-type }", usage)
	}

	t, err := table.NewStatement(stmt)
	if err != nil {
		return err
	}
	switch op {
	case CMD_ADD:
		err = client.AddStatement(t)
	case CMD_DEL:
		err = client.DeleteStatement(t, false)
	case CMD_SET:
		err = client.ReplaceStatement(t)
	default:
		return fmt.Errorf("invalid operation: %s", op)
	}
	return err
}

func modAction(name, op string, args []string) error {
	stmt := config.Statement{
		Name: name,
	}
	usage := fmt.Sprintf("usage: gobgp policy statement %s %s action", name, op)
	if len(args) < 1 {
		return fmt.Errorf("%s { reject | accept | community | ext-community | large-community | med | local-pref | as-prepend | next-hop }", usage)
	}
	typ := args[0]
	args = args[1:]
	switch typ {
	case "reject":
		stmt.Actions.RouteDisposition = config.ROUTE_DISPOSITION_REJECT_ROUTE
	case "accept":
		stmt.Actions.RouteDisposition = config.ROUTE_DISPOSITION_ACCEPT_ROUTE
	case "community":
		if len(args) < 1 {
			return fmt.Errorf("%s community { add | remove | replace } <value>...", usage)
		}
		stmt.Actions.BgpActions.SetCommunity.SetCommunityMethod.CommunitiesList = args[1:]
		switch strings.ToLower(args[0]) {
		case "add":
			stmt.Actions.BgpActions.SetCommunity.Options = string(config.BGP_SET_COMMUNITY_OPTION_TYPE_ADD)
		case "remove":
			stmt.Actions.BgpActions.SetCommunity.Options = string(config.BGP_SET_COMMUNITY_OPTION_TYPE_REMOVE)
		case "replace":
			stmt.Actions.BgpActions.SetCommunity.Options = string(config.BGP_SET_COMMUNITY_OPTION_TYPE_REPLACE)
		default:
			return fmt.Errorf("%s community { add | remove | replace } <value>...", usage)
		}
	case "ext-community":
		if len(args) < 1 {
			return fmt.Errorf("%s ext-community { add | remove | replace } <value>...", usage)
		}
		stmt.Actions.BgpActions.SetExtCommunity.SetExtCommunityMethod.CommunitiesList = args[1:]
		switch strings.ToLower(args[0]) {
		case "add":
			stmt.Actions.BgpActions.SetExtCommunity.Options = string(config.BGP_SET_COMMUNITY_OPTION_TYPE_ADD)
		case "remove":
			stmt.Actions.BgpActions.SetExtCommunity.Options = string(config.BGP_SET_COMMUNITY_OPTION_TYPE_REMOVE)
		case "replace":
			stmt.Actions.BgpActions.SetExtCommunity.Options = string(config.BGP_SET_COMMUNITY_OPTION_TYPE_REPLACE)
		default:
			return fmt.Errorf("%s ext-community { add | remove | replace } <value>...", usage)
		}
	case "large-community":
		if len(args) < 1 {
			return fmt.Errorf("%s large-community { add | remove | replace } <value>...", usage)
		}
		stmt.Actions.BgpActions.SetLargeCommunity.SetLargeCommunityMethod.CommunitiesList = args[1:]
		switch strings.ToLower(args[0]) {
		case "add":
			stmt.Actions.BgpActions.SetLargeCommunity.Options = config.BGP_SET_COMMUNITY_OPTION_TYPE_ADD
		case "remove":
			stmt.Actions.BgpActions.SetLargeCommunity.Options = config.BGP_SET_COMMUNITY_OPTION_TYPE_REMOVE
		case "replace":
			stmt.Actions.BgpActions.SetLargeCommunity.Options = config.BGP_SET_COMMUNITY_OPTION_TYPE_REPLACE
		default:
			return fmt.Errorf("%s large-community { add | remove | replace } <value>...", usage)
		}
	case "med":
		if len(args) < 2 {
			return fmt.Errorf("%s med { add | sub | set } <value>", usage)
		}
		med, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		switch strings.ToLower(args[0]) {
		case "add":
			stmt.Actions.BgpActions.SetMed = config.BgpSetMedType(fmt.Sprintf("+%d", med))
		case "sub":
			stmt.Actions.BgpActions.SetMed = config.BgpSetMedType(fmt.Sprintf("-%d", med))
		case "set":
			stmt.Actions.BgpActions.SetMed = config.BgpSetMedType(fmt.Sprintf("%d", med))
		default:
			return fmt.Errorf("%s med { add | sub | set } <value>", usage)
		}
	case "local-pref":
		if len(args) < 1 {
			return fmt.Errorf("%s local-pref <value>", usage)
		}
		value, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			return err
		}
		stmt.Actions.BgpActions.SetLocalPref = uint32(value)
	case "as-prepend":
		if len(args) < 2 {
			return fmt.Errorf("%s as-prepend { <asn> | last-as } <repeat-value>", usage)
		}
		stmt.Actions.BgpActions.SetAsPathPrepend.As = args[0]
		repeat, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		stmt.Actions.BgpActions.SetAsPathPrepend.RepeatN = uint8(repeat)
	case "next-hop":
		if len(args) != 1 {
			return fmt.Errorf("%s next-hop { <value> | self }", usage)
		}
		stmt.Actions.BgpActions.SetNextHop = config.BgpNextHopType(args[0])
	}
	t, err := table.NewStatement(stmt)
	switch op {
	case CMD_ADD:
		err = client.AddStatement(t)
	case CMD_DEL:
		err = client.DeleteStatement(t, false)
	case CMD_SET:
		err = client.ReplaceStatement(t)
	default:
		return fmt.Errorf("invalid operation: %s", op)
	}
	return err
}

func modPolicy(modtype string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gobgp policy %s <name> [<statement name>...]", modtype)
	}
	name := args[0]
	args = args[1:]
	stmts := make([]config.Statement, 0, len(args))
	for _, n := range args {
		stmts = append(stmts, config.Statement{Name: n})
	}
	policy, err := table.NewPolicy(config.PolicyDefinition{
		Name:       name,
		Statements: stmts,
	})
	if err != nil {
		return err
	}
	switch modtype {
	case CMD_ADD:
		err = client.AddPolicy(policy, true)
	case CMD_DEL:
		all := false
		if len(args) < 1 {
			all = true
		}
		err = client.DeletePolicy(policy, all, true)
	case CMD_SET:
		err = client.ReplacePolicy(policy, true, true)
	}
	return err
}

func NewPolicyCmd() *cobra.Command {
	policyCmd := &cobra.Command{
		Use: CMD_POLICY,
		Run: func(cmd *cobra.Command, args []string) {
			err := showPolicy(args)
			if err != nil {
				exitWithError(err)
			}
		},
	}

	for _, v := range []string{CMD_PREFIX, CMD_NEIGHBOR, CMD_ASPATH, CMD_COMMUNITY, CMD_EXTCOMMUNITY, CMD_LARGECOMMUNITY} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(cmd *cobra.Command, args []string) {
				if err := showDefinedSet(cmd.Use, args); err != nil {
					exitWithError(err)
				}
			},
		}
		for _, w := range []string{CMD_ADD, CMD_DEL, CMD_SET} {
			subcmd := &cobra.Command{
				Use: w,
				Run: func(c *cobra.Command, args []string) {
					if err := modDefinedSet(cmd.Use, c.Use, args); err != nil {
						exitWithError(err)
					}
				},
			}
			cmd.AddCommand(subcmd)
		}
		policyCmd.AddCommand(cmd)
	}

	stmtCmdImpl := &cobra.Command{}
	for _, v := range []string{CMD_ADD, CMD_DEL, CMD_SET} {
		cmd := &cobra.Command{
			Use: v,
		}
		for _, w := range []string{CMD_CONDITION, CMD_ACTION} {
			subcmd := &cobra.Command{
				Use: w,
				Run: func(c *cobra.Command, args []string) {
					name := args[len(args)-1]
					args = args[:len(args)-1]
					var err error
					if c.Use == CMD_CONDITION {
						err = modCondition(name, cmd.Use, args)
					} else {
						err = modAction(name, cmd.Use, args)
					}
					if err != nil {
						exitWithError(err)
					}
				},
			}
			cmd.AddCommand(subcmd)
		}
		stmtCmdImpl.AddCommand(cmd)
	}

	stmtCmd := &cobra.Command{
		Use: CMD_STATEMENT,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if len(args) < 2 {
				err = showStatement(args)
			} else {
				args = append(args[1:], args[0])
				stmtCmdImpl.SetArgs(args)
				err = stmtCmdImpl.Execute()
			}
			if err != nil {
				exitWithError(err)
			}
		},
	}
	for _, v := range []string{CMD_ADD, CMD_DEL} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(c *cobra.Command, args []string) {
				err := modStatement(c.Use, args)
				if err != nil {
					exitWithError(err)
				}
			},
		}
		stmtCmd.AddCommand(cmd)
	}
	policyCmd.AddCommand(stmtCmd)

	for _, v := range []string{CMD_ADD, CMD_DEL, CMD_SET} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(c *cobra.Command, args []string) {
				err := modPolicy(c.Use, args)
				if err != nil {
					exitWithError(err)
				}
			},
		}
		policyCmd.AddCommand(cmd)
	}

	return policyCmd
}
