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

package ifplugin

import (
	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/vpp-agent/api/models/vpp"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
)

// API defines methods exposed by VPP-IfPlugin.
type API interface {
	// GetInterfaceIndex gives read-only access to map with metadata of all configured
	// VPP interfaces.
	GetInterfaceIndex() ifaceidx.IfaceMetadataIndex

	// GetDHCPIndex gives read-only access to (untyped) map with DHCP leases.
	// Cast metadata to "github.com/ligato/vpp-agent/api/models/vpp/interfaces".DHCPLease
	GetDHCPIndex() idxmap.NamedMapping

	// SetNotifyService allows to pass function for updating interface notifications.
	SetNotifyService(notify func(notification *vpp.Notification))
}
