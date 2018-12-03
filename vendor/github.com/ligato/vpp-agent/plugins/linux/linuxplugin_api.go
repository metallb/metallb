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

package linux

import (
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin/l3idx"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
)

// API of Linux Plugin
type API interface {
	// GetLinuxIfIndexes gives access to mapping of logical names (used in ETCD configuration)
	// to corresponding Linux interface indexes. This mapping is especially helpful
	// for plugins that need to watch for newly added or deleted Linux interfaces.
	GetLinuxIfIndexes() ifaceidx.LinuxIfIndex

	// GetLinuxIfIndexes gives access to mapping of logical names (used in ETCD configuration) to corresponding Linux
	// ARP entry indexes. This mapping is especially helpful for plugins that need to watch for newly added or deleted
	// Linux ARP entries.
	GetLinuxARPIndexes() l3idx.LinuxARPIndex

	// GetLinuxIfIndexes gives access to mapping of logical names (used in ETCD configuration) to corresponding Linux
	// route indexes. This mapping is especially helpful for plugins that need to watch for newly added or deleted
	// Linux routes.
	GetLinuxRouteIndexes() l3idx.LinuxRouteIndex

	// GetNamespaceHandler gives access to namespace API which allows plugins to manipulate with linux namespaces
	GetNamespaceHandler() nsplugin.NamespaceAPI
}
