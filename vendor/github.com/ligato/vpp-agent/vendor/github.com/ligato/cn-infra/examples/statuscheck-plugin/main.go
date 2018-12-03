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

package main

import (
	"log"
	"time"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync/kvdbsync"
	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
)

// *************************************************************************
// This example demonstrates the usage of StatusReader API
// ETCD plugin is monitored by status check plugin.
// ExamplePlugin periodically prints the status.
// ************************************************************************/

// PluginName represents name of plugin.
const PluginName = "example"

func main() {
	// Prepare ETCD data sync plugin as an plugin dependency
	etcdDataSync := kvdbsync.NewPlugin(
		kvdbsync.UseDeps(func(deps *kvdbsync.Deps) {
			deps.KvPlugin = &etcd.DefaultPlugin
			deps.ResyncOrch = &resync.DefaultPlugin
		}),
	)
	// Init example plugin dependencies
	p := &ExamplePlugin{
		Log:             logging.ForPlugin(PluginName),
		StatusMonitor:   &statuscheck.DefaultPlugin,
		exampleFinished: make(chan struct{}),
	}
	// Start Agent with example plugin including dependencies
	a := agent.NewAgent(
		agent.AllPlugins(etcdDataSync, p),
		agent.QuitOnClose(p.exampleFinished),
	)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

// ExamplePlugin demonstrates the usage of datasync API.
type ExamplePlugin struct {
	Log           logging.PluginLogger
	StatusMonitor statuscheck.StatusReader

	// Fields below are used to properly finish the example.
	exampleFinished chan struct{}
}

// String return plugin name.
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Init starts the consumer.
func (plugin *ExamplePlugin) Init() error {
	return nil
}

// AfterInit starts the publisher and prepares for the shutdown.
func (plugin *ExamplePlugin) AfterInit() error {
	go plugin.checkStatus(plugin.exampleFinished)

	return nil
}

// checkStatus periodically prints status of plugins that publish their state
// to status check plugin
func (plugin *ExamplePlugin) checkStatus(closeCh chan struct{}) {
	for {
		select {
		case <-closeCh:
			plugin.Log.Info("Closing")
			return
		case <-time.After(1 * time.Second):
			status := plugin.StatusMonitor.GetAllPluginStatus()
			for k, v := range status {
				plugin.Log.Infof("Status[%v] = %v", k, v)
			}
		}
	}
}

// Close shutdowns the consumer and channels used to propagate data resync and data change events.
func (plugin *ExamplePlugin) Close() error {
	return safeclose.Close(plugin.exampleFinished)
}
