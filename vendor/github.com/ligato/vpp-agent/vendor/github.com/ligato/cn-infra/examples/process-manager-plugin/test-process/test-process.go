// Copyright (c) 2018 Cisco and/or its affiliates.
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

// Test application just keeps running indefinitely, or for given time is defined
// via parameter creating a running process. The purpose is to serve as a test
// application for process manager example.

package main

import (
	"time"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/namsral/flag"
)

var log = logrus.DefaultLogger()
var maxUptime = flag.Uint("max-uptime", 0, "Max uptime in seconds the test application can be running")

func main() {
	flag.Parse()

	ta := &TestApp{}
	ta.run()
}

// TestApp is test application for process manager example
type TestApp struct{}

// Run ticker literally forever, or for given time if set
func (ta *TestApp) run() {
	log.Info("test application started")
	if *maxUptime != 0 {
		log.Infof("max uptime set to %d second(s)", *maxUptime)
	}

	ticker := time.NewTicker(1 * time.Second)
	var uptime uint

	func() {
		for range ticker.C {
			uptime++
			if uptime == *maxUptime {
				log.Info("time's up, bye")
				return
			}
		}
	}()

	log.Info("test application ended")
	return
}
