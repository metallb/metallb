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

	"github.com/pkg/errors"

	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
)

// AddInterfaceToBridgeDomain puts interface into bridge domain.
func (h *BridgeDomainVppHandler) AddInterfaceToBridgeDomain(bdIdx uint32, ifaceCfg *l2.BridgeDomain_Interface) error {
	ifaceMeta, found := h.ifIndexes.LookupByName(ifaceCfg.Name)
	if !found {
		return errors.New("failed to get interface metadata")
	}
	if err := h.addDelInterfaceToBridgeDomain(bdIdx, ifaceCfg, ifaceMeta.GetIndex(), true); err != nil {
		return err
	}
	return nil
}

// DeleteInterfaceFromBridgeDomain removes interface from bridge domain.
func (h *BridgeDomainVppHandler) DeleteInterfaceFromBridgeDomain(bdIdx uint32, ifaceCfg *l2.BridgeDomain_Interface) error {
	ifaceMeta, found := h.ifIndexes.LookupByName(ifaceCfg.Name)
	if !found {
		return errors.New("failed to get interface metadata")
	}
	if err := h.addDelInterfaceToBridgeDomain(bdIdx, ifaceCfg, ifaceMeta.GetIndex(), false); err != nil {
		return err
	}
	return nil
}

func (h *BridgeDomainVppHandler) addDelInterfaceToBridgeDomain(bdIdx uint32, ifaceCfg *l2.BridgeDomain_Interface,
	ifIdx uint32, add bool) error {
	req := &l2ba.SwInterfaceSetL2Bridge{
		BdID:        bdIdx,
		RxSwIfIndex: ifIdx,
		Shg:         uint8(ifaceCfg.SplitHorizonGroup),
		Enable:      boolToUint(add),
	}
	// Set as BVI.
	if ifaceCfg.BridgedVirtualInterface {
		req.PortType = l2ba.L2_API_PORT_TYPE_BVI
	}
	reply := &l2ba.SwInterfaceSetL2BridgeReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("%s returned error: %v", reply.GetMessageName(), err)
	}

	return nil
}
