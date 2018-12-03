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

package vpp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/ipsec"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l4"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
	"github.com/ligato/vpp-agent/plugins/vpp/model/srv6"
	"github.com/ligato/vpp-agent/plugins/vpp/model/stn"
	"github.com/ligato/vpp-agent/plugins/vpp/srplugin"
)

// DataResyncReq is used to transfer expected configuration of the VPP to the plugins.
type DataResyncReq struct {
	// ACLs is a list af all access lists that are expected to be in VPP after RESYNC.
	ACLs []*acl.AccessLists_Acl
	// Interfaces is a list af all interfaces that are expected to be in VPP after RESYNC.
	Interfaces []*interfaces.Interfaces_Interface
	// SingleHopBFDSession is a list af all BFD sessions that are expected to be in VPP after RESYNC.
	SingleHopBFDSession []*bfd.SingleHopBFD_Session
	// SingleHopBFDKey is a list af all BFD authentication keys that are expected to be in VPP after RESYNC.
	SingleHopBFDKey []*bfd.SingleHopBFD_Key
	// SingleHopBFDEcho is a list af all BFD echo functions that are expected to be in VPP after RESYNC.
	SingleHopBFDEcho []*bfd.SingleHopBFD_EchoFunction
	// BridgeDomains is a list af all BDs that are expected to be in VPP after RESYNC.
	BridgeDomains []*l2.BridgeDomains_BridgeDomain
	// FibTableEntries is a list af all FIBs that are expected to be in VPP after RESYNC.
	FibTableEntries []*l2.FibTable_FibEntry
	// XConnects is a list af all XCons that are expected to be in VPP after RESYNC.
	XConnects []*l2.XConnectPairs_XConnectPair
	// StaticRoutes is a list af all Static Routes that are expected to be in VPP after RESYNC.
	StaticRoutes []*l3.StaticRoutes_Route
	// ArpEntries is a list af all ARP entries that are expected to be in VPP after RESYNC.
	ArpEntries []*l3.ArpTable_ArpEntry
	// ProxyArpInterfaces is a list af all proxy ARP interface entries that are expected to be in VPP after RESYNC.
	ProxyArpInterfaces []*l3.ProxyArpInterfaces_InterfaceList
	// ProxyArpRanges is a list af all proxy ARP ranges that are expected to be in VPP after RESYNC.
	ProxyArpRanges []*l3.ProxyArpRanges_RangeList
	// IPScanNeigh is a IP scan neighbor config that is expected to be set in VPP after RESYNC.
	IPScanNeigh *l3.IPScanNeighbor
	// L4Features is a bool flag that is expected to be set in VPP after RESYNC.
	L4Features *l4.L4Features
	// AppNamespaces is a list af all App Namespaces that are expected to be in VPP after RESYNC.
	AppNamespaces []*l4.AppNamespaces_AppNamespace
	// StnRules is a list of all STN Rules that are expected to be in VPP after RESYNC
	StnRules []*stn.STN_Rule
	// NatGlobal is a definition of global NAT config
	Nat44Global *nat.Nat44Global
	// Nat44SNat is a list of all SNAT configurations expected to be in VPP after RESYNC
	Nat44SNat []*nat.Nat44SNat_SNatConfig
	// Nat44DNat is a list of all DNAT configurations expected to be in VPP after RESYNC
	Nat44DNat []*nat.Nat44DNat_DNatConfig
	// IPSecSPDs is a list of all IPSec Security Policy Databases expected to be in VPP after RESYNC
	IPSecSPDs []*ipsec.SecurityPolicyDatabases_SPD
	// IPSecSAs is a list of all IPSec Security Associations expected to be in VPP after RESYNC
	IPSecSAs []*ipsec.SecurityAssociations_SA
	// IPSecTunnels is a list of all IPSec Tunnel interfaces expected to be in VPP after RESYNC
	IPSecTunnels []*ipsec.TunnelInterfaces_Tunnel
	// LocalSids is a list of all segment routing local SIDs expected to be in VPP after RESYNC
	LocalSids []*srv6.LocalSID
	// SrPolicies is a list of all segment routing policies expected to be in VPP after RESYNC
	SrPolicies []*srv6.Policy
	// SrPolicySegments is a list of all segment routing policy segments (with identifiable name) expected to be in VPP after RESYNC
	SrPolicySegments []*srplugin.NamedPolicySegment
	// SrSteerings is a list of all segment routing steerings (with identifiable name) expected to be in VPP after RESYNC
	SrSteerings []*srplugin.NamedSteering
}

