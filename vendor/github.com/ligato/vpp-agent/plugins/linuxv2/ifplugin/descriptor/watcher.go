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

package descriptor

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	prototypes "github.com/gogo/protobuf/types"
	"github.com/ligato/cn-infra/logging"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"

	ifmodel "github.com/ligato/vpp-agent/api/models/linux/interfaces"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/linuxcalls"
)

const (
	// InterfaceWatcherName is the name of the descriptor watching Linux interfaces
	// in the default namespace.
	InterfaceWatcherName = "linux-interface-watcher"

	// notificationDelay specifies how long to delay notification when interface changes.
	// Typically interface is created in multiple stages and we do not want to notify
	// scheduler about intermediate states.
	notificationDelay = 500 * time.Millisecond
)

// InterfaceWatcher watches default namespace for newly added/removed Linux interfaces.
type InterfaceWatcher struct {
	// input arguments
	log         logging.Logger
	kvscheduler kvs.KVScheduler
	ifHandler   linuxcalls.NetlinkAPIRead

	// go routine management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// a set of interfaces present in the default namespace
	ifacesMu sync.Mutex
	ifaces   map[string]struct{}

	// interface changes delayed to give Linux time to "finalize" them
	pendingIntfs map[string]bool // interface name -> exists?

	// conditional variable to check if the list of interfaces is in-sync with
	// Linux network stack
	intfsInSync     bool
	intfsInSyncCond *sync.Cond

	// Linux notifications
	notifCh chan netlink.LinkUpdate
	doneCh  chan struct{}
}

// NewInterfaceWatcher creates a new instance of the Interface Watcher.
func NewInterfaceWatcher(kvscheduler kvs.KVScheduler, ifHandler linuxcalls.NetlinkAPI, log logging.PluginLogger) *InterfaceWatcher {
	descriptor := &InterfaceWatcher{
		log:          log.NewLogger("if-watcher"),
		kvscheduler:  kvscheduler,
		ifHandler:    ifHandler,
		ifaces:       make(map[string]struct{}),
		pendingIntfs: make(map[string]bool),
		notifCh:      make(chan netlink.LinkUpdate),
		doneCh:       make(chan struct{}),
	}
	descriptor.intfsInSyncCond = sync.NewCond(&descriptor.ifacesMu)
	descriptor.ctx, descriptor.cancel = context.WithCancel(context.Background())

	return descriptor
}

// GetDescriptor returns descriptor suitable for registration with the KVScheduler.
func (w *InterfaceWatcher) GetDescriptor() *kvs.KVDescriptor {
	return &kvs.KVDescriptor{
		Name:        InterfaceWatcherName,
		KeySelector: w.IsLinuxInterfaceNotification,
		Dump:        w.Dump,
	}
}

// IsLinuxInterfaceNotification returns <true> for keys representing
// notifications about Linux interfaces in the default network namespace.
func (w *InterfaceWatcher) IsLinuxInterfaceNotification(key string) bool {
	return strings.HasPrefix(key, ifmodel.InterfaceHostNameKeyPrefix)
}

// Dump returns key with empty value for every currently existing Linux interface
// in the default network namespace.
func (w *InterfaceWatcher) Dump(correlate []kvs.KVWithMetadata) (dump []kvs.KVWithMetadata, err error) {
	// wait until the set of interfaces is in-sync with the Linux network stack
	w.ifacesMu.Lock()
	if !w.intfsInSync {
		w.intfsInSyncCond.Wait()
	}
	defer w.ifacesMu.Unlock()

	for ifName := range w.ifaces {
		dump = append(dump, kvs.KVWithMetadata{
			Key:    ifmodel.InterfaceHostNameKey(ifName),
			Value:  &prototypes.Empty{},
			Origin: kvs.FromSB,
		})
	}

	return dump, nil
}

