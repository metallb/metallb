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

	"github.com/go-errors/errors"

	"github.com/ligato/cn-infra/logging"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// SetInterfaceToBridgeDomain implements bridge domain handler. Returns an interface configured to the BD.
func (h *BridgeDomainVppHandler) SetInterfaceToBridgeDomain(bdName string, bdIdx uint32, bdIf *l2.BridgeDomains_BridgeDomain_Interfaces,
	swIfIndices ifaceidx.SwIfIndex) (string, error) {
	// Verify that interface exists, otherwise skip it.
	ifIdx, _, found := swIfIndices.LookupIdx(bdIf.Name)
	if !found {
		h.log.Debugf("Required bridge domain %s interface %s not found", bdName, bdIf.Name)
		return "", nil
	}
	if err := h.addDelInterfaceToBridgeDomain(bdName, bdIdx, bdIf, ifIdx, true); err != nil {
		return "", err
	}
	h.log.WithFields(logging.Fields{"Interface": bdIf.Name, "BD": bdName}).Debug("Interface set to bridge domain")

	return bdIf.Name, nil
}

// SetInterfacesToBridgeDomain implements bridge domain handler. Returns a list of interfaces configured to the BD.
func (h *BridgeDomainVppHandler) SetInterfacesToBridgeDomain(bdName string, bdIdx uint32, bdIfs []*l2.BridgeDomains_BridgeDomain_Interfaces,
	swIfIndices ifaceidx.SwIfIndex) ([]string, error) {

	var ifs []string
	for _, bdIf := range bdIfs {
		iface, err := h.SetInterfaceToBridgeDomain(bdName, bdIdx, bdIf, swIfIndices)
		if err != nil {
			return nil, err
		}
		if iface != "" {
			ifs = append(ifs, iface)
		}
	}

	return ifs, nil
}

// UnsetInterfacesFromBridgeDomain implements bridge domain handler. Returns a list of interfaces removed from the BD.
func (h *BridgeDomainVppHandler) UnsetInterfacesFromBridgeDomain(bdName string, bdIdx uint32, bdIfs []*l2.BridgeDomains_BridgeDomain_Interfaces,
	swIfIndices ifaceidx.SwIfIndex) (ifs []string, wasErr error) {
	if len(bdIfs) == 0 {
		h.log.Debugf("Bridge domain %s has no obsolete interface to unset", bdName)
		return nil, nil
	}

	for _, bdIf := range bdIfs {
		// Verify that interface exists, otherwise skip it.
		ifIdx, _, found := swIfIndices.LookupIdx(bdIf.Name)
		if !found {
			h.log.Debugf("Required bridge domain %s interface %s not found", bdName, bdIf.Name)
			// The interface still needs to be added to the list as un-configured
			ifs = append(ifs, bdIf.Name)
			continue
		}
		if err := h.addDelInterfaceToBridgeDomain(bdName, bdIdx, bdIf, ifIdx, false); err != nil {
			return nil, errors.Errorf("failed to remove interface %s from bridge domain %s: %v", bdIf.Name, bdName, err)
		}
		h.log.WithFields(logging.Fields{"Interface": bdIf.Name, "BD": bdName}).Debug("Interface unset from bridge domain")
		ifs = append(ifs, bdIf.Name)
	}

	return ifs, wasErr
}

func (h *BridgeDomainVppHandler) addDelInterfaceToBridgeDomain(bdName string, bdIdx uint32, bdIf *l2.BridgeDomains_BridgeDomain_Interfaces,
	ifIdx uint32, add bool) error {
	req := &l2ba.SwInterfaceSetL2Bridge{
		BdID:        bdIdx,
		RxSwIfIndex: ifIdx,
		Shg:         uint8(bdIf.SplitHorizonGroup),
		Enable:      boolToUint(add),
	}
	// Set as BVI.
	if bdIf.BridgedVirtualInterface {
		req.PortType = l2ba.L2_API_PORT_TYPE_BVI
		h.log.Debugf("Interface %v set as BVI", bdIf.Name)
	}
	reply := &l2ba.SwInterfaceSetL2BridgeReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("error while assigning/removing interface %s to bd %s: %v", bdIf.Name, bdName, err)
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d while assigning/removing interface %s (idx %d) to bd %s",
			reply.GetMessageName(), reply.Retval, bdIf.Name, ifIdx, bdName)
	}

	return nil
}
