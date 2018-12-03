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

package rpc

import (
	"fmt"

	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/rpc/grpc"
	"github.com/ligato/vpp-agent/clientv1/linux"
	"github.com/ligato/vpp-agent/clientv1/linux/localclient"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/rpc"
	"golang.org/x/net/context"
)

// Plugin registers VPP GRPC services in *grpc.Server.
type Plugin struct {
	Deps

	// Services
	changeVppSvc ChangeVppSvc
	resyncVppSvc ResyncVppSvc
	notifSvc     NotificationSvc
}

// Deps - dependencies of Plugin
type Deps struct {
	infra.PluginDeps
	GRPCServer grpc.Server
}

// ChangeVppSvc forwards GRPC request to the localclient.
type ChangeVppSvc struct {
	log logging.Logger
}

// ResyncVppSvc forwards GRPC request to the localclient.
type ResyncVppSvc struct {
	log logging.Logger
}

// Init sets plugin child loggers for changeVppSvc & resyncVppSvc.
func (plugin *Plugin) Init() error {
	// Data change
	plugin.changeVppSvc.log = plugin.Log.NewLogger("changeVppSvc")
	// Data resync
	plugin.resyncVppSvc.log = plugin.Log.NewLogger("resyncVppSvc")
	// Notification service (represents GRPC client)
	plugin.notifSvc.log = plugin.Log.NewLogger("notifSvc")

	// Register all GRPC services if server is available. Register needs to be done
	// before 'ListenAndServe' is called in GRPC plugin
	grpcServer := plugin.GRPCServer.GetServer()
	if grpcServer != nil {
		rpc.RegisterDataChangeServiceServer(grpcServer, &plugin.changeVppSvc)
		rpc.RegisterDataResyncServiceServer(grpcServer, &plugin.resyncVppSvc)
		rpc.RegisterNotificationServiceServer(grpcServer, &plugin.notifSvc)
	}

	return nil
}

// Close does nothing.
func (plugin *Plugin) Close() error {
	return nil
}

// UpdateNotifications stores new notification data
func (plugin *Plugin) UpdateNotifications(ctx context.Context, notification *interfaces.InterfaceNotification) {
	if notification == nil {
		return
	}
	plugin.notifSvc.updateNotifications(ctx, notification)
}

// Put adds configuration data present in data request to the VPP/Linux
func (svc *ChangeVppSvc) Put(ctx context.Context, data *rpc.DataRequest) (*rpc.PutResponse, error) {
	request := localclient.DataChangeRequest("rpc").Put()
	if err := processRequest(ctx, data, request); err != nil {
		return nil, err
	}
	err := request.Send().ReceiveReply()
	return &rpc.PutResponse{}, err
}

// Del removes configuration data present in data request from the VPP/linux
func (svc *ChangeVppSvc) Del(ctx context.Context, data *rpc.DataRequest) (*rpc.DelResponse, error) {
	request := localclient.DataChangeRequest("rpc").Delete()
	if err := processRequest(ctx, data, request); err != nil {
		return nil, err
	}
	err := request.Send().ReceiveReply()
	return &rpc.DelResponse{}, err
}

// Resync creates a resync request which adds data tp the VPP/linux
func (svc *ResyncVppSvc) Resync(ctx context.Context, data *rpc.DataRequest) (*rpc.ResyncResponse, error) {
	request := localclient.DataResyncRequest("rpc")
	if err := processRequest(ctx, data, request); err != nil {
		return nil, err
	}
	err := request.Send().ReceiveReply()
	return &rpc.ResyncResponse{}, err
}

