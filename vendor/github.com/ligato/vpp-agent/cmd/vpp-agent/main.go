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

// Package vpp-agent implements the main entry point into the VPP Agent
// and it is used to build the VPP Agent executable.
package main

import (
	"fmt"
	"os"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/logging"
	log "github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/cmd/vpp-agent/app/v2"
)

const logo = `                                      __
 _  _____  ___ _______ ____ ____ ___ / /_
| |/ / _ \/ _ /___/ _ '/ _ '/ -_/ _ / __/
|___/ .__/ .__/   \_'_/\_' /\__/_//_\__/  %s
   /_/  /_/           /___/

`

var vppAgent = appv2.New()

func main() {
	fmt.Fprintf(os.Stdout, logo, agent.BuildVersion)

	a := agent.NewAgent(agent.AllPlugins(vppAgent))

	if err := a.Run(); err != nil {
		log.DefaultLogger().Fatal(err)
	}
}

func init() {
	log.DefaultLogger().SetOutput(os.Stdout)
	log.DefaultLogger().SetLevel(logging.DebugLevel)
}
