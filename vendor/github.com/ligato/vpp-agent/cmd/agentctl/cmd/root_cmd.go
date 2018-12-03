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
	"github.com/spf13/cobra"
)

// GlobalFlags defines a single type to hold all cobra global flags.
type GlobalFlags struct {
	Endpoints []string
	Label     string
}

var globalFlags GlobalFlags

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:   "agentctl",
	Short: "A CLI tool for the vnf-agent",
	Long: `
A CLI tool to show the state of and to configure agents connected to
Etcd. Use the 'ETCD_ENDPOINTS'' environment variable or the 'endpoints'
flag in the command line to specify one or more Etcd instances to
connect to.`,
	Example: `Specify the etcd to connect to and list all agents that it knows about:
  $ export ETCD_ENDPOINTS=172.17.0.1:2379
  $ ./agentctl list

Do as above, but with a command line flag:
  $ ./agentctl --endpoints 172.17.0.1:2379 list
`,
}

func init() {
	// Root command flags
	RootCmd.PersistentFlags().StringSliceVarP(&globalFlags.Endpoints,
		"endpoints", "e", nil, "One or more comma-separated Etcd endpoints.")
	RootCmd.PersistentFlags().StringVarP(&globalFlags.Label, "label", "l", "",
		"Agent microservice label (identifies the agent)")
}
