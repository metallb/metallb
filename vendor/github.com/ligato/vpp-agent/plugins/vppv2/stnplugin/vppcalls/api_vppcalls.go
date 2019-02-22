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
	stn "github.com/ligato/vpp-agent/api/models/vpp/stn"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
)

// StnVppAPI provides methods for managing STN rules
type StnVppAPI interface {
	StnVppWrite
	StnVppRead
}

// StnVppWrite provides write methods for STN rules
type StnVppWrite interface {
	// AddSTNRule calls StnAddDelRule bin API with IsAdd=1
	AddSTNRule(stnRule *stn.Rule) error
	// DelSTNRule calls StnAddDelRule bin API with IsAdd=0
	DeleteSTNRule(stnRule *stn.Rule) error
}

// StnVppRead provides read methods for STN rules
type StnVppRead interface {
	// DumpSTNRules returns a list of all STN rules configured on the VPP
	DumpSTNRules() ([]*StnDetails, error)
}

// StnVppHandler is accessor for STN-related vppcalls methods
type StnVppHandler struct {
	callsChannel api.Channel
	ifIndexes    ifaceidx.IfaceMetadataIndex
	log          logging.Logger
}

// NewStnVppHandler creates new instance of STN vppcalls handler
func NewStnVppHandler(callsChan api.Channel, ifIndexes ifaceidx.IfaceMetadataIndex, log logging.Logger) *StnVppHandler {
	return &StnVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}
