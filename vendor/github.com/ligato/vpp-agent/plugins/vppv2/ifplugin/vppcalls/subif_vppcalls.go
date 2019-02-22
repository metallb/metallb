//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package vppcalls

import (
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
)

// CreateSubif creates sub interface.
func (h *IfVppHandler) CreateSubif(ifIdx, vlanID uint32) (uint32, error) {
	req := &interfaces.CreateVlanSubif{
		SwIfIndex: ifIdx,
		VlanID:    vlanID,
	}

	reply := &interfaces.CreateVlanSubifReply{}
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, err
	}

	return reply.SwIfIndex, nil
}

// DeleteSubif deletes sub interface.
func (h *IfVppHandler) DeleteSubif(ifIdx uint32) error {
	req := &interfaces.DeleteSubif{
		SwIfIndex: ifIdx,
	}

	reply := &interfaces.DeleteSubifReply{}
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}
