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

import (
	punt "github.com/ligato/vpp-agent/api/models/vpp/punt"
)

// PuntCtl punt plugin related methods for vpp-agent-ctl (bridge domains, FIBs, L2 cross connects)
type PuntCtl interface {
	// PutPunt puts punt configuration to the ETCD
	PutPunt() error
	// DeletePunt removes punt configuration from the ETCD
	DeletePunt() error
	// RegisterPuntViaSocket registers punt via socket to the ETCD
	RegisterPuntViaSocket() error
	// DeregisterPuntViaSocket removes punt socket registration from the ETCD
	DeregisterPuntViaSocket() error
	// PutIPRedirect puts IP redirect configuration to the ETCD
	PutIPRedirect() error
	// DeleteIPRedirect removes IP redirect from the ETCD
	DeleteIPRedirect() error
}

// PutPunt puts punt configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutPunt() error {
	puntCfg := &punt.ToHost{
		L3Protocol: punt.L3Protocol_IPv4,
		L4Protocol: punt.L4Protocol_UDP,
		Port:       9000,
	}

	ctl.Log.Info("Punt put: %v", puntCfg)
	return ctl.broker.Put(punt.ToHostKey(puntCfg.L3Protocol, puntCfg.L4Protocol, puntCfg.Port), puntCfg)
}

// DeletePunt removes punt configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeletePunt() error {
	puntKey := punt.ToHostKey(punt.L3Protocol_IPv4, punt.L4Protocol_UDP, 9000)

	ctl.Log.Info("Punt delete: %v", puntKey)
	_, err := ctl.broker.Delete(puntKey)
	return err
}

// RegisterPuntViaSocket registers punt via socket to the ETCD
func (ctl *VppAgentCtlImpl) RegisterPuntViaSocket() error {
	puntCfg := &punt.ToHost{
		L3Protocol: punt.L3Protocol_IPv4,
		L4Protocol: punt.L4Protocol_UDP,
		Port:       9000,
		SocketPath: "/tmp/socket/path",
	}

	ctl.Log.Info("Punt via socket put: %v", puntCfg)
	return ctl.broker.Put(punt.ToHostKey(puntCfg.L3Protocol, puntCfg.L4Protocol, puntCfg.Port), puntCfg)
}

// DeregisterPuntViaSocket removes punt socket registration from the ETCD
func (ctl *VppAgentCtlImpl) DeregisterPuntViaSocket() error {
	puntKey := punt.ToHostKey(punt.L3Protocol_IPv4, punt.L4Protocol_UDP, 9000)

	ctl.Log.Info("Punt via socket delete: %v", puntKey)
	_, err := ctl.broker.Delete(puntKey)
	return err
}

// PutIPRedirect puts IP redirect configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutIPRedirect() error {
	puntCfg := &punt.IPRedirect{
		L3Protocol:  punt.L3Protocol_IPv4,
		TxInterface: "tap1",
		NextHop:     "192.168.0.1",
	}

	ctl.Log.Info("IP redirect put: %v", puntCfg)
	return ctl.broker.Put(punt.IPRedirectKey(puntCfg.L3Protocol, puntCfg.TxInterface), puntCfg)
}

// DeleteIPRedirect removes IP redirect from the ETCD
func (ctl *VppAgentCtlImpl) DeleteIPRedirect() error {
	puntKey := punt.IPRedirectKey(punt.L3Protocol_IPv4, "tap1")

	ctl.Log.Info("IP redirect delete: %v", puntKey)
	_, err := ctl.broker.Delete(puntKey)
	return err
}
