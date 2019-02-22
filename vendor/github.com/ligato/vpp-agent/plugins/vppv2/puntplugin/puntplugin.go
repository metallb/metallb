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

//go:generate descriptor-adapter --descriptor-name PuntToHost --value-type *vpp_punt.ToHost --import "github.com/ligato/vpp-agent/api/models/vpp/punt" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name IPPuntRedirect --value-type *vpp_punt.IPRedirect --import "github.com/ligato/vpp-agent/api/models/vpp/punt" --output-dir "descriptor"

package puntplugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/puntplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/puntplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/puntplugin/vppcalls"
)

// PuntPlugin configures VPP punt to host or unix domain socket entries and IP redirect entries using GoVPP.
type PuntPlugin struct {
	Deps

	// GoVPP
	vppCh govppapi.Channel

	// handler
	puntHandler vppcalls.PuntVppAPI

	// descriptors
	toHostDescriptor     *descriptor.PuntToHostDescriptor
	ipRedirectDescriptor *descriptor.IPRedirectDescriptor
}

// Deps lists dependencies of the punt plugin.
type Deps struct {
	infra.PluginDeps
	KVScheduler kvs.KVScheduler
	GoVppmux    govppmux.API
	IfPlugin    ifplugin.API
	StatusCheck statuscheck.PluginStatusWriter // optional
}

// Init registers STN-related descriptors.
func (p *PuntPlugin) Init() (err error) {
	// GoVPP channels
	if p.vppCh, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create GoVPP API channel: %v", err)
	}

	// init punt handler
	p.puntHandler = vppcalls.NewPuntVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), p.Log)

	// init and register punt descriptor
	p.toHostDescriptor = descriptor.NewPuntToHostDescriptor(p.puntHandler, p.Log)
	toHostDescriptor := adapter.NewPuntToHostDescriptor(p.toHostDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(toHostDescriptor)

	// init and register IP punt redirect
	p.ipRedirectDescriptor = descriptor.NewIPRedirectDescriptor(p.puntHandler, p.Log)
	ipRedirectDescriptor := adapter.NewIPPuntRedirectDescriptor(p.ipRedirectDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(ipRedirectDescriptor)

	return nil
}

// AfterInit registers plugin with StatusCheck.
func (p *PuntPlugin) AfterInit() error {
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, nil)
	}
	return nil
}
