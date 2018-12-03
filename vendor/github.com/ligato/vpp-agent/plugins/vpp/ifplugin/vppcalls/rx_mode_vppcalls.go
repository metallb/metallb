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
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
)

// SetRxMode implements interface handler.
func (h *IfVppHandler) SetRxMode(ifIdx uint32, rxModeSettings *intf.Interfaces_Interface_RxModeSettings) error {
	req := &interfaces.SwInterfaceSetRxMode{
		SwIfIndex:    ifIdx,
		Mode:         uint8(rxModeSettings.RxMode),
		QueueID:      rxModeSettings.QueueId,
		QueueIDValid: uint8(rxModeSettings.QueueIdValid),
	}
	reply := &interfaces.SwInterfaceSetRxModeReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}
