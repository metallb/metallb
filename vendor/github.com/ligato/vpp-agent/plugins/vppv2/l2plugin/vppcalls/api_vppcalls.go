// Copyright (c) 2017 Cisco and/or its affiliates.
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

package vppcalls

import (
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	"github.com/ligato/vpp-agent/pkg/idxvpp2"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
)

// BridgeDomainVppAPI provides methods for managing bridge domains.
type BridgeDomainVppAPI interface {
	BridgeDomainVppWrite
	BridgeDomainVppRead
}

// BridgeDomainVppWrite provides write methods for bridge domains.
type BridgeDomainVppWrite interface {
	// AddBridgeDomain adds new bridge domain.
	AddBridgeDomain(bdIdx uint32, bd *l2.BridgeDomain) error
	// DeleteBridgeDomain removes existing bridge domain.
	DeleteBridgeDomain(bdIdx uint32) error
	// AddInterfaceToBridgeDomain puts interface into bridge domain.
	AddInterfaceToBridgeDomain(bdIdx uint32, ifaceCfg *l2.BridgeDomain_Interface) error
	// DeleteInterfaceFromBridgeDomain removes interface from bridge domain.
	DeleteInterfaceFromBridgeDomain(bdIdx uint32, ifaceCfg *l2.BridgeDomain_Interface) error
	// AddArpTerminationTableEntry creates ARP termination entry for bridge domain.
	AddArpTerminationTableEntry(bdID uint32, mac string, ip string) error
	// RemoveArpTerminationTableEntry removes ARP termination entry from bridge domain.
	RemoveArpTerminationTableEntry(bdID uint32, mac string, ip string) error
}

// BridgeDomainVppRead provides read methods for bridge domains.
type BridgeDomainVppRead interface {
	// DumpBridgeDomains dumps VPP bridge domain data into the northbound API data structure
	// map indexed by bridge domain ID.
	DumpBridgeDomains() ([]*BridgeDomainDetails, error)
}

// FIBVppAPI provides methods for managing FIBs.
type FIBVppAPI interface {
	FIBVppWrite
	FIBVppRead
}

// FIBVppWrite provides write methods for FIBs.
type FIBVppWrite interface {
	// AddL2FIB creates L2 FIB table entry.
	AddL2FIB(fib *l2.FIBEntry) error
	// DeleteL2FIB removes existing L2 FIB table entry.
	DeleteL2FIB(fib *l2.FIBEntry) error
}

// FIBVppRead provides read methods for FIBs.
type FIBVppRead interface {
	// DumpL2FIBs dumps VPP L2 FIB table entries into the northbound API
	// data structure map indexed by destination MAC address.
	DumpL2FIBs() (map[string]*FibTableDetails, error)
}

// XConnectVppAPI provides methods for managing cross connects.
type XConnectVppAPI interface {
	XConnectVppWrite
	XConnectVppRead
}

// XConnectVppWrite provides write methods for cross connects.
type XConnectVppWrite interface {
	// AddL2XConnect creates xConnect between two existing interfaces.
	AddL2XConnect(rxIface, txIface string) error
	// DeleteL2XConnect removes xConnect between two interfaces.
	DeleteL2XConnect(rxIface, txIface string) error
}

// XConnectVppRead provides read methods for cross connects.
type XConnectVppRead interface {
	// DumpXConnectPairs dumps VPP xconnect pair data into the northbound API
	// data structure map indexed by rx interface index.
	DumpXConnectPairs() (map[uint32]*XConnectDetails, error)
}

// BridgeDomainVppHandler is accessor for bridge domain-related vppcalls methods.
type BridgeDomainVppHandler struct {
	callsChannel govppapi.Channel
	ifIndexes    ifaceidx.IfaceMetadataIndex
	log          logging.Logger
}

// FIBVppHandler is accessor for FIB-related vppcalls methods.
type FIBVppHandler struct {
	callsChannel govppapi.Channel
	ifIndexes    ifaceidx.IfaceMetadataIndex
	bdIndexes    idxvpp2.NameToIndex
	log          logging.Logger
}

// XConnectVppHandler is accessor for cross-connect-related vppcalls methods.
type XConnectVppHandler struct {
	callsChannel govppapi.Channel
	ifIndexes    ifaceidx.IfaceMetadataIndex
	log          logging.Logger
}

// NewBridgeDomainVppHandler creates new instance of bridge domain vppcalls handler.
func NewBridgeDomainVppHandler(callsChan govppapi.Channel, ifIndexes ifaceidx.IfaceMetadataIndex, log logging.Logger) *BridgeDomainVppHandler {
	return &BridgeDomainVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}

// NewFIBVppHandler creates new instance of FIB vppcalls handler.
func NewFIBVppHandler(callsChan govppapi.Channel, ifIndexes ifaceidx.IfaceMetadataIndex, bdIndexes idxvpp2.NameToIndex,
	log logging.Logger) *FIBVppHandler {
	return &FIBVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		bdIndexes:    bdIndexes,
		log:          log,
	}
}

// NewXConnectVppHandler creates new instance of cross connect vppcalls handler.
func NewXConnectVppHandler(callsChan govppapi.Channel, ifIndexes ifaceidx.IfaceMetadataIndex, log logging.Logger) *XConnectVppHandler {
	return &XConnectVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}
