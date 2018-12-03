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
	"fmt"
	"net"

	stnapi "github.com/ligato/vpp-agent/plugins/vpp/binapi/stn"
	"github.com/ligato/vpp-agent/plugins/vpp/model/stn"
)

// StnDetails contains proto-modelled STN data and vpp specific metadata
type StnDetails struct {
	Rules []*stn.STN_Rule
	Meta  *StnMeta
}

// StnMeta contains map of interface name/index
type StnMeta struct {
	IfNameToIdx map[uint32]string
}

// DumpStnRules implements STN handler.
func (h *StnVppHandler) DumpStnRules() (rules *StnDetails, err error) {
	var ruleList []*stn.STN_Rule
	meta := &StnMeta{
		IfNameToIdx: make(map[uint32]string),
	}

	req := &stnapi.StnRulesDump{}
	reqCtx := h.callsChannel.SendMultiRequest(req)
	for {
		msg := &stnapi.StnRulesDetails{}
		stop, err := reqCtx.ReceiveReply(msg)
		if stop {
			break
		}
		if err != nil {
			return nil, err
		}
		ifName, _, found := h.ifIndexes.LookupName(msg.SwIfIndex)
		if !found {
			h.log.Warnf("STN dump: name not found for interface %d", msg.SwIfIndex)
		}

		var stnStrIP string
		if uintToBool(msg.IsIP4) {
			stnStrIP = fmt.Sprintf("%s", net.IP(msg.IPAddress[:4]).To4().String())
		} else {
			stnStrIP = fmt.Sprintf("%s", net.IP(msg.IPAddress).To16().String())
		}

		ruleList = append(ruleList, &stn.STN_Rule{
			IpAddress: stnStrIP,
			Interface: ifName,
		})
		meta.IfNameToIdx[msg.SwIfIndex] = ifName
	}

	return &StnDetails{
		Rules: ruleList,
		Meta:  meta,
	}, nil
}
