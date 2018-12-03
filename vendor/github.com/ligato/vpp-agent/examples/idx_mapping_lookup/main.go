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

package main

import (
	"log"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
)

// *************************************************************************
// This file contains an example of the use of the name-to-index mapping registry
// to register items with unique names, and indexes, and metadata
// and to read these values.
// ************************************************************************/

// Main allows running Example Plugin as a statically linked binary with Agent Core Plugins. Close channel and plugins
// required for the example are initialized. Agent is instantiated with generic plugins (etcd, Kafka, Status check,
// HTTP and Log) and example plugin which demonstrates index mapping lookup functionality.
func main() {
	ep := &ExamplePlugin{
		Log:             logging.DefaultLogger,
		exampleFinished: make(chan struct{}),
	}

	// Start Agent
	a := agent.NewAgent(
		agent.AllPlugins(ep),
		agent.QuitOnClose(ep.exampleFinished),
	)
	if err := a.Run(); err != nil {
		log.Fatal()
	}
}

// PluginName represents name of plugin.
const PluginName = "idx-mapping-lookup"

// ExamplePlugin implements Plugin interface which is used to pass custom plugin instances to the Agent.
type ExamplePlugin struct {
	exampleIdx   idxvpp.NameToIdxRW // Name to index mapping registry
	exampleIDSeq uint32             // Provides unique ID for every item stored in mapping
	// Fields below are used to properly finish the example.
	exampleFinished chan struct{}

	Log logging.Logger
}

// Init is the entry point into the plugin that is called by Agent Core when the Agent is coming up.
// The Go native plugin mechanism that was introduced in Go 1.8
func (plugin *ExamplePlugin) Init() (err error) {
	// Init new name-to-index mapping.
	plugin.exampleIdx = nametoidx.NewNameToIdx(logrus.DefaultLogger(), "example_index", nil)

	// Set the initial ID. After every registration, this ID has to be incremented
	// so new mapping is registered under a unique number.
	plugin.exampleIDSeq = 1

	plugin.Log.Info("Initialization of the custom plugin for the idx-mapping lookup example is completed")

	// Demonstrate mapping lookup functionality.
	plugin.exampleMappingUsage()

	// End the example.
	plugin.Log.Infof("idx-mapping-lookup example finished, sending shutdown ...")
	close(plugin.exampleFinished)

	return err
}

// Close cleans up the resources.
func (plugin *ExamplePlugin) Close() error {
	return nil
}

// String returns plugin name
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Meta structure. It can contain any number of fields of different types. Metadata is optional and can be nil.
type Meta struct {
	ip     string
	prefix uint32
}

// Illustration of index-mapping lookup usage.
func (plugin *ExamplePlugin) exampleMappingUsage() {
	// Random name used to registration. Every registered name should be unique.
	name := "example-entity"

	// Register name, and unique ID, and metadata to the example index map. Metadata
	// are optional, can be nil. Name and ID have to be unique, otherwise the mapping will be overridden.
	plugin.exampleIdx.RegisterName(name, plugin.exampleIDSeq, &Meta{})
	plugin.Log.Infof("Name %v registered", name)

	// Find the registered mapping using lookup index (name has to be known). The function
	// returns an index related to the provided name, and metadata (nil if there are no metadata
	// or mapping was not found), and a bool flag saying whether the mapping with provided name was found or not.
	_, meta, found := plugin.exampleIdx.LookupIdx(name)
	if found && meta != nil {
		plugin.Log.Infof("Name %v stored in mapping", name)
	} else {
		plugin.Log.Errorf("Name %v not found", name)
	}

	// Find the registered mapping using lookup name (index has to be known). The function
	// returns a name related to provided index, and metadata (nil if there are no metadata
	// or mapping was not found), and a bool flag saying whether the mapping with provided index was found or not.
	_, meta, found = plugin.exampleIdx.LookupName(plugin.exampleIDSeq)
	if found && meta != nil {
		plugin.Log.Infof("Index %v stored in mapping", plugin.exampleIDSeq)
	} else {
		plugin.Log.Errorf("Index %v not found", plugin.exampleIDSeq)
	}

	// This is how to remove mapping from registry. Other plugins can be notified about this change.
	plugin.exampleIdx.UnregisterName(name)
	plugin.Log.Infof("Name %v unregistered", name)
}
