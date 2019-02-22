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
	"fmt"
	"net"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/pkg/models"
	"github.com/pkg/errors"

	nat "github.com/ligato/vpp-agent/api/models/vpp/nat"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	vpp_ifdescriptor "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/vppv2/natplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/natplugin/vppcalls"
)

const (
	// NAT44GlobalDescriptorName is the name of the descriptor for VPP NAT44 global
	// configuration.
	NAT44GlobalDescriptorName = "vpp-nat44-global"

	// default virtual reassembly configuration
	natReassTimeoutDefault = 2 // seconds
	natMaxReassDefault     = 1024
	natMaxFragDefault      = 5
	natDropFragDefault     = false
)

// A list of non-retriable errors:
var (
	// ErrNATInterfaceFeatureCollision is returned when VPP NAT features assigned
	// to a single interface collide.
	ErrNATInterfaceFeatureCollision = errors.New("VPP NAT interface feature collision")

	// ErrDuplicateNATAddress is returned when VPP NAT address pool contains duplicate
	// IP addresses.
	ErrDuplicateNATAddress = errors.New("Duplicate VPP NAT address")

	// ErrInvalidNATAddress is returned when IP address from VPP NAT address pool
	// cannot be parsed.
	ErrInvalidNATAddress = errors.New("Invalid VPP NAT address")
)

// defaultGlobalCfg is the default NAT44 global configuration.
var defaultGlobalCfg = &nat.Nat44Global{
	VirtualReassembly: &nat.VirtualReassembly{
		Timeout:         natReassTimeoutDefault,
		MaxReassemblies: natMaxReassDefault,
		MaxFragments:    natMaxFragDefault,
		DropFragments:   natDropFragDefault,
	},
}

// NAT44GlobalDescriptor teaches KVScheduler how to configure global options for
// VPP NAT44.
type NAT44GlobalDescriptor struct {
	log        logging.Logger
	natHandler vppcalls.NatVppAPI
}

