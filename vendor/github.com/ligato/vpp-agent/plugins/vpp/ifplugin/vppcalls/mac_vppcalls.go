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

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
)

// SetInterfaceMac implements interface handler.
func (h *IfVppHandler) SetInterfaceMac(ifIdx uint32, macAddress string) error {
	mac, err := net.ParseMAC(macAddress)
	if err != nil {
		return err
	}

	req := &interfaces.SwInterfaceSetMacAddress{
		SwIfIndex:  ifIdx,
		MacAddress: mac,
	}
	reply := &interfaces.SwInterfaceSetMacAddressReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}
