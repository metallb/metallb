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
	"fmt"
	"net"
	"time"

	"github.com/osrg/gobgp/packet/bgp"
	"github.com/spf13/cobra"
)

func showRPKIServer(args []string) error {
	servers, err := client.GetRPKI()
	if err != nil {
		fmt.Println(err)
		return err
	}
	if len(args) == 0 {
		format := "%-23s %-6s %-10s %s\n"
		fmt.Printf(format, "Session", "State", "Uptime", "#IPv4/IPv6 records")
		for _, r := range servers {
			s := "Down"
			uptime := "never"
			if r.State.Up == true {
				s = "Up"
				uptime = fmt.Sprint(formatTimedelta(int64(time.Now().Sub(time.Unix(r.State.Uptime, 0)).Seconds())))
			}

			fmt.Printf(format, net.JoinHostPort(r.Config.Address, fmt.Sprintf("%d", r.Config.Port)), s, uptime, fmt.Sprintf("%d/%d", r.State.RecordsV4, r.State.RecordsV6))
		}
	} else {
		for _, r := range servers {
			if r.Config.Address == args[0] {
				up := "Down"
				if r.State.Up == true {
					up = "Up"
				}
				fmt.Printf("Session: %s, State: %s\n", r.Config.Address, up)
				fmt.Println("  Port:", r.Config.Port)
				fmt.Println("  Serial:", r.State.SerialNumber)
				fmt.Printf("  Prefix: %d/%d\n", r.State.PrefixesV4, r.State.PrefixesV6)
				fmt.Printf("  Record: %d/%d\n", r.State.RecordsV4, r.State.RecordsV6)
				fmt.Println("  Message statistics:")
				fmt.Printf("    Receivedv4:    %10d\n", r.State.RpkiMessages.RpkiReceived.Ipv4Prefix)
				fmt.Printf("    Receivedv6:    %10d\n", r.State.RpkiMessages.RpkiReceived.Ipv4Prefix)
				fmt.Printf("    SerialNotify:  %10d\n", r.State.RpkiMessages.RpkiReceived.SerialNotify)
				fmt.Printf("    CacheReset:    %10d\n", r.State.RpkiMessages.RpkiReceived.CacheReset)
				fmt.Printf("    CacheResponse: %10d\n", r.State.RpkiMessages.RpkiReceived.CacheResponse)
				fmt.Printf("    EndOfData:     %10d\n", r.State.RpkiMessages.RpkiReceived.EndOfData)
				fmt.Printf("    Error:         %10d\n", r.State.RpkiMessages.RpkiReceived.Error)
				fmt.Printf("    SerialQuery:   %10d\n", r.State.RpkiMessages.RpkiSent.SerialQuery)
				fmt.Printf("    ResetQuery:    %10d\n", r.State.RpkiMessages.RpkiSent.ResetQuery)
			}
		}
	}
	return nil
}

func showRPKITable(args []string) error {
	family, err := checkAddressFamily(bgp.RouteFamily(0))
	if err != nil {
		exitWithError(err)
	}
	roas, err := client.GetROA(family)
	if err != nil {
		exitWithError(err)
	}

	var format string
	afi, _ := bgp.RouteFamilyToAfiSafi(family)
	if afi == bgp.AFI_IP {
		format = "%-18s %-6s %-10s %s\n"
	} else {
		format = "%-42s %-6s %-10s %s\n"
	}
	fmt.Printf(format, "Network", "Maxlen", "AS", "Server")
	for _, r := range roas {
		host, _, _ := net.SplitHostPort(r.Src)
		if len(args) > 0 && args[0] != host {
			continue
		}
		fmt.Printf(format, r.Prefix.String(), fmt.Sprint(r.MaxLen), fmt.Sprint(r.AS), r.Src)
	}
	return nil
}

func NewRPKICmd() *cobra.Command {
	rpkiCmd := &cobra.Command{
		Use: CMD_RPKI,
	}

	serverCmd := &cobra.Command{
		Use: CMD_RPKI_SERVER,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 || len(args) == 1 {
				showRPKIServer(args)
				return
			} else if len(args) != 2 {
				exitWithError(fmt.Errorf("usage: gobgp rpki server <ip address> [reset|softreset|enable]"))
			}
			addr := net.ParseIP(args[0])
			if addr == nil {
				exitWithError(fmt.Errorf("invalid ip address: %s", args[0]))
			}
			var err error
			switch args[1] {
			case "add":
				err = client.AddRPKIServer(addr.String(), 323, 0)
			case "reset":
				err = client.ResetRPKIServer(addr.String())
			case "softreset":
				err = client.SoftResetRPKIServer(addr.String())
			case "enable":
				err = client.EnableRPKIServer(addr.String())
			case "disable":
				err = client.DisableRPKIServer(addr.String())
			default:
				exitWithError(fmt.Errorf("unknown operation: %s", args[1]))
			}
			if err != nil {
				exitWithError(err)
			}
		},
	}
	rpkiCmd.AddCommand(serverCmd)

	tableCmd := &cobra.Command{
		Use: CMD_RPKI_TABLE,
		Run: func(cmd *cobra.Command, args []string) {
			showRPKITable(args)
		},
	}
	tableCmd.PersistentFlags().StringVarP(&subOpts.AddressFamily, "address-family", "a", "", "address family")

	validateCmd := &cobra.Command{
		Use: "validate",
		Run: func(cmd *cobra.Command, args []string) {
			if err := client.ValidateRIBWithRPKI(args...); err != nil {
				exitWithError(err)
			}
		},
	}
	rpkiCmd.AddCommand(validateCmd)

	rpkiCmd.AddCommand(tableCmd)
	return rpkiCmd
}
