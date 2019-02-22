// Copyright (c) 2018 Cisco and/or its affiliates.
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
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vmxnet3"
)

// AddVmxNet3 implements interface handler
func (h *IfVppHandler) AddVmxNet3(ifName string, vmxNet3 *interfaces.VmxNet3Link) (swIdx uint32, err error) {
	var pci uint32
	pci, err = derivePCI(ifName)
	if err != nil {
		return 0, err
	}

	req := &vmxnet3.Vmxnet3Create{
		PciAddr: pci,
	}
	// Optional arguments
	if vmxNet3 != nil {
		req.EnableElog = int32(boolToUint(vmxNet3.EnableElog))
		req.RxqSize = uint16(vmxNet3.RxqSize)
		req.TxqSize = uint16(vmxNet3.TxqSize)
	}

	reply := &vmxnet3.Vmxnet3CreateReply{}
	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, errors.Errorf(err.Error())
	} else if reply.Retval != 0 {
		return 0, errors.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return reply.SwIfIndex, h.SetInterfaceTag(ifName, reply.SwIfIndex)
}

// DeleteVmxNet3 implements interface handler
func (h *IfVppHandler) DeleteVmxNet3(ifName string, ifIdx uint32) error {
	req := &vmxnet3.Vmxnet3Delete{
		SwIfIndex: ifIdx,
	}
	reply := &vmxnet3.Vmxnet3DeleteReply{}
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return errors.Errorf(err.Error())
	} else if reply.Retval != 0 {
		return errors.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return h.RemoveInterfaceTag(ifName, ifIdx)
}

func derivePCI(ifName string) (uint32, error) {
	var function, slot, bus, domain, pci uint32

	numLen, err := fmt.Sscanf(ifName, "vmxnet3-%x/%x/%x/%x", &domain, &bus, &slot, &function)
	if err != nil {
		err = errors.Errorf("cannot parse PCI address from the vmxnet3 interface name %s: %v", ifName, err)
		return 0, err
	}
	if numLen != 4 {
		err = errors.Errorf("cannot parse PCI address from the interface name %s: expected 4 address elements, received %d",
			ifName, numLen)
		return 0, err
	}

	pci |= function << 29
	pci |= slot << 24
	pci |= bus << 16
	pci |= domain

	return pci, nil
}
