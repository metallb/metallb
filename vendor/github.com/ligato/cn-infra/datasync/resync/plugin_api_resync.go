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

import "github.com/ligato/cn-infra/infra"

// Subscriber is an API used by plugins to register for notifications from the
// RESYNC Orcherstrator.
type Subscriber interface {
	// Register function is supposed to be called in Init() by all VPP Agent plugins.
	// Those plugins will use Registration.StatusChan() to listen
	// The plugins are supposed to load current state of their objects when newResync() is called.
	Register(resyncName string) Registration
}

// Reporter is an API for other plugins that need to report to RESYNC Orchestrator.
// Intent of this API is to have a chance to react on error by triggering
// RESYNC among registered plugins.
type Reporter interface {
	// ReportError is called by Plugins when the binary api call was not successful.
	// Based on that the Resync Orchestrator starts the Resync.
	ReportError(name infra.PluginName, err error)
}