// StartWatching starts interface watching.
func (w *InterfaceWatcher) StartWatching() error {
	// watch default namespace to be aware of interfaces not created by this plugin
	err := w.ifHandler.LinkSubscribe(w.notifCh, w.doneCh)
	if err != nil {
		err = errors.Errorf("failed to subscribe link: %v", err)
		w.log.Error(err)
		return err
	}
	w.wg.Add(1)
	go w.watchDefaultNamespace()
	return nil
}

// StopWatching stops interface watching.
func (w *InterfaceWatcher) StopWatching() {
	w.cancel()
	w.wg.Wait()
}

// watchDefaultNamespace watches for notification about added/removed interfaces
// to/from the default namespace.
func (w *InterfaceWatcher) watchDefaultNamespace() {
	defer w.wg.Done()

	// get the set of interfaces already available in the default namespace
	links, err := w.ifHandler.GetLinkList()
	if err == nil {
		for _, link := range links {
			if enabled, err := w.ifHandler.IsInterfaceUp(link.Attrs().Name); enabled && err == nil {
				w.ifaces[link.Attrs().Name] = struct{}{}
			}
		}
	} else {
		w.log.Warnf("failed to list interfaces in the default namespace: %v", err)
	}

	// mark the state in-sync with the Linux network stack
	w.ifacesMu.Lock()
	w.intfsInSync = true
	w.ifacesMu.Unlock()
	w.intfsInSyncCond.Broadcast()

	for {
		select {
		case linkNotif := <-w.notifCh:
			w.processLinkNotification(linkNotif)

		case <-w.ctx.Done():
			close(w.doneCh)
			return
		}
	}
}

// processLinkNotification processes link notification received from Linux.
func (w *InterfaceWatcher) processLinkNotification(linkUpdate netlink.LinkUpdate) {
	w.ifacesMu.Lock()
	defer w.ifacesMu.Unlock()

	ifName := linkUpdate.Attrs().Name
	isEnabled := linkUpdate.Attrs().OperState != netlink.OperDown &&
		linkUpdate.Attrs().OperState != netlink.OperNotPresent

	_, isPendingNotif := w.pendingIntfs[ifName]
	if isPendingNotif {
		// notification for this interface is already scheduled, just update the state
		w.pendingIntfs[ifName] = isEnabled
		return
	}

	if !w.needsUpdate(ifName, isEnabled) {
		// ignore notification if the interface admin status remained the same
		return
	}

	if isEnabled {
		// do not notify until interface is truly finished
		w.pendingIntfs[ifName] = true
		w.wg.Add(1)
		go w.delayNotification(ifName)
		return
	}

	// notification about removed interface is propagated immediately
	w.notifyScheduler(ifName, false)
}

// delayNotification delays notification about enabled interface - typically
// interface is created in multiple stages and we do not want to notify scheduler
// about intermediate states.
func (w *InterfaceWatcher) delayNotification(ifName string) {
	defer w.wg.Done()

	select {
	case <-w.ctx.Done():
		return
	case <-time.After(notificationDelay):
		w.applyDelayedNotification(ifName)
	}
}

// applyDelayedNotification applies delayed interface notification.
func (w *InterfaceWatcher) applyDelayedNotification(ifName string) {
	w.ifacesMu.Lock()
	defer w.ifacesMu.Unlock()

	// in the meantime the status may have changed and may not require update anymore
	isEnabled := w.pendingIntfs[ifName]
	if w.needsUpdate(ifName, isEnabled) {
		w.notifyScheduler(ifName, isEnabled)
	}

	delete(w.pendingIntfs, ifName)
}

// notifyScheduler notifies scheduler about interface change.
func (w *InterfaceWatcher) notifyScheduler(ifName string, enabled bool) {
	var value proto.Message

	if enabled {
		w.ifaces[ifName] = struct{}{}
		value = &prototypes.Empty{}
	} else {
		delete(w.ifaces, ifName)
	}

	w.kvscheduler.PushSBNotification(
		ifmodel.InterfaceHostNameKey(ifName),
		value,
		nil)
}

func (w *InterfaceWatcher) needsUpdate(ifName string, isEnabled bool) bool {
	_, wasEnabled := w.ifaces[ifName]
	return isEnabled != wasEnabled
}
