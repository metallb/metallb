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
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
)

// ArpEntry represents ARP entry for interface
type ArpEntry struct {
	Interface  uint32
	IPAddress  net.IP
	MacAddress string
	Static     bool
}

// vppAddDelArp adds or removes ARP entry according to provided input
func (h *ArpVppHandler) vppAddDelArp(entry *ArpEntry, delete bool) error {
	req := &ip.IPNeighborAddDel{
		SwIfIndex:  entry.Interface,
		IsNoAdjFib: 1,
	}
	if delete {
		req.IsAdd = 0
	} else {
		req.IsAdd = 1
	}

	isIpv6, err := addrs.IsIPv6(entry.IPAddress.String())
	if err != nil {
		return err
	}
	if isIpv6 {
		req.IsIPv6 = 1
		req.DstAddress = []byte(entry.IPAddress.To16())
	} else {
		req.IsIPv6 = 0
		req.DstAddress = []byte(entry.IPAddress.To4())
	}
	if entry.Static {
		req.IsStatic = 1
	} else {
		req.IsStatic = 0
	}
	macAddr, err := net.ParseMAC(entry.MacAddress)
	if err != nil {
		return err
	}
	req.MacAddress = []byte(macAddr)
	reply := &ip.IPNeighborAddDelReply{}

	// Send message
	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// VppAddArp implements arp handler.
func (h *ArpVppHandler) VppAddArp(entry *ArpEntry) error {
	return h.vppAddDelArp(entry, false)
}

// VppDelArp implements arp handler.
func (h *ArpVppHandler) VppDelArp(entry *ArpEntry) error {
	return h.vppAddDelArp(entry, true)
}
