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

import l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"

// L2Ctl L2 plugin related methods for vpp-agent-ctl (bridge domains, FIBs, L2 cross connects)
type L2Ctl interface {
	// PutBridgeDomain puts L2 bridge domain configuration to the ETCD
	PutBridgeDomain() error
	// DeleteBridgeDomain removes bridge domain configuration from the ETCD
	DeleteBridgeDomain() error
	// PutFib puts L2 FIB entry configuration to the ETCD
	PutFib() error
	// DeleteFib removes FIB entry configuration from the ETCD
	DeleteFib() error
	// PutXConn puts L2 cross connect configuration to the ETCD
	PutXConn() error
	// DeleteXConn removes cross connect configuration from the ETCD
	DeleteXConn() error
}

// PutBridgeDomain puts L2 bridge domain configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutBridgeDomain() error {
	bd := &l2.BridgeDomain{
		Name:                "bd1",
		Learn:               true,
		ArpTermination:      true,
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		MacAge:              0,
		Interfaces: []*l2.BridgeDomain_Interface{
			{
				Name: "loop1",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       0,
			},
			{
				Name: "tap1",
				BridgedVirtualInterface: false,
				SplitHorizonGroup:       1,
			},
			{
				Name: "memif1",
				BridgedVirtualInterface: false,
				SplitHorizonGroup:       2,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.50.20",
				PhysAddress: "A7:5D:44:D8:E6:51",
			},
		},
	}

	ctl.Log.Infof("Bridge domain put: %v", bd)
	return ctl.broker.Put(l2.BridgeDomainKey(bd.Name), bd)
}

// DeleteBridgeDomain removes bridge domain configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteBridgeDomain() error {
	bdKey := l2.BridgeDomainKey("bd1")

	ctl.Log.Infof("Bridge domain delete: %v", bdKey)
	_, err := ctl.broker.Delete(bdKey)
	return err
}

// PutFib puts L2 FIB entry configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutFib() error {
	fib := &l2.FIBEntry{
		PhysAddress:             "EA:FE:3C:64:A7:44",
		BridgeDomain:            "bd1",
		OutgoingInterface:       "loop1",
		StaticConfig:            true,
		BridgedVirtualInterface: true,
		Action:                  l2.FIBEntry_FORWARD, // or DROP

	}

	ctl.Log.Infof("FIB put: %v", fib)
	return ctl.broker.Put(l2.FIBKey(fib.BridgeDomain, fib.PhysAddress), fib)
}

// DeleteFib removes FIB entry configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteFib() error {
	fibKey := l2.FIBKey("bd1", "EA:FE:3C:64:A7:44")

	ctl.Log.Infof("FIB delete: %v", fibKey)
	_, err := ctl.broker.Delete(fibKey)
	return err
}

// PutXConn puts L2 cross connect configuration to the ETCD
func (ctl *VppAgentCtlImpl) PutXConn() error {
	xc := &l2.XConnectPair{
		ReceiveInterface:  "tap1",
		TransmitInterface: "loop1",
	}

	ctl.Log.Infof("FIB put: %v", xc)
	return ctl.broker.Put(l2.XConnectKey(xc.ReceiveInterface), xc)
}

// DeleteXConn removes cross connect configuration from the ETCD
func (ctl *VppAgentCtlImpl) DeleteXConn() error {
	xcKey := l2.XConnectKey("loop1")

	ctl.Log.Infof("FIB delete: %v", xcKey)
	_, err := ctl.broker.Delete(xcKey)
	return err
}
