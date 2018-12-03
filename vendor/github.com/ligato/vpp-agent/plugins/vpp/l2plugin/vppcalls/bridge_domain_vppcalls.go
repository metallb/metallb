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

	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// VppAddBridgeDomain implements bridge domain handler.
func (h *BridgeDomainVppHandler) VppAddBridgeDomain(bdIdx uint32, bd *l2.BridgeDomains_BridgeDomain) error {
	req := &l2ba.BridgeDomainAddDel{
		IsAdd:   1,
		BdID:    bdIdx,
		Learn:   boolToUint(bd.Learn),
		ArpTerm: boolToUint(bd.ArpTermination),
		Flood:   boolToUint(bd.Flood),
		UuFlood: boolToUint(bd.UnknownUnicastFlood),
		Forward: boolToUint(bd.Forward),
		MacAge:  uint8(bd.MacAge),
		BdTag:   []byte(bd.Name),
	}
	reply := &l2ba.BridgeDomainAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

// VppDeleteBridgeDomain implements bridge domain handler.
func (h *BridgeDomainVppHandler) VppDeleteBridgeDomain(bdIdx uint32) error {
	req := &l2ba.BridgeDomainAddDel{
		IsAdd: 0,
		BdID:  bdIdx,
	}
	reply := &l2ba.BridgeDomainAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}

func boolToUint(value bool) uint8 {
	if value {
		return 1
	}
	return 0
}
