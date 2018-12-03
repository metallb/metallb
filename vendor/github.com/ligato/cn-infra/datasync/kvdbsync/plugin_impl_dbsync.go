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

package kvdbsync

import (
	"errors"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/datasync/syncbase"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/servicelabel"
)

var (
	// ErrNotReady is an error returned when KVDBSync plugin is being used before the KVPlugin is ready.
	ErrNotReady = errors.New("transport adapter is not ready yet (probably called before AfterInit)")
)

// Plugin dbsync implements synchronization between local memory and db.
// Other plugins can be notified when DB changes occur or resync is needed.
// This plugin reads/pulls the data from db when resync is needed.
type Plugin struct {
	Deps

	adapter  *watcher
	registry *syncbase.Registry
}

// Deps groups dependencies injected into the plugin so that they are
// logically separated from other plugin fields.
type Deps struct {
	infra.PluginName
	Log          logging.PluginLogger
	KvPlugin     keyval.KvProtoPlugin // inject
	ResyncOrch   resync.Subscriber
	ServiceLabel servicelabel.ReaderAPI
}

// Init only initializes plugin.registry.
func (p *Plugin) Init() error {
	p.registry = syncbase.NewRegistry()

	return nil
}

// AfterInit uses provided connection to build new transport watcher.
//
// Plugin.registry subscriptions (registered by Watch method) are used for resync.
// Resync is called only if ResyncOrch was injected (i.e. is not nil).
// The order of plugins in flavor is not important to resync
// since Watch() is called in Plugin.Init() and Resync.Register()
// is called in Plugin.AfterInit().
//
// If provided connection is not ready (not connected), AfterInit starts new goroutine in order to
// 'wait' for the connection. After that, the new transport watcher is built as usual.
func (p *Plugin) AfterInit() error {
	if !p.isKvEnabled() {
		p.Log.Debugf("KVPlugin is nil or disabled, skipping AfterInit")
		return nil
	}

	// set function to be executed on KVPlugin connection
	p.KvPlugin.OnConnect(p.initKvPlugin)

	return nil
}

func (p *Plugin) isKvEnabled() bool {
	return p.KvPlugin != nil && !p.KvPlugin.Disabled()
}

func (p *Plugin) initKvPlugin() error {
	if !p.isKvEnabled() {
		p.Log.Debugf("KVPlugin is nil or disabled, skipping initKvPlugin")
		return nil
	}

	p.adapter = &watcher{
		db:   p.KvPlugin.NewBroker(p.ServiceLabel.GetAgentPrefix()),
		dbW:  p.KvPlugin.NewWatcher(p.ServiceLabel.GetAgentPrefix()),
		base: p.registry,
	}

	if p.ResyncOrch != nil {
		for name, sub := range p.registry.Subscriptions() {
			reg := p.ResyncOrch.Register(name)
			_, err := watchAndResyncBrokerKeys(reg, sub.ChangeChan, sub.ResyncChan, sub.CloseChan,
				p.adapter, sub.KeyPrefixes...)
			if err != nil {
				return err
			}
		}
	} else {
		p.Log.Debugf("ResyncOrch is nil, skipping registration")
	}

	return nil
}

// Watch adds entry to the plugin.registry. By doing this, other plugins will receive notifications
// about data changes and data resynchronization.
//
// This method is supposed to be called in Plugin.Init().
// Calling this method later than kvdbsync.Plugin.AfterInit() will have no effect
// (no notifications will be received).
func (p *Plugin) Watch(resyncName string, changeChan chan datasync.ChangeEvent,
	resyncChan chan datasync.ResyncEvent, keyPrefixes ...string) (datasync.WatchRegistration, error) {

	return p.registry.Watch(resyncName, changeChan, resyncChan, keyPrefixes...)
}

// Put propagates this call to a particular kvdb.Plugin unless the kvdb.Plugin is Disabled().
//
// This method is supposed to be called in Plugin.AfterInit() or later (even from different go routine).
func (p *Plugin) Put(key string, data proto.Message, opts ...datasync.PutOption) error {
	if !p.isKvEnabled() {
		return nil
	}

	if p.adapter != nil {
		return p.adapter.db.Put(key, data, opts...)
	}

	return ErrNotReady
}

// Delete propagates this call to a particular kvdb.Plugin unless the kvdb.Plugin is Disabled().
//
// This method is supposed to be called in Plugin.AfterInit() or later (even from different go routine).
func (p *Plugin) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	if !p.isKvEnabled() {
		return false, nil
	}

	if p.adapter != nil {
		return p.adapter.db.Delete(key, opts...)
	}

	return false, ErrNotReady
}

// Close resources.
func (p *Plugin) Close() error {
	return nil
}
