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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
)

var listAgents = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l", "li"},
	Short:   "List agents recorded in Etcd (active or inactive)",
	Long: `
List all Agents for which records exist in Etcd. A record may be
configuration for an agent that is stored in Etcd by a 3rd party
entity, or state stored in Etcd by an agent.

Agents for which only configuration records exist (i.e. they do
not push status records into Etcd) are listed as 'INACTIVE'.`,
	Run: listAgentsFunc,
}

func init() {
	RootCmd.AddCommand(listAgents)
	listAgents.Flags().BoolVar(&showEtcd, "etcd", false,
		"Show Etcd Metadata (revision, key))")
}

func listAgentsFunc(cmd *cobra.Command, args []string) {
	db, err := utils.GetDbForAllAgents(globalFlags.Endpoints)
	if err != nil {
		utils.ExitWithError(utils.ExitError, errors.New("Failed connect to Etcd - "+err.Error()))
	}

	keyIter, err := db.ListKeys(servicelabel.GetAllAgentsPrefix())
	if err != nil {
		utils.ExitWithError(utils.ExitError, errors.New("Failed to get keys - "+err.Error()))
	}

	ed := utils.NewEtcdDump()
	for {
		if key, _, done := keyIter.GetNext(); !done {
			found, err := ed.ReadDataFromDb(db, key, nil, []string{status.StatusPrefix})
			if err != nil {
				utils.ExitWithError(utils.ExitError, err)
			} else if !found {
				label, _, _, _ := utils.ParseKey(key)
				if _, ok := ed[label]; !ok {
					ed.CreateEmptyRecord(key)
				}
			}
			continue
		}
		break
	}

	if len(ed) > 0 {
		buffer, err := ed.PrintDataAsText(showEtcd, false)
		if err == nil {
			fmt.Print(buffer.String())
		} else {
			fmt.Printf("Error: %v", err)
		}
	} else {
		fmt.Print("No data found.\n")
	}
}