// NewNAT44GlobalDescriptor creates a new instance of the NAT44Global descriptor.
func NewNAT44GlobalDescriptor(natHandler vppcalls.NatVppAPI, log logging.PluginLogger) *NAT44GlobalDescriptor {

	return &NAT44GlobalDescriptor{
		natHandler: natHandler,
		log:        log.NewLogger("nat44-global-descriptor"),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *NAT44GlobalDescriptor) GetDescriptor() *adapter.NAT44GlobalDescriptor {
	return &adapter.NAT44GlobalDescriptor{
		Name:               NAT44GlobalDescriptorName,
		NBKeyPrefix:        nat.ModelNat44Global.KeyPrefix(),
		ValueTypeName:      nat.ModelNat44Global.ProtoName(),
		KeySelector:        nat.ModelNat44Global.IsKeyValid,
		ValueComparator:    d.EquivalentNAT44Global,
		Validate:           d.Validate,
		Add:                d.Add,
		Delete:             d.Delete,
		Modify:             d.Modify,
		DerivedValues:      d.DerivedValues,
		Dump:               d.Dump,
		DumpDependencies:   []string{vpp_ifdescriptor.InterfaceDescriptorName},
	}
}

// EquivalentNAT44Global compares two NAT44 global configs for equality.
func (d *NAT44GlobalDescriptor) EquivalentNAT44Global(key string, oldGlobalCfg, newGlobalCfg *nat.Nat44Global) bool {
	if oldGlobalCfg.Forwarding != newGlobalCfg.Forwarding {
		return false
	}
	if !proto.Equal(getVirtualReassembly(oldGlobalCfg), getVirtualReassembly(newGlobalCfg)) {
		return false
	}

	// Note: interfaces are not compared here as they are represented via derived kv-pairs

	// compare address pools
	obsoleteAddrs, newAddrs := diffNat44AddressPools(oldGlobalCfg.AddressPool, newGlobalCfg.AddressPool)
	return len(obsoleteAddrs) == 0 && len(newAddrs) == 0
}

// Validate validates VPP NAT44 global configuration.
func (d *NAT44GlobalDescriptor) Validate(key string, globalCfg *nat.Nat44Global) error {
	// check NAT interface features for collisions
	natIfaceMap := make(map[string]*natIface)
	for _, iface := range globalCfg.NatInterfaces {
		if _, hasEntry := natIfaceMap[iface.Name]; !hasEntry {
			natIfaceMap[iface.Name] = &natIface{}
		}
		ifaceCfg := natIfaceMap[iface.Name]
		if iface.IsInside {
			ifaceCfg.in++
		} else {
			ifaceCfg.out++
		}
		if iface.OutputFeature {
			ifaceCfg.output++
		}
	}
	natIfaceCollisionErr := kvs.NewInvalidValueError(ErrNATInterfaceFeatureCollision, "nat_interfaces")
	for _, ifaceCfg := range natIfaceMap {
		if ifaceCfg.in > 1 {
			// duplicate IN
			return natIfaceCollisionErr
		}
		if ifaceCfg.out > 1 {
			// duplicate OUT
			return natIfaceCollisionErr
		}
		if ifaceCfg.output == 1 && (ifaceCfg.in+ifaceCfg.out > 1) {
			// OUTPUT interface cannot be both IN and OUT
			return natIfaceCollisionErr
		}
	}

	// check NAT address pool for invalid addresses and duplicities
	var ipAddrs []net.IP
	for _, addr := range globalCfg.AddressPool {
		ipAddr := net.ParseIP(addr.Address)
		if ipAddr == nil {
			return kvs.NewInvalidValueError(ErrInvalidNATAddress,
				fmt.Sprintf("address_pool.address=%s", addr.Address))
		}
		for _, ipAddr2 := range ipAddrs {
			if ipAddr.Equal(ipAddr2) {
				return kvs.NewInvalidValueError(ErrDuplicateNATAddress,
					fmt.Sprintf("address_pool.address=%s", addr.Address))
			}
		}
		ipAddrs = append(ipAddrs, ipAddr)
	}
	return nil
}

// Add applies NAT44 global options.
func (d *NAT44GlobalDescriptor) Add(key string, globalCfg *nat.Nat44Global) (metadata interface{}, err error) {
	return d.Modify(key, defaultGlobalCfg, globalCfg, nil)
}

// Delete sets NAT44 global options back to the defaults.
func (d *NAT44GlobalDescriptor) Delete(key string, globalCfg *nat.Nat44Global, metadata interface{}) error {
	_, err := d.Modify(key, globalCfg, defaultGlobalCfg, metadata)
	return err
}

// Modify updates NAT44 global options.
func (d *NAT44GlobalDescriptor) Modify(key string, oldGlobalCfg, newGlobalCfg *nat.Nat44Global, oldMetadata interface{}) (newMetadata interface{}, err error) {
	// update forwarding
	if oldGlobalCfg.Forwarding != newGlobalCfg.Forwarding {
		if err = d.natHandler.SetNat44Forwarding(newGlobalCfg.Forwarding); err != nil {
			err = errors.Errorf("failed to set NAT44 forwarding to %t: %v", newGlobalCfg.Forwarding, err)
			d.log.Error(err)
			return nil, err
		}
	}

	// update virtual reassembly for IPv4
	if !proto.Equal(getVirtualReassembly(oldGlobalCfg), getVirtualReassembly(newGlobalCfg)) {
		if err = d.natHandler.SetVirtualReassemblyIPv4(getVirtualReassembly(newGlobalCfg)); err != nil {
			err = errors.Errorf("failed to set NAT virtual reassembly for IPv4: %v", err)
			d.log.Error(err)
			return nil, err
		}
	}

	// update the address pool
	obsoleteAddrs, newAddrs := diffNat44AddressPools(oldGlobalCfg.AddressPool, newGlobalCfg.AddressPool)
	// remove obsolete addresses from the pool
	for _, obsoleteAddr := range obsoleteAddrs {
		if err = d.natHandler.DelNat44Address(obsoleteAddr.Address, obsoleteAddr.VrfId, obsoleteAddr.TwiceNat); err != nil {
			err = errors.Errorf("failed to remove address %s from the NAT pool: %v", obsoleteAddr.Address, err)
			d.log.Error(err)
			return nil, err
		}
	}
	// add new addresses into the pool
	for _, newAddr := range newAddrs {
		if err = d.natHandler.AddNat44Address(newAddr.Address, newAddr.VrfId, newAddr.TwiceNat); err != nil {
			err = errors.Errorf("failed to add address %s into the NAT pool: %v", newAddr.Address, err)
			d.log.Error(err)
			return nil, err
		}
	}

	return nil, nil
}

// DerivedValues derives nat.NatInterface for every interface with assigned NAT configuration.
func (d *NAT44GlobalDescriptor) DerivedValues(key string, globalCfg *nat.Nat44Global) (derValues []kvs.KeyValuePair) {
	// NAT interfaces
	for _, natIface := range globalCfg.NatInterfaces {
		derValues = append(derValues, kvs.KeyValuePair{
			Key:   nat.InterfaceNAT44Key(natIface.Name, natIface.IsInside),
			Value: natIface,
		})
	}
	return derValues
}

// Dump returns the current NAT44 global configuration.
func (d *NAT44GlobalDescriptor) Dump(correlate []adapter.NAT44GlobalKVWithMetadata) ([]adapter.NAT44GlobalKVWithMetadata, error) {
	globalCfg, err := d.natHandler.Nat44GlobalConfigDump()
	if err != nil {
		d.log.Error(err)
		return nil, err
	}

	origin := kvs.FromNB
	if proto.Equal(globalCfg, defaultGlobalCfg) {
		origin = kvs.FromSB
	}

	dump := []adapter.NAT44GlobalKVWithMetadata{{
		Key:    models.Key(globalCfg),
		Value:  globalCfg,
		Origin: origin,
	}}

	return dump, nil
}

// natIface accumulates NAT interface configuration for validation purposes.
type natIface struct {
	// feature assignment counters
	in     int
	out    int
	output int
}

func getVirtualReassembly(globalCfg *nat.Nat44Global) *nat.VirtualReassembly {
	if globalCfg.VirtualReassembly == nil {
		return defaultGlobalCfg.VirtualReassembly
	}
	return globalCfg.VirtualReassembly
}

// diffNat44AddressPools compares two address pools.
func diffNat44AddressPools(oldAddrPool, newAddrPool []*nat.Nat44Global_Address) (obsoleteAddrs, newAddrs []*nat.Nat44Global_Address) {
	for _, oldAddr := range oldAddrPool {
		found := false
		for _, newAddr := range newAddrPool {
			if proto.Equal(oldAddr, newAddr) {
				found = true
				break
			}
		}
		if !found {
			obsoleteAddrs = append(obsoleteAddrs, oldAddr)
		}
	}
	for _, newAddr := range newAddrPool {
		found := false
		for _, oldAddr := range oldAddrPool {
			if proto.Equal(oldAddr, newAddr) {
				found = true
				break
			}
		}
		if !found {
			newAddrs = append(newAddrs, newAddr)
		}
	}
	return obsoleteAddrs, newAddrs
}
