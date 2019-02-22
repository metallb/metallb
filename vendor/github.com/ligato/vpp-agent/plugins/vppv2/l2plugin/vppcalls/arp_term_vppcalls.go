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
	"net"

	"github.com/ligato/cn-infra/utils/addrs"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
)

func (h *BridgeDomainVppHandler) callBdIPMacAddDel(isAdd bool, bdID uint32, mac string, ip string) error {
	req := &l2ba.BdIPMacAddDel{
		BdID:  bdID,
		IsAdd: boolToUint(isAdd),
	}

	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return err
	}
	req.MacAddress = macAddr

	isIpv6, err := addrs.IsIPv6(ip)
	if err != nil {
		return err
	}
	ipAddr := net.ParseIP(ip)
	if isIpv6 {
		req.IsIPv6 = 1
		req.IPAddress = []byte(ipAddr.To16())
	} else {
		req.IsIPv6 = 0
		req.IPAddress = []byte(ipAddr.To4())
	}
	reply := &l2ba.BdIPMacAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

// AddArpTerminationTableEntry creates ARP termination entry for bridge domain.
func (h *BridgeDomainVppHandler) AddArpTerminationTableEntry(bdID uint32, mac string, ip string) error {
	err := h.callBdIPMacAddDel(true, bdID, mac, ip)
	if err != nil {
		return err
	}
	return nil
}

// RemoveArpTerminationTableEntry removes ARP termination entry from bridge domain.
func (h *BridgeDomainVppHandler) RemoveArpTerminationTableEntry(bdID uint32, mac string, ip string) error {
	err := h.callBdIPMacAddDel(false, bdID, mac, ip)
	if err != nil {
		return err
	}
	return nil
}
