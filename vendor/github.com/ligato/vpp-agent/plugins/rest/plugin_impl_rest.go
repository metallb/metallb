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

	"git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/rpc/rest"
	access "github.com/ligato/cn-infra/rpc/rest/security/model/access-security"
	"github.com/ligato/cn-infra/utils/safeclose"

	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/linux"
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
	GET  = http.MethodGet
	POST = http.MethodPost
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
func (p *Plugin) Init() (err error) {
	// Check VPP dependency
	if p.VPP == nil {
		return fmt.Errorf("REST plugin requires VPP plugin API")
	}

	// VPP channels
	if p.vppChan, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return err
	}
	if p.dumpChan, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return err
	}

	// VPP Indexes
	ifIndexes := p.VPP.GetSwIfIndexes()
	bdIndexes := p.VPP.GetBDIndexes()
	spdIndexes := p.VPP.GetIPSecSPDIndexes()

	// Initialize VPP handlers
	p.aclHandler = aclvppcalls.NewACLVppHandler(p.vppChan, p.dumpChan)
	p.ifHandler = ifvppcalls.NewIfVppHandler(p.vppChan, p.Log)
	p.bfdHandler = ifvppcalls.NewBfdVppHandler(p.vppChan, ifIndexes, p.Log)
	p.natHandler = ifvppcalls.NewNatVppHandler(p.vppChan, p.dumpChan, ifIndexes, p.Log)
	p.stnHandler = ifvppcalls.NewStnVppHandler(p.vppChan, ifIndexes, p.Log)
	p.ipSecHandler = ipsecvppcalls.NewIPsecVppHandler(p.vppChan, ifIndexes, spdIndexes, p.Log)
	p.bdHandler = l2vppcalls.NewBridgeDomainVppHandler(p.vppChan, ifIndexes, p.Log)
	p.fibHandler = l2vppcalls.NewFibVppHandler(p.vppChan, p.dumpChan, ifIndexes, bdIndexes, p.Log)
	p.xcHandler = l2vppcalls.NewXConnectVppHandler(p.vppChan, ifIndexes, p.Log)
	p.arpHandler = l3vppcalls.NewArpVppHandler(p.vppChan, ifIndexes, p.Log)
	p.pArpHandler = l3vppcalls.NewProxyArpVppHandler(p.vppChan, ifIndexes, p.Log)
	p.rtHandler = l3vppcalls.NewRouteVppHandler(p.vppChan, ifIndexes, p.Log)
	p.l4Handler = l4vppcalls.NewL4VppHandler(p.vppChan, p.Log)

	// Linux indexes and handlers
	if p.Linux != nil {
		linuxIfIndexes := p.Linux.GetLinuxIfIndexes()
		linuxArpIndexes := p.Linux.GetLinuxARPIndexes()
		linuxRtIndexes := p.Linux.GetLinuxRouteIndexes()
		// Initialize Linux handlers
		linuxNsHandler := p.Linux.GetNamespaceHandler()
		p.linuxIfHandler = iflinuxcalls.NewNetLinkHandler(linuxNsHandler, linuxIfIndexes, p.Log)
		p.linuxL3Handler = l3linuxcalls.NewNetLinkHandler(linuxNsHandler, linuxIfIndexes, linuxArpIndexes, linuxRtIndexes, p.Log)
	}

	p.index = &index{
		ItemMap: getIndexMap(),
	}

	// Register permission groups, used if REST security is enabled
	p.HTTPHandlers.RegisterPermissionGroup(getPermissionsGroups()...)

	return nil
}

// AfterInit is used to register HTTP handlers
func (p *Plugin) AfterInit() (err error) {
	p.Log.Debug("REST API Plugin is up and running")

	// VPP handlers
	p.registerAccessListHandlers()
	p.registerInterfaceHandlers()
	p.registerBfdHandlers()
	p.registerNatHandlers()
	p.registerStnHandlers()
	p.registerIPSecHandlers()
	p.registerL2Handlers()
	p.registerL3Handlers()
	p.registerL4Handlers()

	// Linux handlers
	if p.Linux != nil {
		p.registerLinuxInterfaceHandlers()
		p.registerLinuxL3Handlers()
	}

	// Telemetry, command, index, tracer
	p.registerTracerHandler()
	p.registerTelemetryHandlers()
	p.registerCommandHandler()
	p.registerIndexHandlers()

	return nil
}

// Close is used to clean up resources used by Plugin
func (p *Plugin) Close() (err error) {
	return safeclose.Close(p.vppChan, p.dumpChan)
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
			newPermission(resturl.Index, GET),
			newPermission(resturl.Tracer, GET),
		},
	}
	telemetryPg := &access.PermissionGroup{
		Name: "telemetry",
		Permissions: []*access.PermissionGroup_Permissions{
			newPermission(resturl.Index, GET),
			newPermission(resturl.Telemetry, GET),
			newPermission(resturl.TMemory, GET),
			newPermission(resturl.TRuntime, GET),
			newPermission(resturl.TNodeCount, GET),
		},
	}
	dumpPg := &access.PermissionGroup{
		Name: "dump",
		Permissions: []*access.PermissionGroup_Permissions{
			newPermission(resturl.Index, GET),
			newPermission(resturl.ACLIP, GET),
			newPermission(resturl.ACLMACIP, GET),
			newPermission(resturl.Interface, GET),
			newPermission(resturl.Loopback, GET),
			newPermission(resturl.Ethernet, GET),
			newPermission(resturl.Memif, GET),
			newPermission(resturl.Tap, GET),
			newPermission(resturl.VxLan, GET),
			newPermission(resturl.AfPacket, GET),
			newPermission(resturl.IPSecSpd, GET),
			newPermission(resturl.IPSecSa, GET),
			newPermission(resturl.IPSecTnIf, GET),
			newPermission(resturl.Bd, GET),
			newPermission(resturl.BdID, GET),
			newPermission(resturl.Fib, GET),
			newPermission(resturl.Xc, GET),
			newPermission(resturl.Arps, GET),
			newPermission(resturl.Routes, GET),
			newPermission(resturl.PArpIfs, GET),
			newPermission(resturl.PArpRngs, GET),
			newPermission(resturl.Sessions, GET),
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
