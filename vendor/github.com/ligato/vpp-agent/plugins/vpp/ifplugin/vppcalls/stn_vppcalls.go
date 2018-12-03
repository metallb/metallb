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

package vppcalls

import (
	"fmt"
	"net"

	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/stn"
)

// StnRule represents stn rule entry
type StnRule struct {
	IPAddress net.IP
	IfaceIdx  uint32
}

func (h *StnVppHandler) addDelStnRule(ifIdx uint32, addr *net.IP, isAdd bool) error {
	// prepare the message
	req := &stn.StnAddDelRule{
		SwIfIndex: ifIdx,
		IsAdd:     boolToUint(isAdd),
	}

	isIPv6, err := addrs.IsIPv6(addr.String())
	if err != nil {
		return err
	}
	if isIPv6 {
		req.IPAddress = []byte(addr.To16())
		req.IsIP4 = 0
	} else {
		req.IPAddress = []byte(addr.To4())
		req.IsIP4 = 1
	}
	reply := &stn.StnAddDelRuleReply{}

	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil

}

// AddStnRule implements STN handler.
func (h *StnVppHandler) AddStnRule(ifIdx uint32, addr *net.IP) error {
	return h.addDelStnRule(ifIdx, addr, true)

}

// DelStnRule implements STN handler.
func (h *StnVppHandler) DelStnRule(ifIdx uint32, addr *net.IP) error {
	return h.addDelStnRule(ifIdx, addr, false)
}
