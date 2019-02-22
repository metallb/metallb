//  Copyright (c) 2018 Cisco and/or its affiliates.
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

package descriptor

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
	"github.com/pkg/errors"
)

const (
	// ProxyArpInterfaceDescriptorName is the name of the descriptor.
	ProxyArpInterfaceDescriptorName = "vpp-proxy-arp-interface"

	// dependency labels
	proxyArpInterfaceDep = "interface-exists"
)

// ProxyArpInterfaceDescriptor teaches KVScheduler how to configure VPP proxy ARP interfaces.
type ProxyArpInterfaceDescriptor struct {
	log             logging.Logger
	proxyArpHandler vppcalls.ProxyArpVppAPI
	scheduler       kvs.KVScheduler
}

// NewProxyArpInterfaceDescriptor creates a new instance of the ProxyArpInterfaceDescriptor.
func NewProxyArpInterfaceDescriptor(scheduler kvs.KVScheduler,
	proxyArpHandler vppcalls.ProxyArpVppAPI, log logging.PluginLogger) *ProxyArpInterfaceDescriptor {

	return &ProxyArpInterfaceDescriptor{
		scheduler:       scheduler,
		proxyArpHandler: proxyArpHandler,
		log:             log.NewLogger("proxy-arp-interface-descriptor"),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *ProxyArpInterfaceDescriptor) GetDescriptor() *adapter.ProxyARPInterfaceDescriptor {
	return &adapter.ProxyARPInterfaceDescriptor{
		Name: ProxyArpInterfaceDescriptorName,
		KeySelector: func(key string) bool {
			_, isProxyARPInterfaceKey := l3.ParseProxyARPInterfaceKey(key)
			return isProxyARPInterfaceKey
		},
		ValueTypeName: proto.MessageName(&l3.ProxyARP_Interface{}),
		Add:           d.Add,
		Delete:        d.Delete,
		ModifyWithRecreate: func(key string, oldValue, newValue *l3.ProxyARP_Interface, metadata interface{}) bool {
			return true
		},
		Dependencies: d.Dependencies,
	}
}

// Add enables VPP Proxy ARP for interface.
func (d *ProxyArpInterfaceDescriptor) Add(key string, value *l3.ProxyARP_Interface) (metadata interface{}, err error) {
	if err := d.proxyArpHandler.EnableProxyArpInterface(value.Name); err != nil {
		return nil, errors.Errorf("failed to enable proxy ARP for interface %s: %v", value.Name, err)
	}
	return nil, nil
}

// Delete disables VPP Proxy ARP for interface.
func (d *ProxyArpInterfaceDescriptor) Delete(key string, value *l3.ProxyARP_Interface, metadata interface{}) error {
	if err := d.proxyArpHandler.DisableProxyArpInterface(value.Name); err != nil {
		return errors.Errorf("failed to disable proxy ARP for interface %s: %v", value.Name, err)
	}
	return nil
}

// Dependencies returns list of dependencies for VPP Proxy ARP interface.
func (d *ProxyArpInterfaceDescriptor) Dependencies(key string, value *l3.ProxyARP_Interface) (deps []kvs.Dependency) {
	return []kvs.Dependency{
		{
			Label: proxyArpInterfaceDep,
			Key:   interfaces.InterfaceKey(value.Name),
		},
	}
}
