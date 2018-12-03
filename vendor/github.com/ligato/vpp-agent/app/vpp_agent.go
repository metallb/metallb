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

package app

import (
	"sync"

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
	"github.com/ligato/vpp-agent/plugins/linux"
	"github.com/ligato/vpp-agent/plugins/rest"
	"github.com/ligato/vpp-agent/plugins/telemetry"
	"github.com/ligato/vpp-agent/plugins/vpp"
	"github.com/ligato/vpp-agent/plugins/vpp/rpc"
)

// VPPAgent defines plugins which will be loaded and their order.
// Note: the plugin itself is loaded after all its dependencies. It means that the VPP plugin is first in the list
// despite it needs to be loaded after the linux plugin.
type VPPAgent struct {
	LogManager *logmanager.Plugin

	ETCDDataSync   *kvdbsync.Plugin
	ConsulDataSync *kvdbsync.Plugin
	RedisDataSync  *kvdbsync.Plugin

	VPP   *vpp.Plugin
	Linux *linux.Plugin

	GRPCService *rpc.Plugin
	RESTAPI     *rest.Plugin
	Probe       *probe.Plugin
	Telemetry   *telemetry.Plugin
}

// New creates new VPPAgent instance.
func New() *VPPAgent {
	etcdDataSync := kvdbsync.NewPlugin(kvdbsync.UseKV(&etcd.DefaultPlugin))
	consulDataSync := kvdbsync.NewPlugin(kvdbsync.UseKV(&consul.DefaultPlugin))
	redisDataSync := kvdbsync.NewPlugin(kvdbsync.UseKV(&redis.DefaultPlugin))

	watchers := datasync.KVProtoWatchers{
		local.Get(),
		etcdDataSync,
		consulDataSync,
	}
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

	vppPlugin := vpp.NewPlugin(vpp.UseDeps(func(deps *vpp.Deps) {
		deps.Publish = writers
		deps.Watcher = watchers
		deps.IfStatePub = ifStatePub
		deps.DataSyncs = map[string]datasync.KeyProtoValWriter{
			"etcd":  etcdDataSync,
			"redis": redisDataSync,
		}
		deps.GRPCSvc = &rpc.DefaultPlugin
	}))
	linuxPlugin := linux.NewPlugin(linux.UseDeps(func(deps *linux.Deps) {
		deps.VPP = vppPlugin
		deps.Watcher = watchers
	}))
	vppPlugin.Deps.Linux = linuxPlugin

	var watchEventsMutex sync.Mutex
	vppPlugin.Deps.WatchEventsMutex = &watchEventsMutex
	linuxPlugin.Deps.WatchEventsMutex = &watchEventsMutex

	restPlugin := rest.NewPlugin(rest.UseDeps(func(deps *rest.Deps) {
		deps.VPP = vppPlugin
		deps.Linux = linuxPlugin
	}))

	return &VPPAgent{
		LogManager:     &logmanager.DefaultPlugin,
		ETCDDataSync:   etcdDataSync,
		ConsulDataSync: consulDataSync,
		RedisDataSync:  redisDataSync,
		VPP:            vppPlugin,
		Linux:          linuxPlugin,
		GRPCService:    &rpc.DefaultPlugin,
		RESTAPI:        restPlugin,
		Probe:          &probe.DefaultPlugin,
		Telemetry:      &telemetry.DefaultPlugin,
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
