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

package rest

import (
	"fmt"
	"net/http"
	"sync"

	access "github.com/ligato/cn-infra/rpc/rest/security/model/access-security"

	"github.com/ligato/vpp-agent/plugins/linux"

	"git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	iflinuxcalls "github.com/ligato/vpp-agent/plugins/linux/ifplugin/linuxcalls"
	l3linuxcalls "github.com/ligato/vpp-agent/plugins/linux/l3plugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/rest/resturl"
	"github.com/ligato/vpp-agent/plugins/vpp"
	aclvppcalls "github.com/ligato/vpp-agent/plugins/vpp/aclplugin/vppcalls"
	ifvppcalls "github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	ipsecvppcalls "github.com/ligato/vpp-agent/plugins/vpp/ipsecplugin/vppcalls"
	l2vppcalls "github.com/ligato/vpp-agent/plugins/vpp/l2plugin/vppcalls"
	l3vppcalls "github.com/ligato/vpp-agent/plugins/vpp/l3plugin/vppcalls"
	l4vppcalls "github.com/ligato/vpp-agent/plugins/vpp/l4plugin/vppcalls"
)

// REST api methods
const (
	GET  = "GET"
	POST = "POST"
)

// Plugin registers Rest Plugin
type Plugin struct {
	Deps

	// Index page
	index *index

	// Channels
	vppChan  api.Channel
	dumpChan api.Channel

	// VPP Handlers
	aclHandler   aclvppcalls.ACLVppRead
	ifHandler    ifvppcalls.IfVppRead
	bfdHandler   ifvppcalls.BfdVppRead
	natHandler   ifvppcalls.NatVppRead
	stnHandler   ifvppcalls.StnVppRead
	ipSecHandler ipsecvppcalls.IPSecVPPRead
	bdHandler    l2vppcalls.BridgeDomainVppRead
	fibHandler   l2vppcalls.FibVppRead
	xcHandler    l2vppcalls.XConnectVppRead
	arpHandler   l3vppcalls.ArpVppRead
	pArpHandler  l3vppcalls.ProxyArpVppRead
	rtHandler    l3vppcalls.RouteVppRead
	l4Handler    l4vppcalls.L4VppRead
	// Linux handlers
	linuxIfHandler iflinuxcalls.NetlinkAPI
	linuxL3Handler l3linuxcalls.NetlinkAPI

	govppmux sync.Mutex
}

// Deps represents dependencies of Rest Plugin
type Deps struct {
	infra.PluginDeps
	HTTPHandlers rest.HTTPHandlers
	GoVppmux     govppmux.TraceAPI
	VPP          vpp.API
	Linux        linux.API
}

// index defines map of main index page entries
type index struct {
	ItemMap map[string][]indexItem
}

// indexItem is single index page entry
type indexItem struct {
	Name string
	Path string
}

