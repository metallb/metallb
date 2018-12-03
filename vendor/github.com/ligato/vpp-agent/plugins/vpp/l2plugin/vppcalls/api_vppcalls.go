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
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// BridgeDomainVppAPI provides methods for managing bridge domains
type BridgeDomainVppAPI interface {
	BridgeDomainVppWrite
	BridgeDomainVppRead
}

// BridgeDomainVppWrite provides write methods for bridge domains
type BridgeDomainVppWrite interface {
	// VppAddBridgeDomain adds new bridge domain.
	VppAddBridgeDomain(bdIdx uint32, bd *l2.BridgeDomains_BridgeDomain) error
	// VppDeleteBridgeDomain removes existing bridge domain.
	VppDeleteBridgeDomain(bdIdx uint32) error
	// SetInterfaceToBridgeDomain sets single interface to bridge domain. Interface name is returned if configured.
	SetInterfaceToBridgeDomain(bdName string, bdIdx uint32, bdIf *l2.BridgeDomains_BridgeDomain_Interfaces,
		swIfIndices ifaceidx.SwIfIndex) (iface string, wasErr error)
	// SetInterfacesToBridgeDomain attempts to set all provided interfaces to bridge domain. It returns a list of interfaces
	// which were successfully set.
	SetInterfacesToBridgeDomain(bdName string, bdIdx uint32, bdIfs []*l2.BridgeDomains_BridgeDomain_Interfaces,
		swIfIndices ifaceidx.SwIfIndex) (ifs []string, wasErr error)
	// UnsetInterfacesFromBridgeDomain removes all interfaces from bridge domain. It returns a list of interfaces
	// which were successfully unset.
	UnsetInterfacesFromBridgeDomain(bdName string, bdIdx uint32, bdIfs []*l2.BridgeDomains_BridgeDomain_Interfaces,
		swIfIndices ifaceidx.SwIfIndex) (ifs []string, wasErr error)
	// VppAddArpTerminationTableEntry creates ARP termination entry for bridge domain.
	VppAddArpTerminationTableEntry(bdID uint32, mac string, ip string) error
	// VppRemoveArpTerminationTableEntry removes ARP termination entry from bridge domain
	VppRemoveArpTerminationTableEntry(bdID uint32, mac string, ip string) error
}

// BridgeDomainVppRead provides read methods for bridge domains
type BridgeDomainVppRead interface {
	// DumpBridgeDomainIDs lists all configured bridge domains. Auxiliary method for LookupFIBEntries.
	// returns list of bridge domain IDs (BD IDs). First element of returned slice is 0. It is default BD to which all
	// interfaces belong
	DumpBridgeDomainIDs() ([]uint32, error)
	// DumpBridgeDomains dumps VPP bridge domain data into the northbound API data structure
	// map indexed by bridge domain ID.
	//
	// LIMITATIONS:
	// - not able to dump ArpTerminationTable - missing binary API
	//
	DumpBridgeDomains() (map[uint32]*BridgeDomainDetails, error)
}

// FibVppAPI provides methods for managing FIBs
type FibVppAPI interface {
	FibVppWrite
	FibVppRead
}

// FibVppWrite provides write methods for FIBs
type FibVppWrite interface {
	// Add creates L2 FIB table entry.
	Add(mac string, bdID uint32, ifIdx uint32, bvi bool, static bool, callback func(error)) error
	// Delete removes existing L2 FIB table entry.
	Delete(mac string, bdID uint32, ifIdx uint32, callback func(error)) error
}

// FibVppRead provides read methods for FIBs
type FibVppRead interface {
	// DumpFIBTableEntries dumps VPP FIB table entries into the northbound API data structure
	// map indexed by destination MAC address.
	DumpFIBTableEntries() (map[string]*FibTableDetails, error)
	// WatchFIBReplies handles L2 FIB add/del requests
	WatchFIBReplies()
}

// XConnectVppAPI provides methods for managing cross connects
type XConnectVppAPI interface {
	XConnectVppWrite
	XConnectVppRead
}

// XConnectVppWrite provides write methods for cross connects
type XConnectVppWrite interface {
	// AddL2XConnect creates xConnect between two existing interfaces.
	AddL2XConnect(rxIfIdx uint32, txIfIdx uint32) error
	// DeleteL2XConnect removes xConnect between two interfaces.
	DeleteL2XConnect(rxIfIdx uint32, txIfIdx uint32) error
}

// XConnectVppRead provides read methods for cross connects
type XConnectVppRead interface {
	// DumpXConnectPairs dumps VPP xconnect pair data into the northbound API data structure
	// map indexed by rx interface index.
	DumpXConnectPairs() (map[uint32]*XConnectDetails, error)
}

// BridgeDomainVppHandler is accessor for bridge domain-related vppcalls methods
type BridgeDomainVppHandler struct {
	callsChannel govppapi.Channel
	ifIndexes    ifaceidx.SwIfIndex
	log          logging.Logger
}

// FibVppHandler is accessor for FIB-related vppcalls methods
type FibVppHandler struct {
	syncCallsChannel  govppapi.Channel
	asyncCallsChannel govppapi.Channel
	requestChan       chan *FibLogicalReq
	ifIndexes         ifaceidx.SwIfIndex
	bdIndexes         l2idx.BDIndex
	log               logging.Logger
}

// XConnectVppHandler is accessor for cross-connect-related vppcalls methods
type XConnectVppHandler struct {
	callsChannel govppapi.Channel
	ifIndexes    ifaceidx.SwIfIndex
	log          logging.Logger
}

// NewBridgeDomainVppHandler creates new instance of bridge domain vppcalls handler
func NewBridgeDomainVppHandler(callsChan govppapi.Channel, ifIndexes ifaceidx.SwIfIndex, log logging.Logger) *BridgeDomainVppHandler {
	return &BridgeDomainVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}

// NewFibVppHandler creates new instance of FIB vppcalls handler
func NewFibVppHandler(syncChan, asyncChan govppapi.Channel, ifIndexes ifaceidx.SwIfIndex, bdIndexes l2idx.BDIndex,
	log logging.Logger) *FibVppHandler {
	return &FibVppHandler{
		syncCallsChannel:  syncChan,
		asyncCallsChannel: asyncChan,
		requestChan:       make(chan *FibLogicalReq),
		ifIndexes:         ifIndexes,
		bdIndexes:         bdIndexes,
		log:               log,
	}
}

// NewXConnectVppHandler creates new instance of cross connect vppcalls handler
func NewXConnectVppHandler(callsChan govppapi.Channel, ifIndexes ifaceidx.SwIfIndex, log logging.Logger) *XConnectVppHandler {
	return &XConnectVppHandler{
		callsChannel: callsChan,
		ifIndexes:    ifIndexes,
		log:          log,
	}
}
