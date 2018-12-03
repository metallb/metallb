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

package statuscheck

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/ligato/cn-infra/infra"
)

//go:generate protoc --proto_path=model/status --gogo_out=model/status model/status/status.proto

// PluginState is a data type used to describe the current operational state
// of a plugin.
type PluginState string

const (
	// Init state means that the initialization of the plugin is in progress.
	Init PluginState = "init"
	// OK state means that the plugin is healthy.
	OK PluginState = "ok"
	// Error state means that some error has occurred in the plugin.
	Error PluginState = "error"
)

// PluginStateProbe defines parameters of a function used for plugin state
// probing, referred to as "probe".
type PluginStateProbe func() (PluginState, error)

// PluginStatusWriter allows to register & write plugin status by other plugins
type PluginStatusWriter interface {
	// Register registers a plugin for status change reporting.
	// If <probe> is not nil, Statuscheck will periodically probe the plugin
	// state through the provided function. Otherwise, it is expected that the
	// plugin itself will report state updates through ReportStateChange().
	Register(pluginName infra.PluginName, probe PluginStateProbe)

	// ReportStateChange can be used to report a change in the status
	// of a previously registered plugin. It is not a bug, however, to report
	// the same status in consecutive calls. Statuscheck is smart enough
	// to detect an actual status change and propagate only updates to remote
	// clients.
	ReportStateChange(pluginName infra.PluginName, state PluginState, lastError error)

	// ReportStateChangeWithMeta can be used to report a change in the status
	// of a previously registered plugin with added metadata value stored in
	// global agent status. Metadata type is specified in statuscheck model.
	ReportStateChangeWithMeta(pluginName infra.PluginName, state PluginState, lastError error, meta proto.Message)
}

// AgentStatusReader allows to lookup agent status by other plugins.
type AgentStatusReader interface {
	// GetAgentStatus returns the current global operational state of the agent.
	GetAgentStatus() status.AgentStatus
}

// InterfaceStatusReader looks up the interface state and returns updated state data
type InterfaceStatusReader interface {
	GetInterfaceStats() status.InterfaceStats
}

// StatusReader allows to lookup agent status and retrieve a map containing status of all plugins.
type StatusReader interface {
	AgentStatusReader
	InterfaceStatusReader
	GetAllPluginStatus() map[string]*status.PluginStatus
}
