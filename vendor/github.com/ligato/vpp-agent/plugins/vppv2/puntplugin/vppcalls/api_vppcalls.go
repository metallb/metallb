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
	"git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	punt "github.com/ligato/vpp-agent/api/models/vpp/punt"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
)

// PuntVppAPI provides methods for managing VPP punt configuration.
type PuntVppAPI interface {
	PuntVPPWrite
	PuntVPPRead
}

// PuntVPPWrite provides write methods for punt
type PuntVPPWrite interface {
	// AddPunt configures new punt to the host from the VPP
	AddPunt(punt *punt.ToHost) error
	// DeletePunt removes or unregisters punt entry
	DeletePunt(punt *punt.ToHost) error
	// RegisterPuntSocket registers new punt to unix domain socket entry
	RegisterPuntSocket(puntCfg *punt.ToHost) error
	// DeregisterPuntSocket removes existing punt to socket registration
	DeregisterPuntSocket(puntCfg *punt.ToHost) error
	// AddPuntRedirect adds new punt IP redirect entry
	AddPuntRedirect(punt *punt.IPRedirect) error
	// DeletePuntRedirect removes existing redirect entry
	DeletePuntRedirect(punt *punt.IPRedirect) error
}

// PuntVPPRead provides read methods for punt
type PuntVPPRead interface {
	// DumpPuntRegisteredSockets returns all punt socket registrations known to the VPP agent
	// TODO since the API to dump sockets is missing, the method works only with the entries in local cache
	DumpPuntRegisteredSockets() ([]*PuntDetails, error)
}

// PuntVppHandler is accessor for punt-related vppcalls methods.
type PuntVppHandler struct {
	callsChannel api.Channel
	ifIndexes    ifaceidx.IfaceMetadataIndex
	log          logging.Logger
}

// NewPuntVppHandler creates new instance of punt vppcalls handler
func NewPuntVppHandler(callsChan api.Channel, ifIndexes ifaceidx.IfaceMetadataIndex, log logging.Logger) *PuntVppHandler {
	return &PuntVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}
