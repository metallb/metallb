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
	"strconv"

	"github.com/go-errors/errors"
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
 	"github.com/ligato/vpp-agent/plugins/vppv2/ipsecplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/vppv2/ipsecplugin/vppcalls"
)

const (
	// SPDPolicyDescriptorName is the name of the descriptor for bindings between
	// VPP IPSec security policy database and policy database (security association).
	SPDPolicyDescriptorName = "vpp-spd-policy"

	// dependency labels
	policyDep = "policy-exists"
)

// SPDPolicyDescriptor teaches KVScheduler how to put policy database into VPP
// security policy database
type SPDPolicyDescriptor struct {
	// dependencies
	log          logging.Logger
	ipSecHandler vppcalls.IPSecVppAPI
}

// NewSPDPolicyDescriptor creates a new instance of the SPDPolicy descriptor.
func NewSPDPolicyDescriptor(ipSecHandler vppcalls.IPSecVppAPI, log logging.PluginLogger) *SPDPolicyDescriptor {
	return &SPDPolicyDescriptor{
		log:          log.NewLogger("spd-policy-descriptor"),
		ipSecHandler: ipSecHandler,
	}
}

// GetDescriptor returns descriptor suitable for registration (via adapter) with
// the KVScheduler.
func (d *SPDPolicyDescriptor) GetDescriptor() *adapter.SPDPolicyDescriptor {
	return &adapter.SPDPolicyDescriptor{
		Name:               SPDPolicyDescriptorName,
		KeySelector:        d.IsSPDPolicyKey,
		ValueTypeName:      proto.MessageName(&ipsec.SecurityPolicyDatabase{}),
		Add:                d.Add,
		Delete:             d.Delete,
		ModifyWithRecreate: d.ModifyWithRecreate,
		Dependencies:       d.Dependencies,
	}
}

// IsSPDPolicyKey returns true if the key is identifying binding between
// VPP security policy database and security association within policy.
func (d *SPDPolicyDescriptor) IsSPDPolicyKey(key string) bool {
	_, _, isSPDPolicyKey := ipsec.ParseSPDPolicyKey(key)
	return isSPDPolicyKey
}

// Add puts policy into security policy database.
func (d *SPDPolicyDescriptor) Add(key string, policy *ipsec.SecurityPolicyDatabase_PolicyEntry) (metadata interface{}, err error) {
	// get security policy database index
	spdIdx, saIndex, isSPDPolicyKey := ipsec.ParseSPDPolicyKey(key)
	if !isSPDPolicyKey {
		err = errors.Errorf("provided key is not a derived SPD <=> Policy binding key %s", key)
		d.log.Error(err)
		return nil, err
	}

	// convert SPD to numeric index
	spdID, err := strconv.Atoi(spdIdx)
	if err != nil {
		err = errors.Errorf("provided SPD index is not a valid value %s", spdIdx)
		d.log.Error(err)
		return nil, err
	}

	// convert SA to numeric index
	saID, err := strconv.Atoi(saIndex)
	if err != nil {
		err = errors.Errorf("provided SA index is not a valid value %s", spdIdx)
		d.log.Error(err)
		return nil, err
	}

	// put policy into the security policy database
	err = d.ipSecHandler.AddSPDEntry(uint32(spdID), uint32(saID), policy)
	if err != nil {
		d.log.Error(err)
		return nil, err

	}
	return nil, nil
}

// Delete removes policy from security policy database.
func (d *SPDPolicyDescriptor) Delete(key string, policy *ipsec.SecurityPolicyDatabase_PolicyEntry, metadata interface{}) (err error) {
	// get security policy database index
	spdIdx, saIndex, isSPDPolicyKey := ipsec.ParseSPDPolicyKey(key)
	if !isSPDPolicyKey {
		err = errors.Errorf("provided key is not a derived SPD <=> Policy binding key %s", key)
		d.log.Error(err)
		return err
	}

	// convert SPD to numeric index
	spdID, err := strconv.Atoi(spdIdx)
	if err != nil {
		err = errors.Errorf("provided SPD index is not a valid value %s", spdIdx)
		d.log.Error(err)
		return err
	}

	// convert SA to numeric index
	saID, err := strconv.Atoi(saIndex)
	if err != nil {
		err = errors.Errorf("provided SA index is not a valid value %s", spdIdx)
		d.log.Error(err)
		return err
	}

	err = d.ipSecHandler.DeleteSPDEntry(uint32(spdID), uint32(saID), policy)
	if err != nil {
		d.log.Error(err)
		return err

	}
	return nil
}

// ModifyWithRecreate returns always true
func (d *SPDPolicyDescriptor) ModifyWithRecreate(key string, oldSPDPolicy, newSPDPolicy *ipsec.SecurityPolicyDatabase_PolicyEntry, metadata interface{}) bool {
	return true
}

// Dependencies lists the security association as the only dependency for the binding.
func (d *SPDPolicyDescriptor) Dependencies(key string, value *ipsec.SecurityPolicyDatabase_PolicyEntry) []kvs.Dependency {
	return []kvs.Dependency{
		{
			Label: policyDep,
			Key:   ipsec.SAKey(value.SaIndex),
		},
	}
}
