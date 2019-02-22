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

package data

import stn "github.com/ligato/vpp-agent/api/models/vpp/stn"

// StnCtl STN plugin related methods for vpp-agent-ctl
type StnCtl interface {
	// PutStn puts STN configuration to the ETCD
	PutStn() error
	// DeleteStn removes STN configuration from the ETCD
	DeleteStn() error
}

// PutStn puts STN configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutStn() error {
	stnRule := &stn.Rule{
		IpAddress: "192.168.50.12",
		Interface: "memif1",
	}

	ctl.Log.Infof("STN put: %v", stnRule)
	return ctl.broker.Put(stn.Key(stnRule.Interface, stnRule.IpAddress), stnRule)
}

// DeleteStn removes STN configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteStn() error {
	stnRuleKey := stn.Key("memif1", "192.168.50.12")

	ctl.Log.Infof("STN delete: %v", stnRuleKey)
	_, err := ctl.broker.Delete(stnRuleKey)
	return err
}
