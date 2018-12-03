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
	"bytes"
	"fmt"
	"net"

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/session"
)

// SessionDetails represents session details.
type SessionDetails struct {
	TransportProto uint8
	LocalIP        string
	LocalPort      uint16
	RemoteIP       string
	RemotePort     uint16
	ActionIndex    uint32
	AppnsIndex     uint32
	Scope          uint8
	Tag            string
}

// DumpL4Config implements L4VppRead.
func (h *L4VppHandler) DumpL4Config() ([]*SessionDetails, error) {
	var appNsDetails []*SessionDetails
	// Dump ARPs.
	reqCtx := h.callsChannel.SendMultiRequest(&session.SessionRulesDump{})

	for {
		sessions := &session.SessionRulesDetails{}
		stop, err := reqCtx.ReceiveReply(sessions)
		if stop {
			break
		}
		if err != nil {
			h.log.Error(err)
			return nil, err
		}

		// IP addresses
		var localIP, remoteIP string
		if uintToBool(sessions.IsIP4) {
			localIP = fmt.Sprintf("%s", net.IP(sessions.LclIP[:4]).To4().String())
			remoteIP = fmt.Sprintf("%s", net.IP(sessions.RmtIP[:4]).To4().String())
		} else {
			localIP = fmt.Sprintf("%s", net.IP(sessions.LclIP).To16().String())
			remoteIP = fmt.Sprintf("%s", net.IP(sessions.RmtIP).To16().String())
		}

		l4Session := &SessionDetails{
			TransportProto: sessions.TransportProto,
			LocalIP:        localIP,
			LocalPort:      sessions.LclPort,
			RemoteIP:       remoteIP,
			RemotePort:     sessions.RmtPort,
			ActionIndex:    sessions.ActionIndex,
			AppnsIndex:     sessions.AppnsIndex,
			Scope:          sessions.Scope,
			Tag:            string(bytes.SplitN(sessions.Tag, []byte{0x00}, 2)[0]),
		}

		appNsDetails = append(appNsDetails, l4Session)
	}

	return appNsDetails, nil
}

func uintToBool(value uint8) bool {
	if value == 0 {
		return false
	}
	return true
}
