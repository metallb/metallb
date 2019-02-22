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

	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
)

// AddL2XConnect creates xConnect between two existing interfaces.
func (h *XConnectVppHandler) AddL2XConnect(rxIface, txIface string) error {
	return h.addDelXConnect(rxIface, txIface, true)
}

// DeleteL2XConnect removes xConnect between two interfaces.
func (h *XConnectVppHandler) DeleteL2XConnect(rxIface, txIface string) error {
	return h.addDelXConnect(rxIface, txIface, false)
}

func (h *XConnectVppHandler) addDelXConnect(rxIface, txIface string, enable bool) error {
	// get Rx interface metadata
	rxIfaceMeta, found := h.ifIndexes.LookupByName(rxIface)
	if !found {
		return errors.New("failed to get Rx interface metadata")
	}

	// get Tx interface metadata
	txIfaceMeta, found := h.ifIndexes.LookupByName(txIface)
	if !found {
		return errors.New("failed to get Tx interface metadata")
	}

	// add/del xConnect pair
	req := &l2ba.SwInterfaceSetL2Xconnect{
		Enable:      boolToUint(enable),
		TxSwIfIndex: txIfaceMeta.GetIndex(),
		RxSwIfIndex: rxIfaceMeta.GetIndex(),
	}
	reply := &l2ba.SwInterfaceSetL2XconnectReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}
