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

// Agentctl is a command-line tool for monitoring and configuring VPP Agents.
// The tool connects to an Etcd instance, discovers VPP Agents connected
// to the instance and monitors their status. The tool can also write VPP Agent
// configuration into Etcd. Note that the VPP Agent does not have
// to be connected to Etcd for agenctl to be able to change its configuration;
// the agent will simply receive the config with all the changes when it
// connects.
package main

import (
	"fmt"
	"os"

	"github.com/ligato/vpp-agent/cmd/agentctl/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
