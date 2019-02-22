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
	"github.com/vishvananda/netns"

	"github.com/ligato/vpp-agent/api/models/linux/namespace"
	"github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin/linuxcalls"
)

// API defines methods exposed by NsPlugin.
type API interface {
	// SwitchToNamespace switches the network namespace of the current thread.
	// Caller should eventually call the returned "revert" function in order to get back to the original
	// network namespace (for example using "defer revert()").
	SwitchToNamespace(ctx linuxcalls.NamespaceMgmtCtx, ns *linux_namespace.NetNamespace) (revert func(), err error)

	// GetNamespaceHandle returns low-level run-time handle for the given namespace
	// to be used with Netlink API. Do not forget to eventually close the handle using
	// the netns.NsHandle.Close() method.
	GetNamespaceHandle(ctx linuxcalls.NamespaceMgmtCtx, ns *linux_namespace.NetNamespace) (handle netns.NsHandle, err error)
}
