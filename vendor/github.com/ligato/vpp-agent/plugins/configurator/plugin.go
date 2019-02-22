//  Copyright (c) 2019 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package configurator

import (
	"git.fd.io/govpp.git/api"
	"github.com/ligato/vpp-agent/api/models/vpp"
	"github.com/ligato/vpp-agent/plugins/orchestrator"
	ipsecvppcalls "github.com/ligato/vpp-agent/plugins/vppv2/ipsecplugin/vppcalls"
	puntvppcalls "github.com/ligato/vpp-agent/plugins/vppv2/puntplugin/vppcalls"

	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/rpc/grpc"

	rpc "github.com/ligato/vpp-agent/api/configurator"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	iflinuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/linuxcalls"
	l3linuxcalls "github.com/ligato/vpp-agent/plugins/linuxv2/l3plugin/linuxcalls"
	aclvppcalls "github.com/ligato/vpp-agent/plugins/vppv2/aclplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
	ifvppcalls "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin"
	l2vppcalls "github.com/ligato/vpp-agent/plugins/vppv2/l2plugin/vppcalls"
	l3vppcalls "github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
	natvppcalls "github.com/ligato/vpp-agent/plugins/vppv2/natplugin/vppcalls"
)

// Plugin registers VPP GRPC services in *grpc.Server.
type Plugin struct {
	Deps

	configurator configuratorServer

	// Channels
	vppChan  api.Channel
	dumpChan api.Channel
}

// Deps - dependencies of Plugin
type Deps struct {
	infra.PluginDeps
	GRPCServer  grpc.Server
	Orch        *orchestrator.Plugin
	GoVppmux    govppmux.TraceAPI
	VPPIfPlugin ifplugin.API
	VPPL2Plugin *l2plugin.L2Plugin
}

// Init sets plugin child loggers
func (p *Plugin) Init() error {
	p.configurator.log = p.Log.NewLogger("configurator")
	p.configurator.notifyService.log = p.Log.NewLogger("configurator-notify")
	p.configurator.dispatch = p.Orch

	if err := p.initHandlers(); err != nil {
		return err
	}

	grpcServer := p.GRPCServer.GetServer()
	if grpcServer != nil {
		rpc.RegisterConfiguratorServer(grpcServer, &p.configurator)
	}

	if p.VPPIfPlugin != nil {
		p.VPPIfPlugin.SetNotifyService(func(vppNotification *vpp.Notification) {
			p.configurator.notifyService.pushNotification(&rpc.Notification{
				Notification: &rpc.Notification_VppNotification{
					VppNotification: vppNotification,
				},
			})
		})
	}

	return nil
}

// Close does nothing.
func (p *Plugin) Close() error {
	return nil
}

// helper method initializes all VPP/Linux plugin handlers
func (p *Plugin) initHandlers() (err error) {
	// VPP channels
	if p.vppChan, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return err
	}
	if p.dumpChan, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return err
	}

	// VPP Indexes
	ifIndexes := p.VPPIfPlugin.GetInterfaceIndex()
	bdIndexes := p.VPPL2Plugin.GetBDIndex()
	dhcpIndexes := p.VPPIfPlugin.GetDHCPIndex()

	// Initialize VPP handlers
	p.configurator.aclHandler = aclvppcalls.NewACLVppHandler(p.vppChan, p.dumpChan, ifIndexes)
	p.configurator.ifHandler = ifvppcalls.NewIfVppHandler(p.vppChan, p.Log)
	p.configurator.natHandler = natvppcalls.NewNatVppHandler(p.vppChan, ifIndexes, dhcpIndexes, p.Log)
	p.configurator.bdHandler = l2vppcalls.NewBridgeDomainVppHandler(p.vppChan, ifIndexes, p.Log)
	p.configurator.fibHandler = l2vppcalls.NewFIBVppHandler(p.vppChan, ifIndexes, bdIndexes, p.Log)
	p.configurator.xcHandler = l2vppcalls.NewXConnectVppHandler(p.vppChan, ifIndexes, p.Log)
	p.configurator.arpHandler = l3vppcalls.NewArpVppHandler(p.vppChan, ifIndexes, p.Log)
	p.configurator.pArpHandler = l3vppcalls.NewProxyArpVppHandler(p.vppChan, ifIndexes, p.Log)
	p.configurator.rtHandler = l3vppcalls.NewRouteVppHandler(p.vppChan, ifIndexes, p.Log)
	p.configurator.ipsecHandler = ipsecvppcalls.NewIPsecVppHandler(p.vppChan, ifIndexes, p.Log)
	p.configurator.puntHandler = puntvppcalls.NewPuntVppHandler(p.vppChan, ifIndexes, p.Log)

	// Linux indexes and handlers
	p.configurator.linuxIfHandler = iflinuxcalls.NewNetLinkHandler()
	p.configurator.linuxL3Handler = l3linuxcalls.NewNetLinkHandler()

	return nil
}
