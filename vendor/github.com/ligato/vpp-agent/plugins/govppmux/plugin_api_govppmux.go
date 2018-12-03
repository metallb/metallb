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

package govppmux

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging/measure/model/apitrace"
)

// TraceAPI is extended API with ability to get traced VPP binary API calls
type TraceAPI interface {
	API

	// GetTrace serves to obtain measured binary API calls
	GetTrace() *apitrace.Trace
}

// API for other plugins to get connectivity to VPP.
type API interface {
	// NewAPIChannel returns a new API channel for communication with VPP via govpp core.
	// It uses default buffer sizes for the request and reply Go channels.
	//
	// Example of binary API call from some plugin using GOVPP:
	//      ch, _ := govpp_mux.NewAPIChannel()
	//      ch.SendRequest(req).ReceiveReply
	NewAPIChannel() (govppapi.Channel, error)

	// NewAPIChannelBuffered returns a new API channel for communication with VPP via govpp core.
	// It allows to specify custom buffer sizes for the request and reply Go channels.
	//
	// Example of binary API call from some plugin using GOVPP:
	//      ch, _ := govpp_mux.NewAPIChannelBuffered(100, 100)
	//      ch.SendRequest(req).ReceiveReply
	NewAPIChannelBuffered(reqChanBufSize, replyChanBufSize int) (govppapi.Channel, error)
}