// Common method which puts or deletes data of every configuration type separately
func processRequest(ctx context.Context, data *rpc.DataRequest, request interface{}) error {
	switch r := request.(type) {
	case linuxclient.PutDSL:
		for _, aclItem := range data.AccessLists {
			r.ACL(aclItem)
		}
		for _, ifItem := range data.Interfaces {
			r.VppInterface(ifItem)
		}
		for _, sessionItem := range data.BfdSessions {
			r.BfdSession(sessionItem)
		}
		for _, keyItem := range data.BfdAuthKeys {
			r.BfdAuthKeys(keyItem)
		}
		if data.BfdEchoFunction != nil {
			r.BfdEchoFunction(data.BfdEchoFunction)
		}
		for _, bdItem := range data.BridgeDomains {
			r.BD(bdItem)
		}
		for _, fibItem := range data.FIBs {
			r.BDFIB(fibItem)
		}
		for _, xcItem := range data.XCons {
			r.XConnect(xcItem)
		}
		for _, rtItem := range data.StaticRoutes {
			r.StaticRoute(rtItem)
		}
		for _, arpItem := range data.ArpEntries {
			r.Arp(arpItem)
		}
		for _, paiItem := range data.ProxyArpInterfaces {
			r.ProxyArpInterfaces(paiItem)
		}
		for _, parItem := range data.ProxyArpRanges {
			r.ProxyArpRanges(parItem)
		}
		if data.L4Feature != nil {
			r.L4Features(data.L4Feature)
		}
		for _, anItem := range data.ApplicationNamespaces {
			r.AppNamespace(anItem)
		}
		for _, stnItem := range data.StnRules {
			r.StnRule(stnItem)
		}
		if data.NatGlobal != nil {
			r.NAT44Global(data.NatGlobal)
		}
		for _, natItem := range data.DNATs {
			r.NAT44DNat(natItem)
		}
		for _, ifItem := range data.LinuxInterfaces {
			r.LinuxInterface(ifItem)
		}
		for _, arpItem := range data.LinuxArpEntries {
			r.LinuxArpEntry(arpItem)
		}
		for _, rtItem := range data.LinuxRoutes {
			r.LinuxRoute(rtItem)
		}
	case linuxclient.DeleteDSL:
		for _, aclItem := range data.AccessLists {
			r.ACL(aclItem.AclName)
		}
		for _, ifItem := range data.Interfaces {
			r.VppInterface(ifItem.Name)
		}
		for _, sessionItem := range data.BfdSessions {
			r.BfdSession(sessionItem.Interface)
		}
		for _, keyItem := range data.BfdAuthKeys {
			r.BfdAuthKeys(keyItem.Name)
		}
		if data.BfdEchoFunction != nil {
			r.BfdEchoFunction(data.BfdEchoFunction.Name)
		}
		for _, bdItem := range data.BridgeDomains {
			r.BD(bdItem.Name)
		}
		for _, fibItem := range data.FIBs {
			r.BDFIB(fibItem.BridgeDomain, fibItem.PhysAddress)
		}
		for _, xcItem := range data.XCons {
			r.XConnect(xcItem.ReceiveInterface)
		}
		for _, rtItem := range data.StaticRoutes {
			r.StaticRoute(rtItem.VrfId, rtItem.DstIpAddr, rtItem.NextHopAddr)
		}
		for _, arpItem := range data.ArpEntries {
			r.Arp(arpItem.Interface, arpItem.IpAddress)
		}
		for _, paiItem := range data.ProxyArpInterfaces {
			r.ProxyArpInterfaces(paiItem.Label)
		}
		for _, parItem := range data.ProxyArpRanges {
			r.ProxyArpRanges(parItem.Label)
		}
		if data.L4Feature != nil {
			r.L4Features()
		}
		for _, anItem := range data.ApplicationNamespaces {
			r.AppNamespace(anItem.NamespaceId)
		}
		for _, stnItem := range data.StnRules {
			r.StnRule(stnItem.RuleName)
		}
		if data.NatGlobal != nil {
			r.NAT44Global()
		}
		for _, natItem := range data.DNATs {
			r.NAT44DNat(natItem.Label)
		}
		for _, ifItem := range data.LinuxInterfaces {
			r.LinuxInterface(ifItem.Name)
		}
		for _, arpItem := range data.LinuxArpEntries {
			r.LinuxArpEntry(arpItem.Name)
		}
		for _, rtItem := range data.LinuxRoutes {
			r.LinuxRoute(rtItem.Name)
		}
	case linuxclient.DataResyncDSL:
		for _, aclItem := range data.AccessLists {
			r.ACL(aclItem)
		}
		for _, ifItem := range data.Interfaces {
			r.VppInterface(ifItem)
		}
		for _, sessionItem := range data.BfdSessions {
			r.BfdSession(sessionItem)
		}
		for _, keyItem := range data.BfdAuthKeys {
			r.BfdAuthKeys(keyItem)
		}
		if data.BfdEchoFunction != nil {
			r.BfdEchoFunction(data.BfdEchoFunction)
		}
		for _, bdItem := range data.BridgeDomains {
			r.BD(bdItem)
		}
		for _, fibItem := range data.FIBs {
			r.BDFIB(fibItem)
		}
		for _, xcItem := range data.XCons {
			r.XConnect(xcItem)
		}
		for _, rtItem := range data.StaticRoutes {
			r.StaticRoute(rtItem)
		}
		for _, arpItem := range data.ArpEntries {
			r.Arp(arpItem)
		}
		for _, paiItem := range data.ProxyArpInterfaces {
			r.ProxyArpInterfaces(paiItem)
		}
		for _, parItem := range data.ProxyArpRanges {
			r.ProxyArpRanges(parItem)
		}
		if data.L4Feature != nil {
			r.L4Features(data.L4Feature)
		}
		for _, anItem := range data.ApplicationNamespaces {
			r.AppNamespace(anItem)
		}
		for _, stnItem := range data.StnRules {
			r.StnRule(stnItem)
		}
		if data.NatGlobal != nil {
			r.NAT44Global(data.NatGlobal)
		}
		for _, natItem := range data.DNATs {
			r.NAT44DNat(natItem)
		}
		for _, ifItem := range data.LinuxInterfaces {
			r.LinuxInterface(ifItem)
		}
		for _, arpItem := range data.LinuxArpEntries {
			r.LinuxArpEntry(arpItem)
		}
		for _, rtItem := range data.LinuxRoutes {
			r.LinuxRoute(rtItem)
		}
	default:
		return fmt.Errorf("unknown type of request: %v", r)
	}

	return nil
}
