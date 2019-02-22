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
	nat "github.com/ligato/vpp-agent/api/models/vpp/nat"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	vpp_ifdescriptor "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/natplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/natplugin/vppcalls"
)

const (
	// DNAT44DescriptorName is the name of the descriptor for VPP NAT44
	// Destination-NAT configurations.
	DNAT44DescriptorName = "vpp-nat44-dnat"

	// untaggedDNAT is used as a label for DNAT grouping all untagged static
	// and identity mappings.
	untaggedDNAT = "UNTAGGED-DNAT"

	// dependency labels
	mappingInterfaceDep = "interface-exists"
)

// A list of non-retriable errors:
var (
	// ErrDNAT44WithEmptyLabel is returned when NAT44 DNAT configuration is defined
	// with empty label
	ErrDNAT44WithEmptyLabel = errors.New("NAT44 DNAT configuration defined with empty label")
)

// DNAT44Descriptor teaches KVScheduler how to configure Destination NAT44 in VPP.
type DNAT44Descriptor struct {
	log        logging.Logger
	natHandler vppcalls.NatVppAPI
}

// NewDNAT44Descriptor creates a new instance of the DNAT44 descriptor.
func NewDNAT44Descriptor(natHandler vppcalls.NatVppAPI, log logging.PluginLogger) *DNAT44Descriptor {

	return &DNAT44Descriptor{
		natHandler: natHandler,
		log:        log.NewLogger("nat44-dnat-descriptor"),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *DNAT44Descriptor) GetDescriptor() *adapter.DNAT44Descriptor {
	return &adapter.DNAT44Descriptor{
		Name:               DNAT44DescriptorName,
		NBKeyPrefix:        nat.ModelDNat44.KeyPrefix(),
		ValueTypeName:      nat.ModelDNat44.ProtoName(),
		KeySelector:        nat.ModelDNat44.IsKeyValid,
		KeyLabel:           nat.ModelDNat44.StripKeyPrefix,
		ValueComparator:    d.EquivalentDNAT44,
		Validate:           d.Validate,
		Add:                d.Add,
		Delete:             d.Delete,
		Modify:             d.Modify,
		Dependencies:       d.Dependencies,
		Dump:               d.Dump,
		// dump interfaces and allocated IP addresses first
		DumpDependencies: []string{vpp_ifdescriptor.InterfaceDescriptorName, vpp_ifdescriptor.DHCPDescriptorName},
	}
}

// EquivalentDNAT44 compares two instances of DNAT44 for equality.
func (d *DNAT44Descriptor) EquivalentDNAT44(key string, oldDNAT, newDNAT *nat.DNat44) bool {
	// compare identity mappings
	obsoleteIDMappings, newIDMappings := diffIdentityMappings(oldDNAT.IdMappings, newDNAT.IdMappings)
	if len(obsoleteIDMappings) != 0 || len(newIDMappings) != 0 {
		return false
	}

	// compare static mappings
	obsoleteStMappings, newStMappings := diffStaticMappings(oldDNAT.StMappings, newDNAT.StMappings)
	return len(obsoleteStMappings) == 0 && len(newStMappings) == 0
}

// IsRetriableFailure returns <false> for errors related to invalid configuration.
func (d *DNAT44Descriptor) IsRetriableFailure(err error) bool {
	return err != ErrDNAT44WithEmptyLabel
}

// Validate validates VPP destination-NAT44 configuration.
func (d *DNAT44Descriptor) Validate(key string, dnat *nat.DNat44) error {
	if dnat.Label == "" {
		return kvs.NewInvalidValueError(ErrDNAT44WithEmptyLabel, "label")
	}
	return nil
}

// Add adds new destination-NAT44 configuration.
func (d *DNAT44Descriptor) Add(key string, dnat *nat.DNat44) (metadata interface{}, err error) {
	// Add = Modify from empty DNAT
	return d.Modify(key, &nat.DNat44{Label: dnat.Label}, dnat, nil)
}

// Delete removes existing destination-NAT44 configuration.
func (d *DNAT44Descriptor) Delete(key string, dnat *nat.DNat44, metadata interface{}) error {
	// Delete = Modify into empty DNAT
	_, err := d.Modify(key, dnat, &nat.DNat44{Label: dnat.Label}, metadata)
	return err
}

// Modify updates destination-NAT44 configuration.
func (d *DNAT44Descriptor) Modify(key string, oldDNAT, newDNAT *nat.DNat44, oldMetadata interface{}) (newMetadata interface{}, err error) {
	obsoleteIDMappings, newIDMappings := diffIdentityMappings(oldDNAT.IdMappings, newDNAT.IdMappings)
	obsoleteStMappings, newStMappings := diffStaticMappings(oldDNAT.StMappings, newDNAT.StMappings)

	// remove obsolete identity mappings
	for _, oldMapping := range obsoleteIDMappings {
		if err = d.natHandler.DelNat44IdentityMapping(oldMapping, oldDNAT.Label); err != nil {
			err = errors.Errorf("failed to remove identity mapping from DNAT %s: %v", oldDNAT.Label, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// remove obsolete static mappings
	for _, oldMapping := range obsoleteStMappings {
		if err = d.natHandler.DelNat44StaticMapping(oldMapping, oldDNAT.Label); err != nil {
			err = errors.Errorf("failed to remove static mapping from DNAT %s: %v", oldDNAT.Label, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// add new identity mappings
	for _, newMapping := range newIDMappings {
		if err = d.natHandler.AddNat44IdentityMapping(newMapping, newDNAT.Label); err != nil {
			err = errors.Errorf("failed to add identity mapping for DNAT %s: %v", newDNAT.Label, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// add new static mappings
	for _, newMapping := range newStMappings {
		if err = d.natHandler.AddNat44StaticMapping(newMapping, newDNAT.Label); err != nil {
			err = errors.Errorf("failed to add static mapping for DNAT %s: %v", newDNAT.Label, err)
			d.log.Error(err)
			return nil, err
		}
	}

	return nil, nil
}

// Dependencies lists external interfaces from mappings as dependencies.
func (d *DNAT44Descriptor) Dependencies(key string, dnat *nat.DNat44) (dependencies []kvs.Dependency) {
	// collect referenced external interfaces
	externalIfaces := make(map[string]struct{})
	for _, mapping := range dnat.StMappings {
		if mapping.ExternalInterface != "" {
			externalIfaces[mapping.ExternalInterface] = struct{}{}
		}
	}
	for _, mapping := range dnat.IdMappings {
		if mapping.Interface != "" {
			externalIfaces[mapping.Interface] = struct{}{}
		}
	}

	// for every external interface add one dependency
	for externalIface := range externalIfaces {
		dependencies = append(dependencies, kvs.Dependency{
			Label: mappingInterfaceDep + "-" + externalIface,
			Key:   interfaces.InterfaceKey(externalIface),
		})
	}
	return dependencies
}

// Dump returns the current NAT44 global configuration.
func (d *DNAT44Descriptor) Dump(correlate []adapter.DNAT44KVWithMetadata) (
	dump []adapter.DNAT44KVWithMetadata, err error,
) {
	// collect DNATs which are expected to be empty
	corrEmptyDNATs := make(map[string]*nat.DNat44)
	for _, kv := range correlate {
		if len(kv.Value.IdMappings) == 0 && len(kv.Value.StMappings) == 0 {
			corrEmptyDNATs[kv.Value.Label] = kv.Value
		}
	}

	// dump (non-empty) DNATs
	dnatDump, err := d.natHandler.DNat44Dump()
	if err != nil {
		d.log.Error(err)
		return dump, err
	}

	// process DNAT dump
	for _, dnat := range dnatDump {
		if dnat.Label == "" {
			// all untagged mappings are grouped under one DNAT with label <untaggedDNAT>
			// - they will get removed by resync (not configured by agent, or tagging has failed)
			dnat.Label = untaggedDNAT
		}
		if _, expectedToBeEmpty := corrEmptyDNATs[dnat.Label]; expectedToBeEmpty {
			// a DNAT mapping which is expected to be empty, but actually is not
			delete(corrEmptyDNATs, dnat.Label)
		}
		dump = append(dump, adapter.DNAT44KVWithMetadata{
			Key:    nat.DNAT44Key(dnat.Label),
			Value:  dnat,
			Origin: kvs.FromNB,
		})
	}

	// add empty DNATs (nothing from them is dumped)
	for dnatLabel, dnat := range corrEmptyDNATs {
		dump = append(dump, adapter.DNAT44KVWithMetadata{
			Key:    nat.DNAT44Key(dnatLabel),
			Value:  dnat,
			Origin: kvs.FromNB,
		})
	}

	return dump, nil
}

// diffIdentityMappings compares two *sets* of identity mappings.
func diffIdentityMappings(
	oldIDMappings, newIDMappings []*nat.DNat44_IdentityMapping) (obsoleteMappings, newMappings []*nat.DNat44_IdentityMapping) {

	for _, oldMapping := range oldIDMappings {
		found := false
		for _, newMapping := range newIDMappings {
			if proto.Equal(oldMapping, newMapping) {
				found = true
				break
			}
		}
		if !found {
			obsoleteMappings = append(obsoleteMappings, oldMapping)
		}
	}
	for _, newMapping := range newIDMappings {
		found := false
		for _, oldMapping := range oldIDMappings {
			if proto.Equal(oldMapping, newMapping) {
				found = true
				break
			}
		}
		if !found {
			newMappings = append(newMappings, newMapping)
		}
	}
	return obsoleteMappings, newMappings
}

// diffStaticMappings compares two *sets* of static mappings.
func diffStaticMappings(
	oldStMappings, newStMappings []*nat.DNat44_StaticMapping) (obsoleteMappings, newMappings []*nat.DNat44_StaticMapping) {

	for _, oldMapping := range oldStMappings {
		found := false
		for _, newMapping := range newStMappings {
			if equivalentStaticMappings(oldMapping, newMapping) {
				found = true
				break
			}
		}
		if !found {
			obsoleteMappings = append(obsoleteMappings, oldMapping)
		}
	}
	for _, newMapping := range newStMappings {
		found := false
		for _, oldMapping := range oldStMappings {
			if equivalentStaticMappings(oldMapping, newMapping) {
				found = true
				break
			}
		}
		if !found {
			newMappings = append(newMappings, newMapping)
		}
	}
	return obsoleteMappings, newMappings
}

// equivalentStaticMappings compares two static mappings for equality.
func equivalentStaticMappings(stMapping1, stMapping2 *nat.DNat44_StaticMapping) bool {
	// attributes compared as usually
	if stMapping1.Protocol != stMapping2.Protocol || stMapping1.ExternalPort != stMapping2.ExternalPort ||
		stMapping1.ExternalIp != stMapping2.ExternalIp || stMapping1.ExternalInterface != stMapping2.ExternalInterface ||
		stMapping1.TwiceNat != stMapping2.TwiceNat || stMapping1.SessionAffinity != stMapping1.SessionAffinity {
		return false
	}

	// compare locals ignoring their order
	for _, localIP1 := range stMapping1.LocalIps {
		found := false
		for _, localIP2 := range stMapping2.LocalIps {
			if proto.Equal(localIP1, localIP2) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, localIP2 := range stMapping2.LocalIps {
		found := false
		for _, localIP1 := range stMapping1.LocalIps {
			if proto.Equal(localIP1, localIP2) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
