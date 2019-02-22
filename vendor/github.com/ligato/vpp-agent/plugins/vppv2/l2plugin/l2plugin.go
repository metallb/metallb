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

//go:generate descriptor-adapter --descriptor-name BridgeDomain --value-type *vpp_l2.BridgeDomain --meta-type *idxvpp2.OnlyIndex --import "github.com/ligato/vpp-agent/pkg/idxvpp2" --import "github.com/ligato/vpp-agent/api/models/vpp/l2" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name BDInterface --value-type *vpp_l2.BridgeDomain_Interface --import "github.com/ligato/vpp-agent/api/models/vpp/l2" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name FIB  --value-type *vpp_l2.FIBEntry --import "github.com/ligato/vpp-agent/api/models/vpp/l2" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name XConnect  --value-type *vpp_l2.XConnectPair --import "github.com/ligato/vpp-agent/api/models/vpp/l2" --output-dir "descriptor"

package l2plugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/pkg/errors"

	"github.com/ligato/vpp-agent/pkg/idxvpp2"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin/vppcalls"
)

// L2Plugin configures VPP bridge domains, L2 FIBs and xConnects using GoVPP.
type L2Plugin struct {
	Deps

	// GoVPP
	vppCh govppapi.Channel

	// handlers
	bdHandler  vppcalls.BridgeDomainVppAPI
	fibHandler vppcalls.FIBVppAPI
	xCHandler  vppcalls.XConnectVppAPI

	// descriptors
	bdDescriptor      *descriptor.BridgeDomainDescriptor
	bdIfaceDescriptor *descriptor.BDInterfaceDescriptor
	fibDescriptor     *descriptor.FIBDescriptor
	xcDescriptor      *descriptor.XConnectDescriptor

	// index maps
	bdIndex idxvpp2.NameToIndex
}

// Deps lists dependencies of the L2 plugin.
type Deps struct {
	infra.PluginDeps
	KVScheduler kvs.KVScheduler
	GoVppmux    govppmux.API
	IfPlugin    ifplugin.API
	StatusCheck statuscheck.PluginStatusWriter // optional
}

// Init registers L2-related descriptors.
func (p *L2Plugin) Init() (err error) {
	// GoVPP channels
	if p.vppCh, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create GoVPP API channel: %v", err)
	}

	// init BD handler
	p.bdHandler = vppcalls.NewBridgeDomainVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), p.Log)

	// init and register bridge domain descriptor
	p.bdDescriptor = descriptor.NewBridgeDomainDescriptor(p.bdHandler, p.Log)
	bdDescriptor := adapter.NewBridgeDomainDescriptor(p.bdDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(bdDescriptor)

	// obtain read-only references to BD index map
	var withIndex bool
	metadataMap := p.KVScheduler.GetMetadataMap(bdDescriptor.Name)
	p.bdIndex, withIndex = metadataMap.(idxvpp2.NameToIndex)
	if !withIndex {
		return errors.New("missing index with bridge domain metadata")
	}

	// init FIB and xConnect handlers
	p.fibHandler = vppcalls.NewFIBVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), p.bdIndex, p.Log)
	p.xCHandler = vppcalls.NewXConnectVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), p.Log)

	// init & register descriptors
	p.bdIfaceDescriptor = descriptor.NewBDInterfaceDescriptor(p.bdIndex, p.bdHandler, p.Log)
	bdIfaceDescriptor := adapter.NewBDInterfaceDescriptor(p.bdIfaceDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(bdIfaceDescriptor)

	p.fibDescriptor = descriptor.NewFIBDescriptor(p.fibHandler, p.Log)
	fibDescriptor := adapter.NewFIBDescriptor(p.fibDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(fibDescriptor)

	p.xcDescriptor = descriptor.NewXConnectDescriptor(p.xCHandler, p.Log)
	xcDescriptor := adapter.NewXConnectDescriptor(p.xcDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(xcDescriptor)

	return nil
}

// AfterInit registers plugin with StatusCheck.
func (p *L2Plugin) AfterInit() error {
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, nil)
	}
	return nil
}

// GetBDIndex return bridge domain index.
func (p *L2Plugin) GetBDIndex() idxvpp2.NameToIndex {
	return p.bdIndex
}
