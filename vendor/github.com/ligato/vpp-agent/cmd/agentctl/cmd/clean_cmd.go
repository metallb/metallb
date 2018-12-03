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

	"bufio"
	"os"
	"strings"

	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

const dataTypeFlagName = "dataType"

var cleanCommand = &cobra.Command{
	Use:     "clean [agent-label-filter]",
	Aliases: []string{"c", "cl"},
	Short:   "Delete data for specified vpp(s) & data type(s)",
	Long: fmt.Sprintf(`
'clean' deletes from Etcd the data that matches both the label filter
specified in the [agent-label-filter] argument and the Data Type filter
specified in the '%s' flag. Both filters contain lists of comma-
separated strings. A match is performed for each string in the list.

The '%s' flag may contain the following data types:
  - %s
  - %s
  - %s
  - %s
  - %s
  - %s
If no data type filter is specified, all data for the specified vpp(s)
will be deleted. If no [agent-label-filter] argument is specified, data
for all agents will be deleted.`,
		dataTypeFlagName, dataTypeFlagName,
		status.StatusPrefix, interfaces.Prefix,
		interfaces.StatePrefix, l2.BdPrefix,
		l2.XConnectPrefix, l3.RoutesPrefix),
	Example: fmt.Sprintf(`  Delete all data for "vpp1":
    $ agentctl clean vpp1
  Delete status data for "vpp1"":
    $ agentctl clean vpp1 -dataType %s
  Delete status and interface data for "vpp1"":
    $ agentctl clean vpp1 -dataType %s,%s
  Delete all data for all agents (no filter):
    $ agentctl clean`,
		status.StatusPrefix, status.StatusPrefix, interfaces.Prefix),
	Run: cleanFunc,
}

var dataTypeFilter []string

func init() {
	RootCmd.AddCommand(cleanCommand)
	cleanCommand.Flags().StringSliceVarP(&dataTypeFilter, dataTypeFlagName, "d", []string{},
		"Data Type filter (see usage)")
}

func cleanFunc(cmd *cobra.Command, args []string) {

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nType 'yes' or 'y' to confirm: ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm == "yes" || confirm == "y" {
		db, err := utils.GetDbForAllAgents(globalFlags.Endpoints)
		if err != nil {
			utils.ExitWithError(utils.ExitError, errors.New("Failed to connect to Etcd - "+err.Error()))
		}

		keyIter, err := db.ListKeys(servicelabel.GetAllAgentsPrefix())
		if err != nil {
			utils.ExitWithError(utils.ExitError, errors.New("Failed to get keys - "+err.Error()))
		}

		lblFilter := []string{}
		dtFilter := []string{}
		if len(args) > 0 {
			lblFilter = strings.Split(args[0], ",")
		}

		total := 0
		for {
			if key, _, done := keyIter.GetNext(); !done {
				//fmt.Printf("Key: '%s'\n", key)
				if found, err := utils.DeleteDataFromDb(db, key, lblFilter, dtFilter); err != nil {
					utils.ExitWithError(utils.ExitError, err)
				} else if found {
					total++
				}
				continue
			}
			break
		}
		fmt.Printf("%d items deleted.\n", total)

	}
}
