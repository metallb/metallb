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
	"errors"
	"net"

	l2nb "github.com/ligato/vpp-agent/api/models/vpp/l2"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
)

// AddL2FIB creates L2 FIB table entry.
func (h *FIBVppHandler) AddL2FIB(fib *l2nb.FIBEntry) error {
	return h.l2fibAddDel(fib, true)
}

// DeleteL2FIB removes existing L2 FIB table entry.
func (h *FIBVppHandler) DeleteL2FIB(fib *l2nb.FIBEntry) error {
	return h.l2fibAddDel(fib, false)
}

func (h *FIBVppHandler) l2fibAddDel(fib *l2nb.FIBEntry, isAdd bool) (err error) {
	// get bridge domain metadata
	bdMeta, found := h.bdIndexes.LookupByName(fib.BridgeDomain)
	if !found {
		return errors.New("failed to get bridge domain metadata")
	}

	// get outgoing interface index
	swIfIndex := ^uint32(0) // ~0 is used by DROP entries
	if fib.Action == l2nb.FIBEntry_FORWARD {
		ifaceMeta, found := h.ifIndexes.LookupByName(fib.OutgoingInterface)
		if !found {
			return errors.New("failed to get interface metadata")
		}
		swIfIndex = ifaceMeta.GetIndex()
	}

	// parse MAC address
	var mac []byte
	if fib.PhysAddress != "" {
		mac, err = net.ParseMAC(fib.PhysAddress)
		if err != nil {
			return err
		}
	}

	// add L2 FIB
	req := &l2ba.L2fibAddDel{
		IsAdd:     boolToUint(isAdd),
		Mac:       mac,
		BdID:      bdMeta.GetIndex(),
		SwIfIndex: swIfIndex,
		BviMac:    boolToUint(fib.BridgedVirtualInterface),
		StaticMac: boolToUint(fib.StaticConfig),
		FilterMac: boolToUint(fib.Action == l2nb.FIBEntry_DROP),
	}
	reply := &l2ba.L2fibAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}
