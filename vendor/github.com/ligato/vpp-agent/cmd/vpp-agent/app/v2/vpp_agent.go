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

package appv2

import (
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/kvdbsync"
	"github.com/ligato/cn-infra/datasync/kvdbsync/local"
	"github.com/ligato/cn-infra/datasync/msgsync"
	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/db/keyval/consul"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/redis"
	"github.com/ligato/cn-infra/health/probe"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/logging/logmanager"
	"github.com/ligato/cn-infra/messaging/kafka"

	"github.com/ligato/vpp-agent/plugins/configurator"
	"github.com/ligato/vpp-agent/plugins/kvscheduler"
	linux_ifplugin "github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin"
	linux_l3plugin "github.com/ligato/vpp-agent/plugins/linuxv2/l3plugin"
	linux_nsplugin "github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin"
	"github.com/ligato/vpp-agent/plugins/orchestrator"
	"github.com/ligato/vpp-agent/plugins/restv2"
	"github.com/ligato/vpp-agent/plugins/telemetry"
	"github.com/ligato/vpp-agent/plugins/vppv2/aclplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/ipsecplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/natplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/puntplugin"
	"github.com/ligato/vpp-agent/plugins/vppv2/stnplugin"
)

// VPPAgent defines plugins which will be loaded and their order.
// Note: the plugin itself is loaded after all its dependencies. It means that the VPP plugin is first in the list
// despite it needs to be loaded after the linux plugin.
type VPPAgent struct {
	LogManager *logmanager.Plugin

	Orchestrator *orchestrator.Plugin
	Scheduler    *kvscheduler.Scheduler

	ETCDDataSync   *kvdbsync.Plugin
	ConsulDataSync *kvdbsync.Plugin
	RedisDataSync  *kvdbsync.Plugin

	VPP
	Linux

	DataConfigurator *configurator.Plugin
	RESTAPI          *rest.Plugin
	Probe            *probe.Plugin
	Telemetry        *telemetry.Plugin
}

// New creates new VPPAgent instance.
func New() *VPPAgent {
	etcdDataSync := kvdbsync.NewPlugin(kvdbsync.UseKV(&etcd.DefaultPlugin))
	consulDataSync := kvdbsync.NewPlugin(kvdbsync.UseKV(&consul.DefaultPlugin))
	redisDataSync := kvdbsync.NewPlugin(kvdbsync.UseKV(&redis.DefaultPlugin))

	writers := datasync.KVProtoWriters{
		etcdDataSync,
		consulDataSync,
	}
	statuscheck.DefaultPlugin.Transport = writers

	ifStatePub := msgsync.NewPlugin(
		msgsync.UseMessaging(&kafka.DefaultPlugin),
		msgsync.UseConf(msgsync.Config{
			Topic: "if_state",
		}),
	)

	// Set watcher for KVScheduler.
	watchers := datasync.KVProtoWatchers{
		local.DefaultRegistry,
		etcdDataSync,
		consulDataSync,
		redisDataSync,
	}
	orchestrator.DefaultPlugin.Watcher = watchers

	// connect IfPlugins for Linux & VPP
	linux_ifplugin.DefaultPlugin.VppIfPlugin = &ifplugin.DefaultPlugin
	ifplugin.DefaultPlugin.LinuxIfPlugin = &linux_ifplugin.DefaultPlugin
	ifplugin.DefaultPlugin.NsPlugin = &linux_nsplugin.DefaultPlugin

	ifplugin.DefaultPlugin.NotifyStates = ifStatePub
	ifplugin.DefaultPlugin.PublishStatistics = writers

	vpp := DefaultVPP()
	linux := DefaultLinux()

	return &VPPAgent{
		LogManager:       &logmanager.DefaultPlugin,
		Scheduler:        &kvscheduler.DefaultPlugin,
		ETCDDataSync:     etcdDataSync,
		ConsulDataSync:   consulDataSync,
		RedisDataSync:    redisDataSync,
		Orchestrator:     &orchestrator.DefaultPlugin,
		RESTAPI:          &rest.DefaultPlugin,
		VPP:              vpp,
		Linux:            linux,
		DataConfigurator: &configurator.DefaultPlugin,
		Probe:            &probe.DefaultPlugin,
		Telemetry:        &telemetry.DefaultPlugin,
	}
}

// Init initializes main plugin.
func (VPPAgent) Init() error {
	return nil
}

// AfterInit executes resync.
func (VPPAgent) AfterInit() error {
	// manually start resync after all plugins started
	resync.DefaultPlugin.DoResync()
	return nil
}

// Close could close used resources.
func (VPPAgent) Close() error {
	return nil
}

// String returns name of the plugin.
func (VPPAgent) String() string {
	return "VPPAgent"
}

// VPP contains all VPP plugins.
type VPP struct {
	IfPlugin    *ifplugin.IfPlugin
	IPSecPlugin *ipsecplugin.IPSecPlugin
	L2Plugin    *l2plugin.L2Plugin
	L3Plugin    *l3plugin.L3Plugin
	ACLPlugin   *aclplugin.ACLPlugin
	NATPlugin   *natplugin.NATPlugin
	PuntPlugin  *puntplugin.PuntPlugin
	STNPlugin   *stnplugin.STNPlugin
}

func DefaultVPP() VPP {
	return VPP{
		IfPlugin:    &ifplugin.DefaultPlugin,
		IPSecPlugin: &ipsecplugin.DefaultPlugin,
		L2Plugin:    &l2plugin.DefaultPlugin,
		L3Plugin:    &l3plugin.DefaultPlugin,
		ACLPlugin:   &aclplugin.DefaultPlugin,
		NATPlugin:   &natplugin.DefaultPlugin,
		PuntPlugin:  &puntplugin.DefaultPlugin,
		STNPlugin:   &stnplugin.DefaultPlugin,
	}
}

// Linux contains all Linux plugins.
type Linux struct {
	IfPlugin *linux_ifplugin.IfPlugin
	L3Plugin *linux_l3plugin.L3Plugin
	NSPlugin *linux_nsplugin.NsPlugin
}

func DefaultLinux() Linux {
	return Linux{
		IfPlugin: &linux_ifplugin.DefaultPlugin,
		L3Plugin: &linux_l3plugin.DefaultPlugin,
		NSPlugin: &linux_nsplugin.DefaultPlugin,
	}
}
