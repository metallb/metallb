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
)

// AddL2XConnect implements xconnect handler.
func (h *XConnectVppHandler) AddL2XConnect(rxIfIdx uint32, txIfIdx uint32) error {
	return h.addDelXConnect(rxIfIdx, txIfIdx, true)
}

// DeleteL2XConnect implements xconnect handler.
func (h *XConnectVppHandler) DeleteL2XConnect(rxIfIdx uint32, txIfIdx uint32) error {
	return h.addDelXConnect(rxIfIdx, txIfIdx, false)
}

func (h *XConnectVppHandler) addDelXConnect(rxIfaceIdx uint32, txIfaceIdx uint32, enable bool) error {
	req := &l2ba.SwInterfaceSetL2Xconnect{
		Enable:      boolToUint(enable),
		TxSwIfIndex: txIfaceIdx,
		RxSwIfIndex: rxIfaceIdx,
	}
	reply := &l2ba.SwInterfaceSetL2XconnectReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}
