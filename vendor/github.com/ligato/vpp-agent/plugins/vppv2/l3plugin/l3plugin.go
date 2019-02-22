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

//go:generate descriptor-adapter --descriptor-name Route --value-type *vpp_l3.Route --import "github.com/ligato/vpp-agent/api/models/vpp/l3" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name ARPEntry --value-type *vpp_l3.ARPEntry --import "github.com/ligato/vpp-agent/api/models/vpp/l3" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name ProxyARP --value-type *vpp_l3.ProxyARP --import "github.com/ligato/vpp-agent/api/models/vpp/l3" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name ProxyARPInterface --value-type *vpp_l3.ProxyARP_Interface --import "github.com/ligato/vpp-agent/api/models/vpp/l3" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name IPScanNeighbor --value-type *vpp_l3.IPScanNeighbor --import "github.com/ligato/vpp-agent/api/models/vpp/l3" --output-dir "descriptor"

package l3plugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/pkg/errors"

	"github.com/ligato/vpp-agent/plugins/govppmux"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
)

// L3Plugin configures Linux routes and ARP entries using Netlink API.
type L3Plugin struct {
	Deps

	// GoVPP channels
	vppCh govppapi.Channel

	// VPP handlers
	routeHandler    vppcalls.RouteVppAPI
	arpandler       vppcalls.ArpVppAPI
	proxyArpHandler vppcalls.ProxyArpVppAPI
	ipNeigh         vppcalls.IPNeighVppAPI

	// descriptors
	routeDescriptor          *descriptor.RouteDescriptor
	arpDescriptor            *descriptor.ArpDescriptor
	proxyArpDescriptor       *descriptor.ProxyArpDescriptor
	proxyArpIfaceDescriptor  *descriptor.ProxyArpInterfaceDescriptor
	ipScanNeighborDescriptor *descriptor.IPScanNeighborDescriptor
}

// Deps lists dependencies of the interface p.
type Deps struct {
	infra.PluginDeps
	KVScheduler kvs.KVScheduler
	GoVppmux    govppmux.API
	IfPlugin    ifplugin.API
	StatusCheck statuscheck.PluginStatusWriter // optional
}

// Init initializes and registers descriptors for Linux ARPs and Routes.
func (p *L3Plugin) Init() error {
	var err error

	// GoVPP channels
	if p.vppCh, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create GoVPP API channel: %v", err)
	}

	// init handlers
	p.routeHandler = vppcalls.NewRouteVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), nil)
	p.arpandler = vppcalls.NewArpVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), nil)
	p.proxyArpHandler = vppcalls.NewProxyArpVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), nil)
	p.ipNeigh = vppcalls.NewIPNeighVppHandler(p.vppCh, nil)

	// init & register descriptors
	p.routeDescriptor = descriptor.NewRouteDescriptor(p.routeHandler, p.Log)
	routeDescriptor := adapter.NewRouteDescriptor(p.routeDescriptor.GetDescriptor())
	p.Deps.KVScheduler.RegisterKVDescriptor(routeDescriptor)

	p.arpDescriptor = descriptor.NewArpDescriptor(p.KVScheduler, p.arpandler, p.Log)
	arpDescriptor := adapter.NewARPEntryDescriptor(p.arpDescriptor.GetDescriptor())
	p.Deps.KVScheduler.RegisterKVDescriptor(arpDescriptor)

	p.proxyArpDescriptor = descriptor.NewProxyArpDescriptor(p.KVScheduler, p.proxyArpHandler, p.Log)
	proxyArpDescriptor := adapter.NewProxyARPDescriptor(p.proxyArpDescriptor.GetDescriptor())
	p.Deps.KVScheduler.RegisterKVDescriptor(proxyArpDescriptor)

	p.proxyArpIfaceDescriptor = descriptor.NewProxyArpInterfaceDescriptor(p.KVScheduler, p.proxyArpHandler, p.Log)
	proxyArpIfaceDescriptor := adapter.NewProxyARPInterfaceDescriptor(p.proxyArpIfaceDescriptor.GetDescriptor())
	p.Deps.KVScheduler.RegisterKVDescriptor(proxyArpIfaceDescriptor)

	p.ipScanNeighborDescriptor = descriptor.NewIPScanNeighborDescriptor(p.KVScheduler, p.ipNeigh, p.Log)
	ipScanNeighborDescriptor := adapter.NewIPScanNeighborDescriptor(p.ipScanNeighborDescriptor.GetDescriptor())
	p.Deps.KVScheduler.RegisterKVDescriptor(ipScanNeighborDescriptor)

	return nil
}

// AfterInit registers plugin with StatusCheck.
func (p *L3Plugin) AfterInit() error {
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, nil)
	}
	return nil
}
