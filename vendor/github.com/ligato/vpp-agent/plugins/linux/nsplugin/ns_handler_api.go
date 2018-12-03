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

package nsplugin

import (
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
	"github.com/vishvananda/netns"
)

// NamespaceAPI defines all methods required for managing namespaces and microservices
type NamespaceAPI interface {
	NsManagement
	NsConvertor
	Microservices
}

// NsManagement defines methods to manage namespaces
type NsManagement interface {
	// GetOrCreateNamespace returns an existing Linux network namespace or creates a new one if it doesn't exist yet.
	// Only named namespaces can be created
	GetOrCreateNamespace(ns *Namespace) (netns.NsHandle, error)
	// IsNamespaceAvailable verifies whether required namespace exists and is accessible
	IsNamespaceAvailable(ns *interfaces.LinuxInterfaces_Interface_Namespace) bool
	// SwitchNamespace switches the network namespace of the current thread
	SwitchNamespace(ns *Namespace, ctx *NamespaceMgmtCtx) (revert func(), err error)
	// SwitchToNamespace switches the network namespace of the current thread // todo merge these two methods if possible
	SwitchToNamespace(nsMgmtCtx *NamespaceMgmtCtx, ns *interfaces.LinuxInterfaces_Interface_Namespace) (revert func(), err error)
	// GetConfigNamespace returns configuration namespace (used for VETHs)
	GetConfigNamespace() *interfaces.LinuxInterfaces_Interface_Namespace
	//ConvertMicroserviceNsToPidNs converts microservice-referenced namespace into the PID-referenced namespace
	ConvertMicroserviceNsToPidNs(msLabel string) (pidNs *Namespace)
}

// NsConvertor defines common methods to convert namespace types
type NsConvertor interface {
	// IfaceNsToString returns a string representation of namespace
	IfaceNsToString(namespace *interfaces.LinuxInterfaces_Interface_Namespace) string
	// IfNsToGeneric converts interface-specific type namespace to generic type
	IfNsToGeneric(ns *interfaces.LinuxInterfaces_Interface_Namespace) *Namespace
	// ArpNsToGeneric converts arp-specific type namespace to generic type
	ArpNsToGeneric(ns *l3.LinuxStaticArpEntries_ArpEntry_Namespace) *Namespace
	// GenericToArpNs converts generic namespace to arp-specific type
	GenericToArpNs(ns *Namespace) (*l3.LinuxStaticArpEntries_ArpEntry_Namespace, error)
	// RouteNsToGeneric converts route-specific type namespace to generic type
	RouteNsToGeneric(ns *l3.LinuxStaticRoutes_Route_Namespace) *Namespace
}

// Microservices defines all methods needed to manage microservices
type Microservices interface {
	// HandleMicroservices handles microservice changes
	HandleMicroservices(ctx *MicroserviceCtx)
}
