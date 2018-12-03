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
	govppapi "git.fd.io/govpp.git/api"
	aclapi "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
)

// ACLVppAPI provides read/write methods required to handle VPP access lists
type ACLVppAPI interface {
	ACLVppWrite
	ACLVppRead
}

// ACLVppWrite provides write methods for ACL plugin
type ACLVppWrite interface {
	// AddIPACL create new L3/4 ACL. Input index == 0xffffffff, VPP provides index in reply.
	AddIPACL(rules []*acl.AccessLists_Acl_Rule, aclName string) (uint32, error)
	// AddMacIPACL creates new L2 MAC IP ACL. VPP provides index in reply.
	AddMacIPACL(rules []*acl.AccessLists_Acl_Rule, aclName string) (uint32, error)
	// ModifyIPACL uses index (provided by VPP) to identify ACL which is modified.
	ModifyIPACL(aclIndex uint32, rules []*acl.AccessLists_Acl_Rule, aclName string) error
	// ModifyMACIPACL uses index (provided by VPP) to identify ACL which is modified.
	ModifyMACIPACL(aclIndex uint32, rules []*acl.AccessLists_Acl_Rule, aclName string) error
	// DeleteIPACL removes L3/L4 ACL.
	DeleteIPACL(aclIndex uint32) error
	// DeleteMacIPACL removes L2 ACL.
	DeleteMacIPACL(aclIndex uint32) error
	// SetACLToInterfacesAsIngress sets ACL to all provided interfaces as ingress
	SetACLToInterfacesAsIngress(ACLIndex uint32, ifIndices []uint32) error
	// RemoveIPIngressACLFromInterfaces removes ACL from interfaces
	RemoveIPIngressACLFromInterfaces(ACLIndex uint32, ifIndices []uint32) error
	// SetACLToInterfacesAsEgress sets ACL to all provided interfaces as egress
	SetACLToInterfacesAsEgress(ACLIndex uint32, ifIndices []uint32) error
	// RemoveIPEgressACLFromInterfaces removes ACL from interfaces
	RemoveIPEgressACLFromInterfaces(ACLIndex uint32, ifIndices []uint32) error
	// SetMacIPACLToInterface adds L2 ACL to interface.
	SetMacIPACLToInterface(aclIndex uint32, ifIndices []uint32) error
	// RemoveMacIPIngressACLFromInterfaces removes L2 ACL from interfaces.
	RemoveMacIPIngressACLFromInterfaces(removedACLIndex uint32, ifIndices []uint32) error
}

// ACLVppRead provides read methods for ACL plugin
type ACLVppRead interface {
	// DumpIPACL returns all IP-type ACLs
	DumpIPACL(swIfIndices ifaceidx.SwIfIndex) ([]*ACLDetails, error)
	// DumpIPACL returns all MACIP-type ACLs
	DumpMACIPACL(swIfIndices ifaceidx.SwIfIndex) ([]*ACLDetails, error)
	// DumpACLInterfaces returns a map of IP ACL indices with interfaces
	DumpIPACLInterfaces(indices []uint32, swIfIndices ifaceidx.SwIfIndex) (map[uint32]*acl.AccessLists_Acl_Interfaces, error)
	// DumpMACIPACLInterfaces returns a map of MACIP ACL indices with interfaces
	DumpMACIPACLInterfaces(indices []uint32, swIfIndices ifaceidx.SwIfIndex) (map[uint32]*acl.AccessLists_Acl_Interfaces, error)
	// DumpIPAcls returns a list of all configured ACLs with IP-type ruleData.
	DumpIPAcls() (map[ACLMeta][]aclapi.ACLRule, error)
	// DumpMacIPAcls returns a list of all configured ACL with IPMAC-type ruleData.
	DumpMacIPAcls() (map[ACLMeta][]aclapi.MacipACLRule, error)
	// DumpInterfaceAcls finds interface in VPP and returns its ACL configuration
	DumpInterfaceIPAcls(swIndex uint32) (acl.AccessLists, error)
	// DumpInterfaceMACIPAcls finds interface in VPP and returns its MACIP ACL configuration
	DumpInterfaceMACIPAcls(swIndex uint32) (acl.AccessLists, error)
	// DumpInterfaceIPACLs finds interface in VPP and returns its IP ACL configuration.
	DumpInterfaceIPACLs(swIndex uint32) (*aclapi.ACLInterfaceListDetails, error)
	// DumpInterfaceMACIPACLs finds interface in VPP and returns its MACIP ACL configuration.
	DumpInterfaceMACIPACLs(swIndex uint32) (*aclapi.MacipACLInterfaceListDetails, error)
	// DumpInterfaces finds  all interfaces in VPP and returns their ACL configurations
	DumpInterfaces() ([]*aclapi.ACLInterfaceListDetails, []*aclapi.MacipACLInterfaceListDetails, error)
}

// ACLVppHandler is accessor for acl-related vppcalls methods
type ACLVppHandler struct {
	callsChannel govppapi.Channel
	dumpChannel  govppapi.Channel
}

// NewACLVppHandler creates new instance of acl vppcalls handler
func NewACLVppHandler(callsChan, dumpChan govppapi.Channel) *ACLVppHandler {
	return &ACLVppHandler{
		callsChannel: callsChan,
		dumpChannel:  dumpChan,
	}
}