// NewDataResyncReq is a constructor.
func NewDataResyncReq() *DataResyncReq {
	return &DataResyncReq{
		ACLs:                []*acl.AccessLists_Acl{},
		Interfaces:          []*interfaces.Interfaces_Interface{},
		SingleHopBFDSession: []*bfd.SingleHopBFD_Session{},
		SingleHopBFDKey:     []*bfd.SingleHopBFD_Key{},
		SingleHopBFDEcho:    []*bfd.SingleHopBFD_EchoFunction{},
		BridgeDomains:       []*l2.BridgeDomains_BridgeDomain{},
		FibTableEntries:     []*l2.FibTable_FibEntry{},
		XConnects:           []*l2.XConnectPairs_XConnectPair{},
		StaticRoutes:        []*l3.StaticRoutes_Route{},
		ArpEntries:          []*l3.ArpTable_ArpEntry{},
		ProxyArpInterfaces:  []*l3.ProxyArpInterfaces_InterfaceList{},
		ProxyArpRanges:      []*l3.ProxyArpRanges_RangeList{},
		IPScanNeigh:         &l3.IPScanNeighbor{},
		L4Features:          &l4.L4Features{},
		AppNamespaces:       []*l4.AppNamespaces_AppNamespace{},
		StnRules:            []*stn.STN_Rule{},
		Nat44Global:         &nat.Nat44Global{},
		Nat44SNat:           []*nat.Nat44SNat_SNatConfig{},
		Nat44DNat:           []*nat.Nat44DNat_DNatConfig{},
		IPSecSPDs:           []*ipsec.SecurityPolicyDatabases_SPD{},
		IPSecSAs:            []*ipsec.SecurityAssociations_SA{},
		IPSecTunnels:        []*ipsec.TunnelInterfaces_Tunnel{},
		LocalSids:           []*srv6.LocalSID{},
		SrPolicies:          []*srv6.Policy{},
		SrPolicySegments:    []*srplugin.NamedPolicySegment{},
		SrSteerings:         []*srplugin.NamedSteering{},
	}
}

// The function delegates resync request to ifplugin/l2plugin/l3plugin resync requests (in this particular order).
func (plugin *Plugin) resyncConfigPropageFullRequest(req *DataResyncReq) error {
	plugin.Log.Info("resync the VPP Configuration begin")
	startTime := time.Now()
	defer func() {
		vppResync := time.Since(startTime)
		plugin.Log.WithField("durationInNs", vppResync.Nanoseconds()).Infof("resync the VPP Configuration end in %v", vppResync)
	}()

	return plugin.resyncConfig(req)
}

// delegates optimize-cold-stasrt resync request
func (plugin *Plugin) resyncConfigPropageOptimizedRequest(req *DataResyncReq) error {
	plugin.Log.Info("resync the VPP Configuration begin")
	startTime := time.Now()
	defer func() {
		vppResync := time.Since(startTime)
		plugin.Log.WithField("durationInNs", vppResync.Nanoseconds()).Infof("resync the VPP Configuration end in %v", vppResync)
	}()

	// If the strategy is optimize-cold-start, run interface configurator resync which provides the information
	// whether resync should continue or be terminated
	stopResync := plugin.ifConfigurator.VerifyVPPConfigPresence(req.Interfaces)
	if stopResync {
		// terminate the resync operation
		return nil
	}
	// continue resync normally
	return plugin.resyncConfig(req)
}

