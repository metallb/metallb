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

	stn "github.com/ligato/vpp-agent/api/models/vpp/stn"
	"github.com/pkg/errors"

	api "github.com/ligato/vpp-agent/plugins/vpp/binapi/stn"
	"strings"
)

// AddSTNRule implements STN handler, adds a new STN rule to the VPP.
func (h *StnVppHandler) AddSTNRule(stnRule *stn.Rule) error {
	return h.addDelStnRule(stnRule, true)
}

// DeleteSTNRule implements STN handler, removes the provided STN rule from the VPP.
func (h *StnVppHandler) DeleteSTNRule(stnRule *stn.Rule) error {
	return h.addDelStnRule(stnRule, false)
}

func (h *StnVppHandler) addDelStnRule(stnRule *stn.Rule, isAdd bool) error {
	// get interface index
	ifaceMeta, found := h.ifIndexes.LookupByName(stnRule.Interface)
	if !found {
		return errors.New("failed to get interface metadata")
	}
	swIfIndex := ifaceMeta.GetIndex()

	// remove mask from IP address if necessary
	ipAddr := stnRule.IpAddress
	ipParts := strings.Split(ipAddr, "/")
	if len(ipParts) > 1 {
		h.log.Debugf("STN IP address %s is defined with mask, removing it")
		ipAddr = ipParts[0]
	}

	// parse IP address
	var byteIP []byte
	var isIPv4 uint8
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return errors.Errorf("failed to parse IP address %s", ipAddr)
	} else if ip.To4() == nil {
		byteIP = []byte(ip.To16())
		isIPv4 = 0
	} else {
		byteIP = []byte(ip.To4())
		isIPv4 = 1
	}

	// add STN rule
	req := &api.StnAddDelRule{
		IsIP4:     isIPv4,
		IPAddress: byteIP,
		SwIfIndex: swIfIndex,
		IsAdd:     boolToUint(isAdd),
	}
	reply := &api.StnAddDelRuleReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil

}

func boolToUint(input bool) uint8 {
	if input {
		return 1
	}
	return 0
}
