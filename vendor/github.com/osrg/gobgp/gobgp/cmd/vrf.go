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
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/spf13/cobra"
	"sort"
	"strconv"
	"strings"
)

func getVrfs() (vrfs, error) {
	ret, err := client.GetVRF()
	if err != nil {
		return nil, err
	}
	sort.Sort(vrfs(ret))
	return ret, nil
}

func showVrfs() error {
	maxLens := []int{20, 20, 20, 20, 5}
	vrfs, err := getVrfs()
	if err != nil {
		return err
	}
	if globalOpts.Json {
		j, _ := json.Marshal(vrfs)
		fmt.Println(string(j))
		return nil
	}
	if globalOpts.Quiet {
		for _, v := range vrfs {
			fmt.Println(v.Name)
		}
		return nil
	}
	lines := make([][]string, 0, len(vrfs))
	for _, v := range vrfs {
		name := v.Name
		rd := v.Rd.String()

		f := func(rts []bgp.ExtendedCommunityInterface) (string, error) {
			ret := make([]string, 0, len(rts))
			for _, rt := range rts {
				ret = append(ret, rt.String())
			}
			return strings.Join(ret, ", "), nil
		}

		importRts, _ := f(v.ImportRt)
		exportRts, _ := f(v.ExportRt)
		lines = append(lines, []string{name, rd, importRts, exportRts, fmt.Sprintf("%d", v.Id)})

		for i, v := range []int{len(name), len(rd), len(importRts), len(exportRts)} {
			if v > maxLens[i] {
				maxLens[i] = v + 4
			}
		}

	}
	format := fmt.Sprintf("  %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds\n", maxLens[0], maxLens[1], maxLens[2], maxLens[3], maxLens[4])
	fmt.Printf(format, "Name", "RD", "Import RT", "Export RT", "ID")
	for _, l := range lines {
		fmt.Printf(format, l[0], l[1], l[2], l[3], l[4])
	}
	return nil
}

func showVrf(name string) error {
	return showNeighborRib(CMD_VRF, name, nil)
}

func modVrf(typ string, args []string) error {
	var err error
	switch typ {
	case CMD_ADD:
		a := extractReserved(args, []string{"rd", "rt", "id"})
		if len(a[""]) != 1 || len(a["rd"]) != 1 || len(a["rt"]) < 2 || len(a["id"]) > 1 {
			return fmt.Errorf("Usage: gobgp vrf add <vrf name> [ id <id> ] rd <rd> rt { import | export | both } <rt>...")
		}
		name := a[""][0]
		var rd bgp.RouteDistinguisherInterface
		rd, err = bgp.ParseRouteDistinguisher(a["rd"][0])
		if err != nil {
			return err
		}
		cur := ""
		importRt := make([]bgp.ExtendedCommunityInterface, 0)
		exportRt := make([]bgp.ExtendedCommunityInterface, 0)
		for _, elem := range a["rt"] {
			if elem == "import" || elem == "export" || elem == "both" {
				cur = elem
				continue
			}
			rt, err := bgp.ParseRouteTarget(elem)
			if err != nil {
				return err
			}
			switch cur {
			case "import":
				importRt = append(importRt, rt)
			case "export":
				exportRt = append(exportRt, rt)
			case "both":
				importRt = append(importRt, rt)
				exportRt = append(exportRt, rt)
			default:
				return fmt.Errorf("Usage: gobgp vrf add <vrf name> rd <rd> rt { import | export | both } <rt>...")
			}
		}
		vrfId := 0
		if len(a["id"]) > 0 {
			vrfId, err = strconv.Atoi(a["id"][0])
			if err != nil {
				return err
			}
		}
		err = client.AddVRF(name, vrfId, rd, importRt, exportRt)
	case CMD_DEL:
		if len(args) != 1 {
			return fmt.Errorf("Usage: gobgp vrf del <vrf name>")
		}
		err = client.DeleteVRF(args[0])
	}
	return err
}

func NewVrfCmd() *cobra.Command {

	ribCmd := &cobra.Command{
		Use: CMD_RIB,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if len(args) == 1 {
				err = showVrf(args[0])
			} else {
				err = fmt.Errorf("usage: gobgp vrf <vrf-name> rib")
			}
			if err != nil {
				exitWithError(err)
			}
		},
	}

	for _, v := range []string{CMD_ADD, CMD_DEL} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(cmd *cobra.Command, args []string) {
				err := modPath(CMD_VRF, args[len(args)-1], cmd.Use, args[:len(args)-1])
				if err != nil {
					exitWithError(err)
				}
			},
		}
		ribCmd.AddCommand(cmd)
	}

	neighborCmd := &cobra.Command{
		Use: CMD_NEIGHBOR,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if len(args) == 1 {
				var vs vrfs
				vs, err = getVrfs()
				if err != nil {
					exitWithError(err)
				}
				found := false
				for _, v := range vs {
					if v.Name == args[0] {
						found = true
						break
					}
				}
				if !found {
					err = fmt.Errorf("vrf %s not found", args[0])
				} else {
					err = showNeighbors(args[0])
				}
			} else {
				err = fmt.Errorf("usage: gobgp vrf <vrf-name> neighbor")
			}
			if err != nil {
				exitWithError(err)
			}
		},
	}

	vrfCmdImpl := &cobra.Command{}
	vrfCmdImpl.AddCommand(ribCmd, neighborCmd)

	vrfCmd := &cobra.Command{
		Use: CMD_VRF,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			if len(args) == 0 {
				err = showVrfs()
			} else if len(args) == 1 {
			} else {
				args = append(args[1:], args[0])
				vrfCmdImpl.SetArgs(args)
				err = vrfCmdImpl.Execute()
			}
			if err != nil {
				exitWithError(err)
			}
		},
	}

	for _, v := range []string{CMD_ADD, CMD_DEL} {
		cmd := &cobra.Command{
			Use: v,
			Run: func(cmd *cobra.Command, args []string) {
				err := modVrf(cmd.Use, args)
				if err != nil {
					exitWithError(err)
				}
			},
		}
		vrfCmd.AddCommand(cmd)
	}
	vrfCmd.PersistentFlags().StringVarP(&subOpts.AddressFamily, "address-family", "a", "", "address family")

	return vrfCmd
}
