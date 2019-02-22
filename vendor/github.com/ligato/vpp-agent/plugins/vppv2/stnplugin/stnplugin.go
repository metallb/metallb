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

//go:generate descriptor-adapter --descriptor-name STN --value-type *vpp_stn.Rule --import "github.com/ligato/vpp-agent/api/models/vpp/stn" --output-dir "descriptor"

package stnplugin

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/stnplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/stnplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/stnplugin/vppcalls"
	"github.com/pkg/errors"
)

// STNPlugin configures VPP STN rules using GoVPP.
type STNPlugin struct {
	Deps

	// GoVPP
	vppCh govppapi.Channel

	// handlers
	stnHandler vppcalls.StnVppAPI

	// descriptors
	stnDescriptor *descriptor.STNDescriptor
}

// Deps lists dependencies of the STN plugin.
type Deps struct {
	infra.PluginDeps
	KVScheduler kvs.KVScheduler
	GoVppmux    govppmux.API
	IfPlugin    ifplugin.API
	StatusCheck statuscheck.PluginStatusWriter // optional
}

// Init registers STN-related descriptors.
func (p *STNPlugin) Init() (err error) {
	// GoVPP channels
	if p.vppCh, err = p.GoVppmux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create GoVPP API channel: %v", err)
	}

	// init STN handler
	p.stnHandler = vppcalls.NewStnVppHandler(p.vppCh, p.IfPlugin.GetInterfaceIndex(), p.Log)

	// init and register STN descriptor
	p.stnDescriptor = descriptor.NewSTNDescriptor(p.stnHandler, p.Log)
	stnDescriptor := adapter.NewSTNDescriptor(p.stnDescriptor.GetDescriptor())
	p.KVScheduler.RegisterKVDescriptor(stnDescriptor)

	return nil
}

// AfterInit registers plugin with StatusCheck.
func (p *STNPlugin) AfterInit() error {
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, nil)
	}
	return nil
}
