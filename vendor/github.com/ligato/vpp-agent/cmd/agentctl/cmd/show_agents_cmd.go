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

package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"fmt"
	"strings"

	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
)

var showAgents = &cobra.Command{
	Use:     "show [agent-label-filter]",
	Aliases: []string{"s", "sh"},
	Short:   "Show detailed config and status data",
	Long: `
'show' prints out Etcd configuration and status data (where applicable)
for agents whose microservice label matches the label filter specified
in the command's '[agent-label-filter] argument. The filter contains a
list of comma-separated strings. A match is performed for each string
in the filter list.

The commands can print out agents' data in one of three formats - plain
text, tree or json'.

Note that agent's configuration data is stored into Etcd by a 3rd party
orchestrator. Agent's state data is periodically updated in Etcd by
healthy agents themselves. Agents for which only configuration records
exist (i.e. they do not push status records into Etcd) are listed as
'INACTIVE'.

The etcd flag set to true enables the printout of Etcd metadata for each
data record (except JSON-formatted output)
`,
	Example: `  Show agents where agent microservice label matches "vpp1":
    $ agentctl show vpp1
  Show agents where agent microservice label matches "vpp" and "cable"":
    $ agentctl show vpp,cable
    $ agentctl show "vpp, cable"
    $ agentctl show 'vpp, cable'
  Show all agents (no filter):
    $ agentctl show
  Show all agents, out put in tree format:
    $ agentctl show -f tree
  Show all agents, out put in json format (including filtering):
    $ agentctl show -f json
    $ agentctl show -f json vpp1
    $ agentctl show -f json vpp1,vpp2
    $ agentctl show -f json "vpp1, vpp2"
    $ agentctl show -f json 'vpp1,vpp2'`,

	Run: showAgentsFunc,
}

var (
	showEtcd    bool
	printFormat string
)

func init() {
	RootCmd.AddCommand(showAgents)
	showAgents.Flags().BoolVar(&showEtcd, "etcd", false,
		"Show Etcd Metadata for each record (revision, key)")
	showAgents.Flags().StringVarP(&printFormat, "format", "f", "txt",
		"Format of printed data (txt | tree | json)")
}

func showAgentsFunc(cmd *cobra.Command, args []string) {
	db, err := utils.GetDbForAllAgents(globalFlags.Endpoints)
	if err != nil {
		utils.ExitWithError(utils.ExitError, errors.New("Failed to connect to Etcd - "+err.Error()))
	}

	keyIter, err := db.ListKeys(servicelabel.GetAllAgentsPrefix())
	if err != nil {
		utils.ExitWithError(utils.ExitError, errors.New("Failed to get keys - "+err.Error()))
	}

	filter := []string{}
	if len(args) > 0 {
		filter = strings.Split(args[0], ",")
	}

	ed := utils.NewEtcdDump()
	for {
		if key, _, done := keyIter.GetNext(); !done {
			// fmt.Printf("Key: '%s'\n", key)
			if _, err = ed.ReadDataFromDb(db, key, filter, nil); err != nil {
				utils.ExitWithError(utils.ExitError, err)
			}
			continue
		}
		break
	}

	if len(ed) > 0 {
		switch printFormat {
		case "txt":
			buffer, err := ed.PrintDataAsText(showEtcd, false)
			if err == nil {
				fmt.Print(buffer.String())
			} else {
				fmt.Printf("Error: %v", err)
			}
		case "tree":
			ed.PrintDataAsText(showEtcd, true)
			// Data are rendered within render method.
		case "json":
			buffer, err := ed.PrintDataAsJSON(args)
			if err != nil {
				utils.ExitWithError(utils.ExitError, errors.New("Error while JSON processing"))
			}
			fmt.Print(buffer.String())
		default:
			utils.ExitWithError(utils.ExitInvalidInput, errors.New("Invalid text format"))
		}
	} else {
		fmt.Print("No data found.\n")
	}
}
