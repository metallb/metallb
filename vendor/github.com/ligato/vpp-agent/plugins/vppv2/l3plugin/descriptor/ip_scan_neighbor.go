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

	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	"github.com/ligato/vpp-agent/pkg/models"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
)

const (
	// IPScanNeighborDescriptorName is the name of the descriptor.
	IPScanNeighborDescriptorName = "vpp-ip-scan-neighbor"
)

const (
	defaultScanInterval   = 1
	defaultMaxProcTime    = 20
	defaultMaxUpdate      = 10
	defaultScanIntDelay   = 1
	defaultStaleThreshold = 4
)

var defaultIPScanNeighbor = &l3.IPScanNeighbor{
	Mode:           l3.IPScanNeighbor_DISABLED,
	ScanInterval:   defaultScanInterval,
	MaxProcTime:    defaultMaxProcTime,
	MaxUpdate:      defaultMaxUpdate,
	ScanIntDelay:   defaultScanIntDelay,
	StaleThreshold: defaultStaleThreshold,
}

// IPScanNeighborDescriptor teaches KVScheduler how to configure VPP proxy ARPs.
type IPScanNeighborDescriptor struct {
	log       logging.Logger
	ipNeigh   vppcalls.IPNeighVppAPI
	scheduler kvs.KVScheduler
}

// NewIPScanNeighborDescriptor creates a new instance of the IPScanNeighborDescriptor.
func NewIPScanNeighborDescriptor(scheduler kvs.KVScheduler,
	proxyArpHandler vppcalls.IPNeighVppAPI, log logging.PluginLogger) *IPScanNeighborDescriptor {

	return &IPScanNeighborDescriptor{
		scheduler: scheduler,
		ipNeigh:   proxyArpHandler,
		log:       log.NewLogger("ip-scan-neigh-descriptor"),
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *IPScanNeighborDescriptor) GetDescriptor() *adapter.IPScanNeighborDescriptor {
	return &adapter.IPScanNeighborDescriptor{
		Name:               IPScanNeighborDescriptorName,
		NBKeyPrefix:        l3.ModelIPScanNeighbor.KeyPrefix(),
		ValueTypeName:      l3.ModelIPScanNeighbor.ProtoName(),
		KeySelector:        l3.ModelIPScanNeighbor.IsKeyValid,
		ValueComparator:    d.EquivalentIPScanNeighbors,
		Add:                d.Add,
		Modify:             d.Modify,
		Delete:             d.Delete,
		Dump:               d.Dump,
	}
}

// EquivalentIPScanNeighbors compares the IP Scan Neighbor values.
func (d *IPScanNeighborDescriptor) EquivalentIPScanNeighbors(key string, oldValue, newValue *l3.IPScanNeighbor) bool {
	return proto.Equal(withDefaults(oldValue), withDefaults(newValue))
}

// Add adds VPP IP Scan Neighbor.
func (d *IPScanNeighborDescriptor) Add(key string, value *l3.IPScanNeighbor) (metadata interface{}, err error) {
	return d.Modify(key, defaultIPScanNeighbor, value, nil)
}

// Delete deletes VPP IP Scan Neighbor.
func (d *IPScanNeighborDescriptor) Delete(key string, value *l3.IPScanNeighbor, metadata interface{}) error {
	_, err := d.Modify(key, value, defaultIPScanNeighbor, metadata)
	return err
}

// Modify modifies VPP IP Scan Neighbor.
func (d *IPScanNeighborDescriptor) Modify(key string, oldValue, newValue *l3.IPScanNeighbor, oldMetadata interface{}) (newMetadata interface{}, err error) {
	if err := d.ipNeigh.SetIPScanNeighbor(newValue); err != nil {
		return nil, err
	}
	return nil, nil
}

// Dump dumps VPP IP Scan Neighbor.
func (d *IPScanNeighborDescriptor) Dump(correlate []adapter.IPScanNeighborKVWithMetadata) (
	dump []adapter.IPScanNeighborKVWithMetadata, err error,
) {
	// Retrieve VPP configuration
	ipNeigh, err := d.ipNeigh.GetIPScanNeighbor()
	if err != nil {
		return nil, err
	}
	fillDefaults(ipNeigh)

	var origin = kvs.FromNB
	if proto.Equal(ipNeigh, defaultIPScanNeighbor) {
		origin = kvs.FromSB
	}

	dump = append(dump, adapter.IPScanNeighborKVWithMetadata{
		Key:    models.Key(ipNeigh),
		Value:  ipNeigh,
		Origin: origin,
	})

	return dump, nil
}
func withDefaults(orig *l3.IPScanNeighbor) *l3.IPScanNeighbor {
	var val = *orig
	if val.ScanInterval == 0 {
		val.ScanInterval = defaultScanInterval
	}
	if val.MaxProcTime == 0 {
		val.MaxProcTime = defaultMaxProcTime
	}
	if val.MaxUpdate == 0 {
		val.MaxUpdate = defaultMaxUpdate
	}
	if val.ScanIntDelay == 0 {
		val.ScanIntDelay = defaultScanIntDelay
	}
	if val.StaleThreshold == 0 {
		val.StaleThreshold = defaultStaleThreshold
	}
	return &val
}

func fillDefaults(orig *l3.IPScanNeighbor) {
	var val = orig
	if val.ScanInterval == 0 {
		val.ScanInterval = defaultScanInterval
	}
	if val.MaxProcTime == 0 {
		val.MaxProcTime = defaultMaxProcTime
	}
	if val.MaxUpdate == 0 {
		val.MaxUpdate = defaultMaxUpdate
	}
	if val.ScanIntDelay == 0 {
		val.ScanIntDelay = defaultScanIntDelay
	}
	if val.StaleThreshold == 0 {
		val.StaleThreshold = defaultStaleThreshold
	}
}
