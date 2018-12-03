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

	acl_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
)

// ACLInterfaceLogicalReq groups multiple fields to not enumerate all of them in one function call
type ACLInterfaceLogicalReq struct {
	aclIndex  uint32
	ifIndices []uint32
	ingress   bool
}

// SetACLToInterfacesAsIngress implements ACL handler.
func (h *ACLVppHandler) SetACLToInterfacesAsIngress(ACLIndex uint32, ifIndices []uint32) error {
	return h.requestSetACLToInterfaces(&ACLInterfaceLogicalReq{
		aclIndex:  ACLIndex,
		ifIndices: ifIndices,
		ingress:   true,
	})
}

// RemoveIPIngressACLFromInterfaces implements ACL handler.
func (h *ACLVppHandler) RemoveIPIngressACLFromInterfaces(ACLIndex uint32, ifIndices []uint32) error {
	return h.requestRemoveInterfacesFromACL(&ACLInterfaceLogicalReq{
		aclIndex:  ACLIndex,
		ifIndices: ifIndices,
		ingress:   true,
	})
}

// SetACLToInterfacesAsEgress implements ACL handler.
func (h *ACLVppHandler) SetACLToInterfacesAsEgress(ACLIndex uint32, ifIndices []uint32) error {
	return h.requestSetACLToInterfaces(&ACLInterfaceLogicalReq{
		aclIndex:  ACLIndex,
		ifIndices: ifIndices,
		ingress:   false,
	})
}

// RemoveIPEgressACLFromInterfaces implements ACL handler.
func (h *ACLVppHandler) RemoveIPEgressACLFromInterfaces(ACLIndex uint32, ifIndices []uint32) error {
	return h.requestRemoveInterfacesFromACL(&ACLInterfaceLogicalReq{
		aclIndex:  ACLIndex,
		ifIndices: ifIndices,
		ingress:   false,
	})
}

// SetMacIPACLToInterface implements ACL handler.
func (h *ACLVppHandler) SetMacIPACLToInterface(aclIndex uint32, ifIndices []uint32) error {
	for _, ingressIfIdx := range ifIndices {
		req := &acl_api.MacipACLInterfaceAddDel{
			ACLIndex:  aclIndex,
			IsAdd:     1,
			SwIfIndex: ingressIfIdx,
		}

		reply := &acl_api.MacipACLInterfaceAddDelReply{}

		err := h.callsChannel.SendRequest(req).ReceiveReply(reply)
		if err != nil {
			return fmt.Errorf("failed to set interface %d to L2 ACL %d: %v", ingressIfIdx, aclIndex, err)
		}
		if reply.Retval != 0 {
			return fmt.Errorf("set interface %d to L2 ACL %d returned %d", ingressIfIdx, aclIndex, reply.Retval)
		}
	}

	return nil
}

// RemoveMacIPIngressACLFromInterfaces implements ACL handler.
func (h *ACLVppHandler) RemoveMacIPIngressACLFromInterfaces(removedACLIndex uint32, ifIndices []uint32) error {
	for _, ifIdx := range ifIndices {
		req := &acl_api.MacipACLInterfaceAddDel{
			ACLIndex:  removedACLIndex,
			SwIfIndex: ifIdx,
			IsAdd:     0,
		}

		reply := &acl_api.MacipACLInterfaceAddDelReply{}

		if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
			return fmt.Errorf("failed to remove L2 ACL %d from interface %d: %v", removedACLIndex, ifIdx, err)
		}
		if reply.Retval != 0 {
			return fmt.Errorf("remove L2 ACL %d from interface %d returned error %d", removedACLIndex,
				removedACLIndex, reply.Retval)
		}
	}
	return nil
}

func (h *ACLVppHandler) requestSetACLToInterfaces(logicalReq *ACLInterfaceLogicalReq) error {
	for _, aclIfIdx := range logicalReq.ifIndices {
		// Create acl list with new entry
		var ACLs []uint32

		// All previously assigned ACLs have to be dumped and added to acl list
		aclInterfaceDetails, err := h.DumpInterfaceIPACLs(aclIfIdx)
		if err != nil {
			return err
		}

		var nInput uint8
		if aclInterfaceDetails != nil {
			nInput = aclInterfaceDetails.NInput
			if logicalReq.ingress {
				// Construct ACL list. ACLs within NInput are defined as ingress, so provided new aclIndex has to be
				// added to the beginning of the list
				// TODO it would be nicer to add new acl index to newNInput index
				ACLs = append(ACLs, logicalReq.aclIndex)
				for _, aclIndex := range aclInterfaceDetails.Acls {
					ACLs = append(ACLs, aclIndex)
				}
				nInput++ // Rise NInput
			} else {
				// Construct ACL list. ACLs outside of NInput are defined as egress, so provided new aclIndex has to be
				// added to the end of the list
				for _, aclIndex := range aclInterfaceDetails.Acls {
					ACLs = append(ACLs, aclIndex)
				}
				ACLs = append(ACLs, logicalReq.aclIndex)
				// NInput remains the same
			}
		}

		msg := &acl_api.ACLInterfaceSetACLList{
			Acls:      ACLs,
			Count:     uint8(len(ACLs)),
			SwIfIndex: aclIfIdx,
			NInput:    nInput,
		}
		reply := &acl_api.ACLInterfaceSetACLListReply{}

		err = h.callsChannel.SendRequest(msg).ReceiveReply(reply)
		if err != nil {
			return err
		}
		if reply.Retval != 0 {
			return fmt.Errorf("setting up interface ACL list returned %v", reply.Retval)
		}
	}

	return nil
}

func (h *ACLVppHandler) requestRemoveInterfacesFromACL(logicalReq *ACLInterfaceLogicalReq) error {
	var wasErr error
	for _, aclIfIdx := range logicalReq.ifIndices {
		// Create empty ACL list
		var ACLs []uint32

		// All assigned ACLs have to be dumped
		aclInterfaceDetails, err := h.DumpInterfaceIPACLs(aclIfIdx)
		if err != nil {
			return err
		}

		// Reconstruct ACL list without removed ACL
		var nInput uint8
		if aclInterfaceDetails != nil {
			nInput = aclInterfaceDetails.NInput
			for idx, aclIndex := range aclInterfaceDetails.Acls {
				if (aclIndex != logicalReq.aclIndex) ||
					(logicalReq.ingress && idx >= int(aclInterfaceDetails.NInput)) ||
					(!logicalReq.ingress && idx < int(aclInterfaceDetails.NInput)) {
					ACLs = append(ACLs, aclIndex)
				} else {
					// Decrease NInput if ingress, otherwise keep it the same
					if logicalReq.ingress {
						nInput--
					}
				}
			}
		}

		msg := &acl_api.ACLInterfaceSetACLList{
			Acls:      ACLs,
			Count:     uint8(len(ACLs)),
			SwIfIndex: aclIfIdx,
			NInput:    nInput,
		}

		reply := &acl_api.ACLInterfaceSetACLListReply{}
		err = h.callsChannel.SendRequest(msg).ReceiveReply(reply)
		if err != nil {
			wasErr = err
		}
		if reply.Retval != 0 {
			wasErr = fmt.Errorf("setting up interface ACL list returned %v", reply.Retval)
		}
	}

	return wasErr
}
