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

package descriptor

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	"github.com/pkg/errors"

	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	"github.com/ligato/vpp-agent/pkg/idxvpp2"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin/vppcalls"
)

const (
	// BDInterfaceDescriptorName is the name of the descriptor for bindings between
	// VPP bridge domains and interfaces.
	BDInterfaceDescriptorName = "vpp-bd-interface"

	// dependency labels
	interfaceDep = "interface-exists"
)

// BDInterfaceDescriptor teaches KVScheduler how to put interface into VPP bridge
// domain.
type BDInterfaceDescriptor struct {
	// dependencies
	log       logging.Logger
	bdIndex   idxvpp2.NameToIndex
	bdHandler vppcalls.BridgeDomainVppAPI
}

// NewBDInterfaceDescriptor creates a new instance of the BDInterface descriptor.
func NewBDInterfaceDescriptor(bdIndex idxvpp2.NameToIndex, bdHandler vppcalls.BridgeDomainVppAPI, log logging.PluginLogger) *BDInterfaceDescriptor {

	return &BDInterfaceDescriptor{
		bdIndex:   bdIndex,
		bdHandler: bdHandler,
		log:       log.NewLogger("bd-iface-descriptor"),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *BDInterfaceDescriptor) GetDescriptor() *adapter.BDInterfaceDescriptor {
	return &adapter.BDInterfaceDescriptor{
		Name:               BDInterfaceDescriptorName,
		KeySelector:        d.IsBDInterfaceKey,
		ValueTypeName:      proto.MessageName(&l2.BridgeDomain_Interface{}),
		Add:                d.Add,
		Delete:             d.Delete,
		ModifyWithRecreate: d.ModifyWithRecreate,
		Dependencies:       d.Dependencies,
	}
}

// IsBDInterfaceKey returns true if the key is identifying binding between
// VPP bridge domain and interface.
func (d *BDInterfaceDescriptor) IsBDInterfaceKey(key string) bool {
	_, _, isBDIfaceKey := l2.ParseBDInterfaceKey(key)
	return isBDIfaceKey
}

// Add puts interface into bridge domain.
func (d *BDInterfaceDescriptor) Add(key string, bdIface *l2.BridgeDomain_Interface) (metadata interface{}, err error) {
	// get bridge domain index
	bdName, _, _ := l2.ParseBDInterfaceKey(key)
	bdMeta, found := d.bdIndex.LookupByName(bdName)
	if !found {
		err = errors.Errorf("failed to obtain metadata for bridge domain %s", bdName)
		d.log.Error(err)
		return nil, err
	}

	// put interface into the bridge domain
	err = d.bdHandler.AddInterfaceToBridgeDomain(bdMeta.GetIndex(), bdIface)
	if err != nil {
		d.log.Error(err)
		return nil, err

	}
	return nil, nil
}

// Delete removes interface from bridge domain.
func (d *BDInterfaceDescriptor) Delete(key string, bdIface *l2.BridgeDomain_Interface, metadata interface{}) error {
	// get bridge domain index
	bdName, _, _ := l2.ParseBDInterfaceKey(key)
	bdMeta, found := d.bdIndex.LookupByName(bdName)
	if !found {
		err := errors.Errorf("failed to obtain metadata for bridge domain %s", bdName)
		d.log.Error(err)
		return err
	}

	err := d.bdHandler.DeleteInterfaceFromBridgeDomain(bdMeta.GetIndex(), bdIface)
	if err != nil {
		d.log.Error(err)
		return err

	}
	return nil
}

// ModifyWithRecreate returns always true - a change in BVI or SHG is always performed
// via Delete+Add.
func (d *BDInterfaceDescriptor) ModifyWithRecreate(key string, oldBDIface, newBDIface *l2.BridgeDomain_Interface, metadata interface{}) bool {
	return true
}

// Dependencies lists the interface as the only dependency for the binding.
func (d *BDInterfaceDescriptor) Dependencies(key string, value *l2.BridgeDomain_Interface) []kvs.Dependency {
	return []kvs.Dependency{
		{
			Label: interfaceDep,
			Key:   interfaces.InterfaceKey(value.Name),
		},
	}
}
