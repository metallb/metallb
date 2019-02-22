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

package localclient

// Plugin allows loading Linux localclient as a plugin
// (even though the Init() method does not really do anything).
type Plugin struct {
}

// Init does nothing.
func (plugin *Plugin) Init() error {
	return nil
}

// Close does nothing
func (plugin *Plugin) Close() error {
	return nil
}