// The function delegates resync request to ifplugin/l2plugin/l3plugin resync requests (in this particular order).
func (plugin *Plugin) resyncConfig(req *DataResyncReq) error {
	// store all resync errors
	var resyncErrs []error

	if !plugin.droppedFromResync(interfaces.Prefix) {
		if err := plugin.ifConfigurator.Resync(req.Interfaces); err != nil {
			resyncErrs = append(resyncErrs, plugin.ifConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(acl.Prefix) {
		if err := plugin.aclConfigurator.Resync(req.ACLs); err != nil {
			resyncErrs = append(resyncErrs, plugin.aclConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(bfd.AuthKeysPrefix) {
		if err := plugin.bfdConfigurator.ResyncAuthKey(req.SingleHopBFDKey); err != nil {
			resyncErrs = append(resyncErrs, plugin.bfdConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(bfd.SessionPrefix) {
		if err := plugin.bfdConfigurator.ResyncSession(req.SingleHopBFDSession); err != nil {
			resyncErrs = append(resyncErrs, plugin.bfdConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(bfd.EchoFunctionPrefix) {
		if err := plugin.bfdConfigurator.ResyncEchoFunction(req.SingleHopBFDEcho); err != nil {
			resyncErrs = append(resyncErrs, plugin.bfdConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l2.BdPrefix) {
		if err := plugin.bdConfigurator.Resync(req.BridgeDomains); err != nil {
			resyncErrs = append(resyncErrs, plugin.bdConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l2.FibPrefix) {
		if err := plugin.fibConfigurator.Resync(req.FibTableEntries); err != nil {
			resyncErrs = append(resyncErrs, err)
		}
	}
	if !plugin.droppedFromResync(l2.XConnectPrefix) {
		if err := plugin.xcConfigurator.Resync(req.XConnects); err != nil {
			resyncErrs = append(resyncErrs, plugin.xcConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l3.RoutesPrefix) {
		if err := plugin.routeConfigurator.Resync(req.StaticRoutes); err != nil {
			resyncErrs = append(resyncErrs, plugin.routeConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l3.ArpPrefix) {
		if err := plugin.arpConfigurator.Resync(req.ArpEntries); err != nil {
			resyncErrs = append(resyncErrs, plugin.arpConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l3.ProxyARPInterfacePrefix) {
		if err := plugin.proxyArpConfigurator.ResyncInterfaces(req.ProxyArpInterfaces); err != nil {
			resyncErrs = append(resyncErrs, plugin.proxyArpConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l3.ProxyARPRangePrefix) {
		if err := plugin.proxyArpConfigurator.ResyncRanges(req.ProxyArpRanges); err != nil {
			resyncErrs = append(resyncErrs, plugin.proxyArpConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l3.IPScanNeighPrefix) {
		if err := plugin.ipNeighConfigurator.Resync(req.IPScanNeigh); err != nil {
			resyncErrs = append(resyncErrs, plugin.ipNeighConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l4.FeaturesPrefix) {
		if err := plugin.appNsConfigurator.ResyncFeatures(req.L4Features); err != nil {
			resyncErrs = append(resyncErrs, plugin.appNsConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(l4.NamespacesPrefix) {
		if err := plugin.appNsConfigurator.ResyncAppNs(req.AppNamespaces); err != nil {
			resyncErrs = append(resyncErrs, plugin.appNsConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(stn.Prefix) {
		if err := plugin.stnConfigurator.Resync(req.StnRules); err != nil {
			resyncErrs = append(resyncErrs, plugin.stnConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(nat.GlobalPrefix) {
		if err := plugin.natConfigurator.ResyncNatGlobal(req.Nat44Global); err != nil {
			resyncErrs = append(resyncErrs, plugin.natConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(nat.SNatPrefix) {
		if err := plugin.natConfigurator.ResyncSNat(req.Nat44SNat); err != nil {
			resyncErrs = append(resyncErrs, plugin.natConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(nat.DNatPrefix) {
		if err := plugin.natConfigurator.ResyncDNat(req.Nat44DNat); err != nil {
			resyncErrs = append(resyncErrs, plugin.natConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(ipsec.KeyPrefix) {
		if err := plugin.ipSecConfigurator.Resync(req.IPSecSPDs, req.IPSecSAs, req.IPSecTunnels); err != nil {
			resyncErrs = append(resyncErrs, plugin.ipSecConfigurator.LogError(err))
		}
	}
	if !plugin.droppedFromResync(srv6.BasePrefix()) {
		if err := plugin.srv6Configurator.Resync(req.LocalSids, req.SrPolicies, req.SrPolicySegments, req.SrSteerings); err != nil {
			resyncErrs = append(resyncErrs, plugin.srv6Configurator.LogError(err))
		}
	}
	// log errors if any
	if len(resyncErrs) == 0 {
		return nil
	}
	for i, err := range resyncErrs {
		plugin.Log.Errorf("resync error #%d: %v", i, err)
	}
	return fmt.Errorf("%v errors occured during vppplugin resync", len(resyncErrs))
}

func (plugin *Plugin) resyncParseEvent(resyncEv datasync.ResyncEvent) *DataResyncReq {
	req := NewDataResyncReq()
	for key, resyncData := range resyncEv.GetValues() {
		plugin.Log.Debugf("Received RESYNC key %q", key)
		if plugin.droppedFromResync(key) {
			continue
		}
		if strings.HasPrefix(key, acl.Prefix) {
			plugin.appendACLInterface(resyncData, req)
		} else if strings.HasPrefix(key, interfaces.Prefix) {
			plugin.appendResyncInterface(resyncData, req)
		} else if strings.HasPrefix(key, bfd.SessionPrefix) {
			plugin.resyncAppendBfdSession(resyncData, req)
		} else if strings.HasPrefix(key, bfd.AuthKeysPrefix) {
			plugin.resyncAppendBfdAuthKeys(resyncData, req)
		} else if strings.HasPrefix(key, bfd.EchoFunctionPrefix) {
			plugin.resyncAppendBfdEcho(resyncData, req)
		} else if strings.HasPrefix(key, l2.BdPrefix) {
			plugin.resyncAppendBDs(resyncData, req)
		} else if strings.HasPrefix(key, l2.XConnectPrefix) {
			plugin.resyncAppendXCons(resyncData, req)
		} else if strings.HasPrefix(key, l3.VrfPrefix) {
			plugin.resyncAppendVRFs(resyncData, req)
		} else if strings.HasPrefix(key, l3.ArpPrefix) {
			plugin.resyncAppendARPs(resyncData, req)
		} else if strings.HasPrefix(key, l3.ProxyARPInterfacePrefix) {
			plugin.resyncAppendProxyArpInterfaces(resyncData, req)
		} else if strings.HasPrefix(key, l3.IPScanNeighPrefix) {
			plugin.resyncAppendIPScanNeighs(resyncData, req)
		} else if strings.HasPrefix(key, l3.ProxyARPRangePrefix) {
			plugin.resyncAppendProxyArpRanges(resyncData, req)
		} else if strings.HasPrefix(key, l4.FeaturesPrefix) {
			plugin.resyncFeatures(resyncData, req)
		} else if strings.HasPrefix(key, l4.NamespacesPrefix) {
			plugin.resyncAppendAppNs(resyncData, req)
		} else if strings.HasPrefix(key, stn.Prefix) {
			plugin.appendResyncStnRules(resyncData, req)
		} else if strings.HasPrefix(key, nat.GlobalPrefix) {
			plugin.resyncNatGlobal(resyncData, req)
		} else if strings.HasPrefix(key, nat.SNatPrefix) {
			plugin.appendResyncSNat(resyncData, req)
		} else if strings.HasPrefix(key, nat.DNatPrefix) {
			plugin.appendResyncDNat(resyncData, req)
		} else if strings.HasPrefix(key, ipsec.KeyPrefix) {
			plugin.appendResyncIPSec(resyncData, req)
		} else if strings.HasPrefix(key, srv6.BasePrefix()) {
			plugin.appendResyncSR(resyncData, req)
		} else {
			plugin.Log.Warnf("ignoring prefix %q by VPP standard plugins", key)
		}
	}
	return req
}
func (plugin *Plugin) droppedFromResync(key string) bool {
	for _, prefix := range plugin.omittedPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func (plugin *Plugin) resyncAppendARPs(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if arpData, stop := resyncData.GetNext(); stop {
			break
		} else {
			entry := &l3.ArpTable_ArpEntry{}
			if err := arpData.GetValue(entry); err != nil {
				plugin.Log.Errorf("error getting value of ARP: %v", err)
				continue
			}
			req.ArpEntries = append(req.ArpEntries, entry)
			num++

			plugin.Log.WithField("revision", arpData.GetRevision()).
				Debugf("Processing resync for key: %q", arpData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC ARP values %d", num)
}

func (plugin *Plugin) resyncAppendProxyArpInterfaces(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if arpData, stop := resyncData.GetNext(); stop {
			break
		} else {
			entry := &l3.ProxyArpInterfaces_InterfaceList{}
			if err := arpData.GetValue(entry); err != nil {
				plugin.Log.Errorf("error getting value of proxy ARP: %v", err)
				continue
			}
			req.ProxyArpInterfaces = append(req.ProxyArpInterfaces, entry)
			num++

			plugin.Log.WithField("revision", arpData.GetRevision()).
				Debugf("Processing resync for key: %q", arpData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC proxy ARP values %d", num)
}

func (plugin *Plugin) resyncAppendIPScanNeighs(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		ipScan, stop := resyncData.GetNext()
		if stop {
			break
		}
		entry := &l3.IPScanNeighbor{}
		if err := ipScan.GetValue(entry); err != nil {
			plugin.Log.Errorf("error getting value of IP scan neigh: %v", err)
			continue
		}
		req.IPScanNeigh = entry
		num++

		plugin.Log.WithField("revision", ipScan.GetRevision()).
			Debugf("Processing resync for key: %q", ipScan.GetKey())
	}

	plugin.Log.Debugf("Received RESYNC IP scan neigh values %d", num)
}

func (plugin *Plugin) resyncAppendProxyArpRanges(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if arpData, stop := resyncData.GetNext(); stop {
			break
		} else {
			entry := &l3.ProxyArpRanges_RangeList{}
			if err := arpData.GetValue(entry); err != nil {
				plugin.Log.Errorf("error getting value of proxy ARP ranges: %v", err)
				continue
			}
			req.ProxyArpRanges = append(req.ProxyArpRanges, entry)
			num++

			plugin.Log.WithField("revision", arpData.GetRevision()).
				Debugf("Processing resync for key: %q", arpData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC proxy ARP ranges %d ", num)
}

func (plugin *Plugin) resyncAppendL3FIB(fibData datasync.KeyVal, vrfIndex string, req *DataResyncReq) error {
	route := &l3.StaticRoutes_Route{}
	err := fibData.GetValue(route)
	if err != nil {
		return err
	}
	// Ensure every route has the corresponding VRF index.
	intVrfKeyIndex, err := strconv.Atoi(vrfIndex)
	if err != nil {
		return err
	}
	if vrfIndex != strconv.Itoa(int(route.VrfId)) {
		plugin.Log.Warnf("Resync: VRF index from key (%v) and from config (%v) does not match, using value from the key",
			intVrfKeyIndex, route.VrfId)
		route.VrfId = uint32(intVrfKeyIndex)
	}
	req.StaticRoutes = append(req.StaticRoutes, route)

	plugin.Log.WithField("revision", fibData.GetRevision()).
		Debugf("Processing resync for key: %q", fibData.GetKey())

	return nil
}

func (plugin *Plugin) resyncAppendVRFs(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	numL3FIBs := 0
	for {
		if vrfData, stop := resyncData.GetNext(); stop {
			break
		} else {
			key := vrfData.GetKey()
			fib, vrfIndex, _, _, _ := l3.ParseRouteKey(key)
			if fib {
				if err := plugin.resyncAppendL3FIB(vrfData, vrfIndex, req); err != nil {
					plugin.Log.Errorf("error resyncing L3FIB: %v", err)
					continue
				}
				numL3FIBs++

				plugin.Log.WithField("revision", vrfData.GetRevision()).
					Debugf("Processing resync for key: %q", vrfData.GetKey())
			} else {
				plugin.Log.Warn("VRF RESYNC is not implemented")
			}
		}
	}

	plugin.Log.Debugf("Received RESYNC L3 FIB values %d", numL3FIBs)
}

func (plugin *Plugin) resyncAppendXCons(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if xConnectData, stop := resyncData.GetNext(); stop {
			break
		} else {
			value := &l2.XConnectPairs_XConnectPair{}
			if err := xConnectData.GetValue(value); err != nil {
				plugin.Log.Errorf("error getting value of XConnect: %v", err)
				continue
			}
			req.XConnects = append(req.XConnects, value)
			num++

			plugin.Log.WithField("revision", xConnectData.GetRevision()).
				Debugf("Processing resync for key: %q", xConnectData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC XConnects values %d", num)
}
func (plugin *Plugin) resyncAppendL2FIB(fibData datasync.KeyVal, req *DataResyncReq) error {
	value := &l2.FibTable_FibEntry{}
	if err := fibData.GetValue(value); err != nil {
		return fmt.Errorf("error getting value of L2FIB: %v", err)
	}
	req.FibTableEntries = append(req.FibTableEntries, value)

	plugin.Log.WithField("revision", fibData.GetRevision()).
		Debugf("Processing resync for key: %q", fibData.GetKey())

	return nil
}

func (plugin *Plugin) resyncAppendBDs(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	numBDs := 0
	numL2FIBs := 0
	for {
		if bridgeDomainData, stop := resyncData.GetNext(); stop {
			break
		} else {
			key := bridgeDomainData.GetKey()
			fib, _, _ := l2.ParseFibKey(key)
			if fib {
				if err := plugin.resyncAppendL2FIB(bridgeDomainData, req); err != nil {
					plugin.Log.Errorf("error resyncing L2FIB: %v", err)
					continue
				}
				numL2FIBs++

			} else {
				value := &l2.BridgeDomains_BridgeDomain{}
				if err := bridgeDomainData.GetValue(value); err != nil {
					plugin.Log.Errorf("error getting value of bridge domain: %v", err)
					continue
				}
				req.BridgeDomains = append(req.BridgeDomains, value)
				numBDs++

			}

			plugin.Log.WithField("revision", bridgeDomainData.GetRevision()).
				Debugf("Processing resync for key: %q", bridgeDomainData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC BD values %d", numBDs)
	plugin.Log.Debugf("Received RESYNC L2 FIB values %d", numL2FIBs)
}

func (plugin *Plugin) resyncAppendBfdEcho(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if bfdData, stop := resyncData.GetNext(); stop {
			break
		} else {
			bfdEcho := &bfd.SingleHopBFD_EchoFunction{}
			if err := bfdData.GetValue(bfdEcho); err != nil {
				plugin.Log.Errorf("error getting value of BFD echo function: %v", err)
				continue
			}
			req.SingleHopBFDEcho = append(req.SingleHopBFDEcho, bfdEcho)
			num++

			plugin.Log.WithField("revision", bfdData.GetRevision()).
				Debugf("Processing resync for key: %q", bfdData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC BFD Echo values %d", num)
}

func (plugin *Plugin) resyncAppendBfdAuthKeys(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if bfdData, stop := resyncData.GetNext(); stop {
			break
		} else {
			bfdKey := &bfd.SingleHopBFD_Key{}
			if err := bfdData.GetValue(bfdKey); err != nil {
				plugin.Log.Errorf("error getting value of BFD auth key: %v", err)
				continue
			}
			req.SingleHopBFDKey = append(req.SingleHopBFDKey, bfdKey)
			num++

			plugin.Log.WithField("revision", bfdData.GetRevision()).
				Debugf("Processing resync for key: %q", bfdData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC BFD auth keys %d", num)
}

func (plugin *Plugin) resyncAppendBfdSession(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if bfdData, stop := resyncData.GetNext(); stop {
			break
		} else {
			bfdSession := &bfd.SingleHopBFD_Session{}
			if err := bfdData.GetValue(bfdSession); err != nil {
				plugin.Log.Errorf("error getting value of BFD session: %v", err)
				continue
			}
			req.SingleHopBFDSession = append(req.SingleHopBFDSession, bfdSession)
			num++

			plugin.Log.WithField("revision", bfdData.GetRevision()).
				Debugf("Processing resync for key: %q", bfdData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC BFD auth sessions %d", num)
}

func (plugin *Plugin) appendACLInterface(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if data, stop := resyncData.GetNext(); stop {
			break
		} else {
			aclData := &acl.AccessLists_Acl{}
			if err := data.GetValue(aclData); err != nil {
				plugin.Log.Errorf("error getting value of ACL: %v", err)
				continue
			}
			req.ACLs = append(req.ACLs, aclData)
			num++

			plugin.Log.WithField("revision", data.GetRevision()).
				Debugf("Processing resync for key: %q", data.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC ACLs %d", num)
}

func (plugin *Plugin) appendResyncInterface(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if interfaceData, stop := resyncData.GetNext(); stop {
			break
		} else {
			ifData := &interfaces.Interfaces_Interface{}
			if err := interfaceData.GetValue(ifData); err != nil {
				plugin.Log.Errorf("error getting value of interface: %v", err)
				continue
			}
			req.Interfaces = append(req.Interfaces, ifData)
			num++

			plugin.Log.WithField("revision", interfaceData.GetRevision()).
				Debugf("Processing resync for key: %q", interfaceData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC interfaces %d", num)
}

func (plugin *Plugin) resyncFeatures(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		appResyncData, stop := resyncData.GetNext()
		if stop {
			break
		}
		value := &l4.L4Features{}
		if err := appResyncData.GetValue(value); err != nil {
			plugin.Log.Errorf("error getting value of L4 features: %v", err)
			continue
		}
		req.L4Features = value
		num++

		plugin.Log.WithField("revision", appResyncData.GetRevision()).
			Debugf("Processing resync for key: %q", appResyncData.GetKey())
	}

	plugin.Log.Debugf("Received RESYNC L4 features %d", num)
}

func (plugin *Plugin) resyncAppendAppNs(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if appResyncData, stop := resyncData.GetNext(); stop {
			break
		} else {
			value := &l4.AppNamespaces_AppNamespace{}
			if err := appResyncData.GetValue(value); err != nil {
				plugin.Log.Errorf("error getting value of App namespaces: %v", err)
				continue
			}
			req.AppNamespaces = append(req.AppNamespaces, value)
			num++

			plugin.Log.WithField("revision", appResyncData.GetRevision()).
				Debugf("Processing resync for key: %q", appResyncData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC app namespaces %d", num)
}

func (plugin *Plugin) appendResyncStnRules(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if stnData, stop := resyncData.GetNext(); stop {
			break
		} else {
			value := &stn.STN_Rule{}
			if err := stnData.GetValue(value); err != nil {
				plugin.Log.Errorf("error getting value of STN rules: %v", err)
				continue
			}
			req.StnRules = append(req.StnRules, value)
			num++

			plugin.Log.WithField("revision", stnData.GetRevision()).
				Debugf("Processing resync for key: %q", stnData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC STN data %d", num)
}

func (plugin *Plugin) resyncNatGlobal(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		natGlobalData, stop := resyncData.GetNext()
		if stop {
			break
		}
		value := &nat.Nat44Global{}
		if err := natGlobalData.GetValue(value); err != nil {
			plugin.Log.Errorf("error getting value of NAT global: %v", err)
			continue
		}
		req.Nat44Global = value
		num++

		plugin.Log.WithField("revision", natGlobalData.GetRevision()).
			Debugf("Processing resync for key: %q", natGlobalData.GetKey())
	}

	plugin.Log.Debugf("Received RESYNC NAT global %d", num)
}

func (plugin *Plugin) appendResyncSNat(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if sNatData, stop := resyncData.GetNext(); stop {
			break
		} else {
			value := &nat.Nat44SNat_SNatConfig{}
			if err := sNatData.GetValue(value); err != nil {
				plugin.Log.Errorf("error getting value of SNAT: %v", err)
				continue
			}
			req.Nat44SNat = append(req.Nat44SNat, value)
			num++

			plugin.Log.WithField("revision", sNatData.GetRevision()).
				Debugf("Processing resync for key: %q", sNatData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC SNAT global %d", num)
}

func (plugin *Plugin) appendResyncDNat(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if dNatData, stop := resyncData.GetNext(); stop {
			break
		} else {
			value := &nat.Nat44DNat_DNatConfig{}
			if err := dNatData.GetValue(value); err != nil {
				plugin.Log.Errorf("error getting value of DNAT: %v", err)
				continue
			}
			req.Nat44DNat = append(req.Nat44DNat, value)
			num++

			plugin.Log.WithField("revision", dNatData.GetRevision()).
				Debugf("Processing resync for key: %q", dNatData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC DNAT global %d", num)
}

func (plugin *Plugin) appendResyncIPSec(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if data, stop := resyncData.GetNext(); stop {
			break
		} else {
			if strings.HasPrefix(data.GetKey(), ipsec.KeyPrefixSPD) {
				value := &ipsec.SecurityPolicyDatabases_SPD{}
				if err := data.GetValue(value); err != nil {
					plugin.Log.Errorf("error getting value of IPSec SPD: %v", err)
					continue
				}
				req.IPSecSPDs = append(req.IPSecSPDs, value)
				num++
			} else if strings.HasPrefix(data.GetKey(), ipsec.KeyPrefixSA) {
				value := &ipsec.SecurityAssociations_SA{}
				if err := data.GetValue(value); err != nil {
					plugin.Log.Errorf("error getting value of IPSec SA: %v", err)
					continue
				}
				req.IPSecSAs = append(req.IPSecSAs, value)
				num++
			} else if strings.HasPrefix(data.GetKey(), ipsec.KeyPrefixTunnel) {
				value := &ipsec.TunnelInterfaces_Tunnel{}
				if err := data.GetValue(value); err != nil {
					plugin.Log.Errorf("error getting value of IPSec tunnel: %v", err)
					continue
				}
				req.IPSecTunnels = append(req.IPSecTunnels, value)
				num++
			}

			plugin.Log.WithField("revision", data.GetRevision()).
				Debugf("Processing resync for key: %q", data.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC IPSec configs %d", num)
}

func (plugin *Plugin) appendResyncSR(resyncData datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if data, stop := resyncData.GetNext(); stop {
			break
		} else {
			if strings.HasPrefix(data.GetKey(), srv6.LocalSIDPrefix()) {
				value := &srv6.LocalSID{}
				if err := data.GetValue(value); err != nil {
					plugin.Log.Errorf("error getting value of SR sid: %v", err)
					continue
				}
				req.LocalSids = append(req.LocalSids, value)
				num++
			} else if strings.HasPrefix(data.GetKey(), srv6.PolicyPrefix()) {
				if srv6.IsPolicySegmentPrefix(data.GetKey()) { //Policy segment
					value := &srv6.PolicySegment{}
					if err := data.GetValue(value); err != nil {
						plugin.Log.Errorf("error getting value of SR policy segment: %v", err)
						continue
					}
					if name, err := srv6.ParsePolicySegmentKey(data.GetKey()); err != nil {
						plugin.Log.Errorf("failed to parse SR policy segment %s: %v", data.GetKey(), err)
						continue
					} else {
						req.SrPolicySegments = append(req.SrPolicySegments, &srplugin.NamedPolicySegment{Name: name, Segment: value})
						num++
					}
				} else { // Policy
					value := &srv6.Policy{}
					if err := data.GetValue(value); err != nil {
						plugin.Log.Errorf("error getting value of SR policy: %v", err)
						continue
					}
					req.SrPolicies = append(req.SrPolicies, value)
					num++
				}
			} else if strings.HasPrefix(data.GetKey(), srv6.SteeringPrefix()) {
				value := &srv6.Steering{}
				if err := data.GetValue(value); err != nil {
					plugin.Log.Errorf("error getting value of SR steering: %v", err)
					continue
				}
				req.SrSteerings = append(req.SrSteerings, &srplugin.NamedSteering{Name: strings.TrimPrefix(data.GetKey(), srv6.SteeringPrefix()), Steering: value})
				num++
			}

			plugin.Log.WithField("revision", data.GetRevision()).
				Debugf("Processing resync for key: %q", data.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC SR configs %d", num)
}

// All registration for above channel select (it ensures proper order during initialization) are put here.
func (plugin *Plugin) subscribeWatcher() (err error) {
	plugin.Log.Debug("subscribeWatcher begin")
	plugin.swIfIndexes.WatchNameToIdx(plugin.String(), plugin.ifIdxWatchCh)
	plugin.Log.Debug("swIfIndexes watch registration finished")
	plugin.bdIndexes.WatchNameToIdx(plugin.String(), plugin.bdIdxWatchCh)
	plugin.Log.Debug("bdIndexes watch registration finished")
	if plugin.Linux != nil {
		// Get pointer to the map with Linux interface indexes.
		linuxIfIndexes := plugin.Linux.GetLinuxIfIndexes()
		if linuxIfIndexes == nil {
			return fmt.Errorf("linux plugin enabled but interface indexes are not available")
		}
		linuxIfIndexes.WatchNameToIdx(plugin.String(), plugin.linuxIfIdxWatchCh)
		plugin.Log.Debug("linuxIfIndexes watch registration finished")
	}

	plugin.watchConfigReg, err = plugin.Watcher.
		Watch("Config VPP default plug:IF/L2/L3", plugin.changeChan, plugin.resyncConfigChan,
			acl.Prefix,
			interfaces.Prefix,
			bfd.SessionPrefix,
			bfd.AuthKeysPrefix,
			bfd.EchoFunctionPrefix,
			l2.BdPrefix,
			l2.XConnectPrefix,
			l3.VrfPrefix,
			l3.ArpPrefix,
			l3.ProxyARPInterfacePrefix,
			l3.ProxyARPRangePrefix,
			l3.IPScanNeighPrefix,
			l4.FeaturesPrefix,
			l4.NamespacesPrefix,
			stn.Prefix,
			nat.GlobalPrefix,
			nat.SNatPrefix,
			nat.DNatPrefix,
			ipsec.KeyPrefix,
			srv6.BasePrefix(),
		)
	if err != nil {
		return err
	}

	plugin.watchStatusReg, err = plugin.Watcher.
		Watch("Status VPP default plug:IF/L2/L3", nil, plugin.resyncStatusChan,
			interfaces.StatePrefix, l2.BdStatePrefix)
	if err != nil {
		return err
	}

	plugin.Log.Debug("data Transport watch finished")

	return nil
}
