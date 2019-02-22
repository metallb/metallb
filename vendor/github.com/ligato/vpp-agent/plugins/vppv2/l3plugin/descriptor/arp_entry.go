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
	"github.com/pkg/errors"

	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	ifdescriptor "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
)

const (
	// ArpDescriptorName is the name of the descriptor.
	ArpDescriptorName = "vpp-arp"

	// dependency labels
	arpEntryInterfaceDep = "interface-exists"
)

// ArpDescriptor teaches KVScheduler how to configure VPP ARPs.
type ArpDescriptor struct {
	log        logging.Logger
	arpHandler vppcalls.ArpVppAPI
	scheduler  kvs.KVScheduler
}

// NewArpDescriptor creates a new instance of the ArpDescriptor.
func NewArpDescriptor(scheduler kvs.KVScheduler,
	arpHandler vppcalls.ArpVppAPI, log logging.PluginLogger) *ArpDescriptor {

	return &ArpDescriptor{
		scheduler:  scheduler,
		arpHandler: arpHandler,
		log:        log.NewLogger("arp-descriptor"),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *ArpDescriptor) GetDescriptor() *adapter.ARPEntryDescriptor {
	return &adapter.ARPEntryDescriptor{
		Name:            ArpDescriptorName,
		NBKeyPrefix:     l3.ModelARPEntry.KeyPrefix(),
		ValueTypeName:   l3.ModelARPEntry.ProtoName(),
		KeySelector:     l3.ModelARPEntry.IsKeyValid,
		KeyLabel:        l3.ModelARPEntry.StripKeyPrefix,
		ValueComparator: d.EquivalentArps,
		Add:             d.Add,
		Delete:          d.Delete,
		ModifyWithRecreate: func(key string, oldValue, newValue *l3.ARPEntry, metadata interface{}) bool {
			return true
		},
		Dependencies:       d.Dependencies,
		Dump:               d.Dump,
		DumpDependencies:   []string{ifdescriptor.InterfaceDescriptorName},
	}
}

// EquivalentArps is comparison function for ARP entries.
func (d *ArpDescriptor) EquivalentArps(key string, oldArp, newArp *l3.ARPEntry) bool {
	return proto.Equal(oldArp, newArp)
}

// Add adds VPP ARP entry.
func (d *ArpDescriptor) Add(key string, arp *l3.ARPEntry) (interface{}, error) {
	if err := d.arpHandler.VppAddArp(arp); err != nil {
		return nil, err
	}
	return nil, nil
}

// Delete removes VPP ARP entry.
func (d *ArpDescriptor) Delete(key string, arp *l3.ARPEntry, metadata interface{}) error {
	if err := d.arpHandler.VppDelArp(arp); err != nil {
		return err
	}
	return nil
}

// Dependencies lists dependencies for a VPP ARP entry.
func (d *ArpDescriptor) Dependencies(key string, arp *l3.ARPEntry) (deps []kvs.Dependency) {
	// the outgoing interface must exist
	if arp.Interface != "" {
		deps = append(deps, kvs.Dependency{
			Label: arpEntryInterfaceDep,
			Key:   interfaces.InterfaceKey(arp.Interface),
		})
	}
	return deps
}

// Dump returns all ARP entries associated with interfaces managed by this agent.
func (d *ArpDescriptor) Dump(correlate []adapter.ARPEntryKVWithMetadata) (
	dump []adapter.ARPEntryKVWithMetadata, err error,
) {
	// Retrieve VPP ARP entries.
	arpEntries, err := d.arpHandler.DumpArpEntries()
	if err != nil {
		return nil, errors.Errorf("failed to dump VPP arps: %v", err)
	}

	for _, arp := range arpEntries {
		dump = append(dump, adapter.ARPEntryKVWithMetadata{
			Key:    l3.ArpEntryKey(arp.Arp.Interface, arp.Arp.IpAddress),
			Value:  arp.Arp,
			Origin: kvs.UnknownOrigin,
		})
	}

	return dump, nil
}
