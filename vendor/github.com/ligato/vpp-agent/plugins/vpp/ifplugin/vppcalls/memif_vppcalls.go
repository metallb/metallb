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

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/memif"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
)

// AddMemifInterface implements interface handler.
func (h *IfVppHandler) AddMemifInterface(ifName string, memIface *intf.Interfaces_Interface_Memif, socketID uint32) (swIdx uint32, err error) {
	req := &memif.MemifCreate{
		ID:         memIface.Id,
		Mode:       uint8(memIface.Mode),
		Secret:     []byte(memIface.Secret),
		SocketID:   socketID,
		BufferSize: uint16(memIface.BufferSize),
		RingSize:   memIface.RingSize,
		RxQueues:   uint8(memIface.RxQueues),
		TxQueues:   uint8(memIface.TxQueues),
	}
	if memIface.Master {
		req.Role = 0
	} else {
		req.Role = 1
	}
	// TODO: temporary fix, waiting for https://gerrit.fd.io/r/#/c/7266/
	if req.RxQueues == 0 {
		req.RxQueues = 1
	}
	if req.TxQueues == 0 {
		req.TxQueues = 1
	}
	reply := &memif.MemifCreateReply{}

	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, err
	} else if reply.Retval != 0 {
		return 0, fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return reply.SwIfIndex, h.SetInterfaceTag(ifName, reply.SwIfIndex)
}

// DeleteMemifInterface implements interface handler.
func (h *IfVppHandler) DeleteMemifInterface(ifName string, idx uint32) error {
	req := &memif.MemifDelete{
		SwIfIndex: idx,
	}
	reply := &memif.MemifDeleteReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return h.RemoveInterfaceTag(ifName, idx)
}

// RegisterMemifSocketFilename implements interface handler.
func (h *IfVppHandler) RegisterMemifSocketFilename(filename []byte, id uint32) error {
	req := &memif.MemifSocketFilenameAddDel{
		SocketFilename: filename,
		SocketID:       id,
		IsAdd:          1, // sockets can be added only
	}
	reply := &memif.MemifSocketFilenameAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}
