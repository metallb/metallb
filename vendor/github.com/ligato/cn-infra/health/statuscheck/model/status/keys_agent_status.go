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

package status

const (
	// StatusPrefix is the relative key prefix for the agent/plugin status.
	StatusPrefix = "check/status/v1/"
	// AgentStatusPrefix is the relative key prefix for the agent status,
	// filtering out statuses of individual plugins.
	AgentStatusPrefix = StatusPrefix + "agent"
)

// AgentStatusKey returns the key used in ETCD to store the operational status
// of the vpp agent instance.
func AgentStatusKey() string {
	return AgentStatusPrefix
}

// PluginStatusKey returns the key used in ETCD to store the operational status
// of the vpp agent plugin.
func PluginStatusKey(pluginLabel string) string {
	return StatusPrefix + "plugin/" + pluginLabel
}
