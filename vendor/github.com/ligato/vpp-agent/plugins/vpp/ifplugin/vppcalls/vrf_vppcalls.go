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
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
)

// CreateVrf implements interface handler.
func (h *IfVppHandler) CreateVrf(vrfID uint32) error {
	return h.createVrfIfNeeded(vrfID, false)
}

// CreateVrfIPv6 implements interface handler.
func (h *IfVppHandler) CreateVrfIPv6(vrfID uint32) error {
	return h.createVrfIfNeeded(vrfID, true)
}

// SetInterfaceVrf implements interface handler.
func (h *IfVppHandler) SetInterfaceVrf(ifIdx, vrfID uint32) error {
	return h.setInterfaceVrf(ifIdx, vrfID, false)
}

// SetInterfaceVrfIPv6 implements interface handler.
func (h *IfVppHandler) SetInterfaceVrfIPv6(ifIdx, vrfID uint32) error {
	return h.setInterfaceVrf(ifIdx, vrfID, true)
}

// GetInterfaceVrf implements interface handler.
func (h *IfVppHandler) GetInterfaceVrf(ifIdx uint32) (vrfID uint32, err error) {
	return h.getInterfaceVrf(ifIdx, false)
}

// GetInterfaceVrfIPv6 implements interface handler.
func (h *IfVppHandler) GetInterfaceVrfIPv6(ifIdx uint32) (vrfID uint32, err error) {
	return h.getInterfaceVrf(ifIdx, true)
}

// New VRF with provided ID for IPv4 or IPv6 will be created if missing.
func (h *IfVppHandler) createVrfIfNeeded(vrfID uint32, isIPv6 bool) error {
	// Zero VRF exists by default
	if vrfID == 0 {
		return nil
	}

	// Get all VRFs for IPv4 or IPv6
	var exists bool
	if isIPv6 {
		ipv6Tables, err := h.dumpVrfTablesIPv6()
		if err != nil {
			return fmt.Errorf("dumping IPv6 VRF tables failed: %v", err)
		}
		_, exists = ipv6Tables[vrfID]
	} else {
		tables, err := h.dumpVrfTables()
		if err != nil {
			return fmt.Errorf("dumping IPv4 VRF tables failed: %v", err)
		}
		_, exists = tables[vrfID]
	}
	// Create new VRF if needed
	if !exists {
		h.log.Debugf("VRF table %d does not exists and will be created", vrfID)
		return h.vppAddIPTable(vrfID, isIPv6)
	}

	return nil
}

// Interface is set to VRF table. Table IP version has to be defined.
func (h *IfVppHandler) setInterfaceVrf(ifIdx, vrfID uint32, isIPv6 bool) error {
	if err := h.createVrfIfNeeded(vrfID, isIPv6); err != nil {
		return fmt.Errorf("creating VRF failed: %v", err)
	}

	req := &interfaces.SwInterfaceSetTable{
		SwIfIndex: ifIdx,
		VrfID:     vrfID,
		IsIPv6:    boolToUint(isIPv6),
	}
	reply := &interfaces.SwInterfaceSetTableReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	h.log.Debugf("Interface %d set to VRF %d", ifIdx, vrfID)

	return nil
}

// Returns VRF ID for provided interface.
func (h *IfVppHandler) getInterfaceVrf(ifIdx uint32, isIPv6 bool) (vrfID uint32, err error) {
	req := &interfaces.SwInterfaceGetTable{
		SwIfIndex: ifIdx,
		IsIPv6:    boolToUint(isIPv6),
	}
	reply := &interfaces.SwInterfaceGetTableReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, err
	} else if reply.Retval != 0 {
		return 0, fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return reply.VrfID, nil
}

// Returns all IPv4 VRF tables
func (h *IfVppHandler) dumpVrfTables() (map[uint32][]*ip.IPFibDetails, error) {
	fibs := map[uint32][]*ip.IPFibDetails{}
	reqCtx := h.callsChannel.SendMultiRequest(&ip.IPFibDump{})
	for {
		fibDetails := &ip.IPFibDetails{}
		stop, err := reqCtx.ReceiveReply(fibDetails)
		if stop {
			break
		}
		if err != nil {
			return nil, err
		}

		tableID := fibDetails.TableID
		fibs[tableID] = append(fibs[tableID], fibDetails)
	}

	return fibs, nil
}

// Returns all IPv6 VRF tables
func (h *IfVppHandler) dumpVrfTablesIPv6() (map[uint32][]*ip.IP6FibDetails, error) {
	fibs := map[uint32][]*ip.IP6FibDetails{}
	reqCtx := h.callsChannel.SendMultiRequest(&ip.IP6FibDump{})
	for {
		fibDetails := &ip.IP6FibDetails{}
		stop, err := reqCtx.ReceiveReply(fibDetails)
		if stop {
			break
		}
		if err != nil {
			return nil, err
		}

		tableID := fibDetails.TableID
		fibs[tableID] = append(fibs[tableID], fibDetails)
	}

	return fibs, nil
}

// Creates new VRF table with provided ID and for desired IP version
func (h *IfVppHandler) vppAddIPTable(vrfID uint32, isIPv6 bool) error {
	req := &ip.IPTableAddDel{
		TableID: vrfID,
		IsIPv6:  boolToUint(isIPv6),
		IsAdd:   1,
	}
	reply := &ip.IPTableAddDelReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}
