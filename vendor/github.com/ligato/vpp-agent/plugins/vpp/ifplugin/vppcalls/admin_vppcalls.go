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

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
)

// InterfaceAdminDown implements interface handler.
func (h *IfVppHandler) InterfaceAdminDown(ifIdx uint32) error {
	return h.interfaceSetFlags(ifIdx, false)
}

// InterfaceAdminUp implements interface handler.
func (h *IfVppHandler) InterfaceAdminUp(ifIdx uint32) error {
	return h.interfaceSetFlags(ifIdx, true)
}

// SetInterfaceTag implements interface handler.
func (h *IfVppHandler) SetInterfaceTag(tag string, ifIdx uint32) error {
	return h.handleInterfaceTag(tag, ifIdx, true)
}

// RemoveInterfaceTag implements interface handler.
func (h *IfVppHandler) RemoveInterfaceTag(tag string, ifIdx uint32) error {
	return h.handleInterfaceTag(tag, ifIdx, false)
}

func (h *IfVppHandler) interfaceSetFlags(ifIdx uint32, adminUp bool) error {
	req := &interfaces.SwInterfaceSetFlags{
		SwIfIndex:   ifIdx,
		AdminUpDown: boolToUint(adminUp),
	}
	reply := &interfaces.SwInterfaceSetFlagsReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

func (h *IfVppHandler) handleInterfaceTag(tag string, ifIdx uint32, isAdd bool) error {
	req := &interfaces.SwInterfaceTagAddDel{
		Tag:   []byte(tag),
		IsAdd: boolToUint(isAdd),
	}
	// For some reason, if deleting tag, the software interface index has to be 0 and only name should be set.
	// Otherwise reply returns with error core -2 (incorrect sw_if_idx)
	if isAdd {
		req.SwIfIndex = ifIdx
	}
	reply := &interfaces.SwInterfaceTagAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s %v (index %v) add/del returned %v", reply.GetMessageName(), tag, ifIdx, reply.Retval)
	}

	return nil
}

func boolToUint(input bool) uint8 {
	if input {
		return 1
	}
	return 0
}
