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
	"time"

	"strings"

	"github.com/buger/goterm"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/spf13/cobra"
)

var watchCommand = &cobra.Command{
	Use:     "watch [agent-label-filter(s)] [interface-filter(s)]",
	Aliases: []string{"w", "wa"},
	Short:   "Show real-time traffic data for vpp interfaces",
	Long: `
'watch' displays a table with periodically updated interface statistics
counters. Data is printed for every agent whose configuration contains
at least one interface with statistics (in case no filters are applied).

Data can be filtered using one or more agent's microservice labels (only
data for specified labels are shown) and, in case microservice label is
defined, it is possible to filter interfaces using the second argument
[interface-filter(s)] for that agent. It is also possible to filter multiple
interfaces, however, it is not allowed to filter interfaces if more than
one agent is specified (see examples)

Note that agent's configuration data is stored in Etcd by a 3rd party
orchestrator. Agent's state data is periodically updated in Etcd by agents
themselves every 10 seconds. `,
	Example: `  Watch all agent with all interfaces containing statistics:
    $ agentctl watch
  Watch statistics for the specific agent:
    $ agentctl watch vpp
  Watch statistics for multiple agents. Note: in this case is not possible
  to filter interfaces:
    $ agentctl watch vpp1,vpp2
    $ agentctl watch "vpp1, vpp2"
    $ agentctl watch 'vpp1, vpp2'
  Watch statistics for the specific interface:
    $ agentctl watch vpp interface
  Watch statistics for multiple interfaces:
    $ agentctl watch vpp interface1,interface2
    $ agentctl watch vpp "interface1, interface2"
    $ agentctl watch vpp 'interface1,interface2'
  Show shortened output (in_packets, out_packets and drop only). Can be
  arbitrarily combined with filters.
    $ agentctl watch --short
  Filter all interfaces/statistic types with full zero values. Can be arbitrarily
  combined with filters. Note: if used together with --short, it is
  possible to see full zero output because there could be fields which are
  non-zero and not shown in shortened output:
    $ agentctl watch --active`,
	Run: watchFunc,
}

var (
	showShort  bool
	showActive bool
)

func init() {
	RootCmd.AddCommand(watchCommand)
	watchCommand.Flags().BoolVarP(&showShort, "short", "s", false,
		"show shortened list of statistics for every vpp/interface")
	watchCommand.Flags().BoolVarP(&showActive, "active", "a", false,
		"only vpp/interfaces with non-zero statistics will be displayed")
}

func watchFunc(cmd *cobra.Command, args []string) {
	if len(args) > 2 {
		utils.ExitWithError(utils.ExitBadArgs, errors.New("Too many arguments, use 'show <label> <interface>'"))
	}

	// Clear current screen
	goterm.Clear()
	for {
		// Get console dimensions
		width := goterm.Width()
		height := goterm.Height()

		if width != 0 && height != 0 {
			goterm.Clear()
		}
		// This hooks watcher to the top of the console window
		goterm.MoveCursor(1, 1)
		table := goterm.NewTable(0, 10, 2, ' ', 0)

		if width < 145 && !showShort {
			fmt.Fprintf(table, "%s", utils.NoSpace)
		} else {

			etcdDump := getLatestEtcdData()

			if showShort {
				_, err := etcdDump.PrintDataAsTable(table, args, showShort, showActive)
				if err != nil {
					utils.ExitWithError(utils.ExitError, errors.New(err.Error()))
				}
			} else {
				_, err := etcdDump.PrintDataAsTable(table, args, showShort, showActive)
				if err != nil {
					utils.ExitWithError(utils.ExitError, errors.New(err.Error()))
				}
			}
		}

		goterm.Println(table)

		// Fill the output buffer and ensure that it will not overflow the screen.
		for idx, str := range strings.Split(goterm.Screen.String(), "\n") {
			if idx > goterm.Height() {
				break
			}
			goterm.Output.WriteString(str + "\n")
		}
		// Write (flush) buffered data.
		err := goterm.Output.Flush()
		if err != nil {
			logrus.DefaultLogger().Errorf("%v", err)
		}
		// Reset the screen buffer.
		goterm.Screen.Reset()

		fmt.Println("Press Ctrl-C to exit watcher ...")

		// etcd interface statistics are updated every 10 seconds (interval set directly in VPP).
		time.Sleep(time.Second)
	}
}

// Read latest etcd data to keep counters updated.
func getLatestEtcdData() utils.EtcdDump {
	// Get data broker
	dataBroker, err := utils.GetDbForAllAgents(globalFlags.Endpoints)
	if err != nil {
		utils.ExitWithError(utils.ExitError, errors.New("Failed to connect to Etcd - "+err.Error()))
	}
	// Read agent prefixes.
	keyIter, err := dataBroker.ListKeys(servicelabel.GetAllAgentsPrefix())
	if err != nil {
		utils.ExitWithError(utils.ExitError, errors.New("Failed to get keys - "+err.Error()))
	}
	etcdDump := utils.NewEtcdDump()
	for {
		if key, _, done := keyIter.GetNext(); !done {
			if _, err = etcdDump.ReadDataFromDb(dataBroker, key, nil, nil); err != nil {
				utils.ExitWithError(utils.ExitError, err)
			}
			continue
		}
		break
	}
	return etcdDump
}