// Init initializes the Rest Plugin
func (plugin *Plugin) Init() (err error) {
	// Check VPP dependency
	if plugin.VPP == nil {
		return fmt.Errorf("REST plugin requires VPP plugin API")
	}
	// VPP channels
	if plugin.vppChan, err = plugin.GoVppmux.NewAPIChannel(); err != nil {
		return err
	}
	if plugin.dumpChan, err = plugin.GoVppmux.NewAPIChannel(); err != nil {
		return err
	}
	// VPP Indexes
	ifIndexes := plugin.VPP.GetSwIfIndexes()
	bdIndexes := plugin.VPP.GetBDIndexes()
	spdIndexes := plugin.VPP.GetIPSecSPDIndexes()
	// Initialize VPP handlers
	plugin.aclHandler = aclvppcalls.NewACLVppHandler(plugin.vppChan, plugin.dumpChan)
	plugin.ifHandler = ifvppcalls.NewIfVppHandler(plugin.vppChan, plugin.Log)
	plugin.bfdHandler = ifvppcalls.NewBfdVppHandler(plugin.vppChan, ifIndexes, plugin.Log)
	plugin.natHandler = ifvppcalls.NewNatVppHandler(plugin.vppChan, plugin.dumpChan, ifIndexes, plugin.Log)
	plugin.stnHandler = ifvppcalls.NewStnVppHandler(plugin.vppChan, ifIndexes, plugin.Log)
	plugin.ipSecHandler = ipsecvppcalls.NewIPsecVppHandler(plugin.vppChan, ifIndexes, spdIndexes, plugin.Log)
	plugin.bdHandler = l2vppcalls.NewBridgeDomainVppHandler(plugin.vppChan, ifIndexes, plugin.Log)
	plugin.fibHandler = l2vppcalls.NewFibVppHandler(plugin.vppChan, plugin.dumpChan, ifIndexes, bdIndexes, plugin.Log)
	plugin.xcHandler = l2vppcalls.NewXConnectVppHandler(plugin.vppChan, ifIndexes, plugin.Log)
	plugin.arpHandler = l3vppcalls.NewArpVppHandler(plugin.vppChan, ifIndexes, plugin.Log)
	plugin.pArpHandler = l3vppcalls.NewProxyArpVppHandler(plugin.vppChan, ifIndexes, plugin.Log)
	plugin.rtHandler = l3vppcalls.NewRouteVppHandler(plugin.vppChan, ifIndexes, plugin.Log)
	plugin.l4Handler = l4vppcalls.NewL4VppHandler(plugin.vppChan, plugin.Log)
	// Linux indexes and handlers
	if plugin.Linux != nil {
		linuxIfIndexes := plugin.Linux.GetLinuxIfIndexes()
		linuxArpIndexes := plugin.Linux.GetLinuxARPIndexes()
		linuxRtIndexes := plugin.Linux.GetLinuxRouteIndexes()
		// Initialize Linux handlers
		linuxNsHandler := plugin.Linux.GetNamespaceHandler()
		plugin.linuxIfHandler = iflinuxcalls.NewNetLinkHandler(linuxNsHandler, linuxIfIndexes, plugin.Log)
		plugin.linuxL3Handler = l3linuxcalls.NewNetLinkHandler(linuxNsHandler, linuxIfIndexes, linuxArpIndexes, linuxRtIndexes, plugin.Log)
	}

	plugin.index = &index{
		ItemMap: getIndexMap(),
	}

	// Register permission groups, used if REST security is enabled
	plugin.HTTPHandlers.RegisterPermissionGroup(getPermissionsGroups()...)

	return nil
}

// AfterInit is used to register HTTP handlers
func (plugin *Plugin) AfterInit() (err error) {
	plugin.Log.Debug("REST API Plugin is up and running")

	// VPP handlers
	plugin.registerAccessListHandlers()
	plugin.registerInterfaceHandlers()
	plugin.registerBfdHandlers()
	plugin.registerNatHandlers()
	plugin.registerStnHandlers()
	plugin.registerIPSecHandlers()
	plugin.registerL2Handlers()
	plugin.registerL3Handlers()
	plugin.registerL4Handlers()
	// Linux handlers
	if plugin.Linux != nil {
		plugin.registerLinuxInterfaceHandlers()
		plugin.registerLinuxL3Handlers()
	}
	// Telemetry, command, index, tracer
	plugin.registerTracerHandler()
	plugin.registerTelemetryHandlers()
	plugin.registerCommandHandler()
	plugin.registerIndexHandlers()

	return nil
}

// Close is used to clean up resources used by Plugin
func (plugin *Plugin) Close() (err error) {
	return safeclose.Close(plugin.vppChan, plugin.dumpChan)
}

