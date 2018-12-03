// Copyright (c) 2018 Bell Canada, Pantheon Technologies and/or its affiliates.
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

package srplugin

import (
	"github.com/go-errors/errors"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
)

// NamedPolicySegment represents named policy segment.
// TODO: move unique identifiable name into srv6 models
type NamedPolicySegment struct {
	Name    string /* unique identifiable name */
	Segment *srv6.PolicySegment
}

// NamedSteering represents named steetring.
type NamedSteering struct {
	Name     string /* unique identifiable name */
	Steering *srv6.Steering
}

// Resync writes missing segment routing configs to the VPP and removes obsolete ones.
func (c *SRv6Configurator) Resync(localSids []*srv6.LocalSID, policies []*srv6.Policy, namedSegments []*NamedPolicySegment, namedSteerings []*NamedSteering) error {
	// Re-initialize cache
	c.clearMapping()

	// TODO: dump existing configuration from VPP, compare it with etcd and change vpp according to etcd (now is handled only case when VPP is clean, i.e. from starting)

	for _, localsid := range localSids {
		if err := c.AddLocalSID(localsid); err != nil {
			return errors.Errorf("sr resync error: failed to add local sid %s: %v", localsid, err)
		}
	}

	for _, policy := range policies {
		if err := c.AddPolicy(policy); err != nil {
			return errors.Errorf("sr resync error: failed to add policy %s: %v", policy.GetBsid(), err)
		}
	}

	for _, namedSegment := range namedSegments {
		if err := c.AddPolicySegment(namedSegment.Name, namedSegment.Segment); err != nil {
			return errors.Errorf("sr resync error: failed to add policy segment %s: %v", namedSegment.Name, err)
		}
	}

	for _, namedSteering := range namedSteerings {
		if err := c.AddSteering(namedSteering.Name, namedSteering.Steering); err != nil {
			return errors.Errorf("sr resync error: failed to add steering %s: %v", namedSteering.Name, err)
		}
	}

	c.log.Info("Segment routing resync done")
	return nil
}
