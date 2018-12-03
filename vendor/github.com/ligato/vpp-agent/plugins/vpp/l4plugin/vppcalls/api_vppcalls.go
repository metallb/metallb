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

package vppcalls

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
)

// L4VppAPI provides methods for managing L4 layer configuration
type L4VppAPI interface {
	L4VppWrite
	L4VppRead
}

// L4VppWrite provides write methods for L4
type L4VppWrite interface {
	// EnableL4Features sets L4 feature flag on VPP to true
	EnableL4Features() error
	// DisableL4Features sets L4 feature flag on VPP to false
	DisableL4Features() error
	// AddAppNamespace calls respective VPP binary api to configure AppNamespace
	AddAppNamespace(secret uint64, swIfIdx, ip4FibID, ip6FibID uint32, id []byte) (appNsIdx uint32, err error)
}

// L4VppRead provides read methods for L4
type L4VppRead interface {
	// DumpL4Config returns L4 configuration
	DumpL4Config() ([]*SessionDetails, error)
}

// L4VppHandler is accessor for l4-related vppcalls methods
type L4VppHandler struct {
	callsChannel govppapi.Channel
	log          logging.Logger
}

// NewL4VppHandler creates new instance of L4 vppcalls handler
func NewL4VppHandler(callsChan govppapi.Channel, log logging.Logger) *L4VppHandler {
	return &L4VppHandler{
		callsChannel: callsChan,
		log:          log,
	}
}