// Fill index item lists
func getIndexMap() map[string][]indexItem {
	idxMap := map[string][]indexItem{
		"ACL plugin": {
			{Name: "IP-type access lists", Path: resturl.ACLIP},
			{Name: "MACIP-type access lists", Path: resturl.ACLMACIP},
		},
		"Interface plugin": {
			{Name: "All interfaces", Path: resturl.Interface},
			{Name: "Loopbacks", Path: resturl.Loopback},
			{Name: "Ethernets", Path: resturl.Ethernet},
			{Name: "Memifs", Path: resturl.Memif},
			{Name: "Taps", Path: resturl.Tap},
			{Name: "VxLANs", Path: resturl.VxLan},
			{Name: "Af-packets", Path: resturl.AfPacket},
		},
		"IPSec plugin": {
			{Name: "Security policy databases", Path: resturl.IPSecSpd},
			{Name: "Security associations", Path: resturl.IPSecSa},
			{Name: "Tunnel interfaces", Path: resturl.IPSecTnIf},
		},
		"L2 plugin": {
			{Name: "Bridge domains", Path: resturl.Bd},
			{Name: "Bridge domain IDs", Path: resturl.BdID},
			{Name: "L2Fibs", Path: resturl.Fib},
			{Name: "Cross connects", Path: resturl.Xc},
		},
		"L3 plugin": {
			{Name: "Routes", Path: resturl.Routes},
			{Name: "ARPs", Path: resturl.Arps},
			{Name: "Proxy ARP interfaces", Path: resturl.PArpIfs},
			{Name: "Proxy ARP ranges", Path: resturl.PArpRngs},
		},
		"L4 plugin": {
			{Name: "L4 sessions", Path: resturl.Sessions},
		},
		"Telemetry": {
			{Name: "All data", Path: resturl.Telemetry},
			{Name: "Memory", Path: resturl.TMemory},
			{Name: "Runtime", Path: resturl.TRuntime},
			{Name: "Node count", Path: resturl.TNodeCount},
		},
		"Tracer": {
			{Name: "Binary API", Path: resturl.Tracer},
		},
	}
	return idxMap
}

// Create permission groups (tracer, telemetry, dump - optionally add more in the future). Used only if
// REST security is enabled in plugin
func getPermissionsGroups() []*access.PermissionGroup {
	tracerPg := &access.PermissionGroup{
		Name: "tracer",
		Permissions: []*access.PermissionGroup_Permissions{
			newPermission(resturl.Index, http.MethodGet),
			newPermission(resturl.Tracer, http.MethodGet),
		},
	}
	telemetryPg := &access.PermissionGroup{
		Name: "telemetry",
		Permissions: []*access.PermissionGroup_Permissions{
			newPermission(resturl.Index, http.MethodGet),
			newPermission(resturl.Telemetry, http.MethodGet),
			newPermission(resturl.TMemory, http.MethodGet),
			newPermission(resturl.TRuntime, http.MethodGet),
			newPermission(resturl.TNodeCount, http.MethodGet),
		},
	}
	dumpPg := &access.PermissionGroup{
		Name: "dump",
		Permissions: []*access.PermissionGroup_Permissions{
			newPermission(resturl.Index, http.MethodGet),
			newPermission(resturl.ACLIP, http.MethodGet),
			newPermission(resturl.ACLMACIP, http.MethodGet),
			newPermission(resturl.Interface, http.MethodGet),
			newPermission(resturl.Loopback, http.MethodGet),
			newPermission(resturl.Ethernet, http.MethodGet),
			newPermission(resturl.Memif, http.MethodGet),
			newPermission(resturl.Tap, http.MethodGet),
			newPermission(resturl.VxLan, http.MethodGet),
			newPermission(resturl.AfPacket, http.MethodGet),
			newPermission(resturl.IPSecSpd, http.MethodGet),
			newPermission(resturl.IPSecSa, http.MethodGet),
			newPermission(resturl.IPSecTnIf, http.MethodGet),
			newPermission(resturl.Bd, http.MethodGet),
			newPermission(resturl.BdID, http.MethodGet),
			newPermission(resturl.Fib, http.MethodGet),
			newPermission(resturl.Xc, http.MethodGet),
			newPermission(resturl.Arps, http.MethodGet),
			newPermission(resturl.Routes, http.MethodGet),
			newPermission(resturl.PArpIfs, http.MethodGet),
			newPermission(resturl.PArpRngs, http.MethodGet),
			newPermission(resturl.Sessions, http.MethodGet),
		},
	}

	return []*access.PermissionGroup{tracerPg, telemetryPg, dumpPg}
}

// Returns permission object with url and provided methods
func newPermission(url string, methods ...string) *access.PermissionGroup_Permissions {
	return &access.PermissionGroup_Permissions{
		Url:            url,
		AllowedMethods: methods,
	}
}
