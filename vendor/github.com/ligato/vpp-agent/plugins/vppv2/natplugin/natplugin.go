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

//go:generate descriptor-adapter --descriptor-name NAT44Global --value-type *vpp_nat.Nat44Global --import "github.com/ligato/vpp-agent/api/models/vpp/nat" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name NAT44Interface --value-type *vpp_nat.Nat44Global_Interface --import "github.com/ligato/vpp-agent/api/models/vpp/nat" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name DNAT44 --value-type *vpp_nat.DNat44 --import "github.com/ligato/vpp-agent/api/models/vpp/nat" --output-dir "descriptor"

package natplugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/pkg/errors"

	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"

	"github.com/ligato/vpp-agent/plugins/govppmux"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/natplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/natplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/natplugin/vppcalls"
)

// NATPlugin configures VPP NAT.
type NATPlugin struct {
	Deps

	// GoVPP
	vppCh govppapi.Channel

	// handlers
	natHandler vppcalls.NatVppAPI

	// descriptors
	nat44GlobalDescriptor *descriptor.NAT44GlobalDescriptor
	nat44IfaceDescriptor  *descriptor.NAT44InterfaceDescriptor
	dnat44Descriptor      *descriptor.DNAT44Descriptor
}

// Deps lists dependencies of the NAT plugin.
type Deps struct {
	infra.PluginDeps
	KVScheduler kvs.KVScheduler
	GoVppmux    govppmux.API
	IfPlugin    ifplugin.API
	StatusCheck statuscheck.PluginStatusWriter // optional
}

// Init registers NAT-related descriptors.
func (p *NATPlugin) Init() error {
	var err error

	// GoVPP channels
	if p.vppCh, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create GoVPP API channel: %v", err)
	}

	// init NAT handler
	p.natHandler = vppcalls.NewNatVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), p.IfPlugin.GetDHCPIndex(), p.Log)

	// init and register descriptors
	p.nat44GlobalDescriptor = descriptor.NewNAT44GlobalDescriptor(p.natHandler, p.Log)
	nat44GlobalDescriptor := adapter.NewNAT44GlobalDescriptor(p.nat44GlobalDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(nat44GlobalDescriptor)

	p.nat44IfaceDescriptor = descriptor.NewNAT44InterfaceDescriptor(p.natHandler, p.Log)
	nat44IfaceDescriptor := adapter.NewNAT44InterfaceDescriptor(p.nat44IfaceDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(nat44IfaceDescriptor)

	p.dnat44Descriptor = descriptor.NewDNAT44Descriptor(p.natHandler, p.Log)
	dnat44Descriptor := adapter.NewDNAT44Descriptor(p.dnat44Descriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(dnat44Descriptor)

	return nil
}

// AfterInit registers plugin with StatusCheck.
func (p *NATPlugin) AfterInit() error {
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, nil)
	}
	return nil
}
