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
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
)

const (
	addInterfaceIP uint8 = 1
	delInterfaceIP uint8 = 0
)

func (h *IfVppHandler) addDelInterfaceIP(ifIdx uint32, addr *net.IPNet, isAdd uint8) error {
	req := &interfaces.SwInterfaceAddDelAddress{
		SwIfIndex: ifIdx,
		IsAdd:     isAdd,
	}

	prefix, _ := addr.Mask.Size()
	req.AddressLength = byte(prefix)

	isIPv6, err := addrs.IsIPv6(addr.IP.String())
	if err != nil {
		return err
	}
	if isIPv6 {
		req.Address = []byte(addr.IP.To16())
		req.IsIPv6 = 1
	} else {
		req.Address = []byte(addr.IP.To4())
		req.IsIPv6 = 0
	}
	reply := &interfaces.SwInterfaceAddDelAddressReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

// AddInterfaceIP implements interface handler.
func (h *IfVppHandler) AddInterfaceIP(ifIdx uint32, addr *net.IPNet) error {
	return h.addDelInterfaceIP(ifIdx, addr, addInterfaceIP)
}

// DelInterfaceIP implements interface handler.
func (h *IfVppHandler) DelInterfaceIP(ifIdx uint32, addr *net.IPNet) error {
	return h.addDelInterfaceIP(ifIdx, addr, delInterfaceIP)
}

const (
	setUnnumberedIP   uint8 = 1
	unsetUnnumberedIP uint8 = 0
)

func (h *IfVppHandler) setUnsetUnnumberedIP(uIfIdx uint32, ifIdxWithIP uint32, isAdd uint8) error {
	// Prepare the message.
	req := &interfaces.SwInterfaceSetUnnumbered{
		SwIfIndex:           ifIdxWithIP,
		UnnumberedSwIfIndex: uIfIdx,
		IsAdd:               isAdd,
	}
	reply := &interfaces.SwInterfaceSetUnnumberedReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

// SetUnnumberedIP implements interface handler.
func (h *IfVppHandler) SetUnnumberedIP(uIfIdx uint32, ifIdxWithIP uint32) error {
	return h.setUnsetUnnumberedIP(uIfIdx, ifIdxWithIP, setUnnumberedIP)
}

// UnsetUnnumberedIP implements interface handler.
func (h *IfVppHandler) UnsetUnnumberedIP(uIfIdx uint32) error {
	return h.setUnsetUnnumberedIP(uIfIdx, 0, unsetUnnumberedIP)
}
