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

package resync

// Status used in the events.
type Status string

const (
	// Started means that the Resync has started.
	Started Status = "Started"
	// NotActive means that Resync has not started yet or it has been finished.
	NotActive Status = "NotActive"
)

// StatusEvent is the base type that will be propagated to the channel.
type StatusEvent interface {
	// Status() is used by the Plugin if it needs to Start resync.
	ResyncStatus() Status

	// Ack() is used by the Plugin to acknowledge that it processed this event.
	// This is supposed to be called after the configuration was applied by the Plugin.
	Ack()
}
