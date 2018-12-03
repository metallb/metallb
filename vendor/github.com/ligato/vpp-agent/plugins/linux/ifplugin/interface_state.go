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

package ifplugin

import (
	"context"
	"sync"

	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/vishvananda/netlink"
)

// LinuxInterfaceStateNotification aggregates operational status derived from netlink with
// the details (state) about the interface.
type LinuxInterfaceStateNotification struct {
	// State of the network interface
	interfaceType  string
	interfaceState netlink.LinkOperState
	attributes     *netlink.LinkAttrs
}

// LinuxInterfaceStateUpdater processes all linux interface state data
type LinuxInterfaceStateUpdater struct {
	log     logging.Logger
	cfgLock sync.Mutex

	// Go routine management
	wg sync.WaitGroup

	// Linux interface state
	stateWatcherRunning bool
	ifStateChan         chan *LinuxInterfaceStateNotification
	ifWatcherNotifCh    chan netlink.LinkUpdate
	ifWatcherDoneCh     chan struct{}
}

// Init channels for interface state watcher, start it in separate go routine and subscribe to default namespace
func (c *LinuxInterfaceStateUpdater) Init(ctx context.Context, logger logging.PluginLogger, ifIndexes ifaceidx.LinuxIfIndexRW,
	stateChan chan *LinuxInterfaceStateNotification) error {
	// Logger
	c.log = logger.NewLogger("if-state")

	// Channels
	c.ifStateChan = stateChan
	c.ifWatcherNotifCh = make(chan netlink.LinkUpdate, 10)
	c.ifWatcherDoneCh = make(chan struct{})

	// Start watch on linux interfaces
	go c.watchLinuxInterfaces(ctx)

	// Subscribe to default linux namespace
	if err := c.subscribeInterfaceState(); err != nil {
		return errors.Errorf("failed to subscribe interface state: %v", err)
	}

	c.log.Debug("Linux interface state updater initialized")

	return nil
}

// Close watcher channel (state chan is closed in LinuxInterfaceConfigurator)
func (c *LinuxInterfaceStateUpdater) Close() error {
	if err := safeclose.Close(c.ifWatcherNotifCh); err != nil {
		return errors.Errorf("failed to safeclose linux interface state updater: %v", err)
	}
	return nil
}

// NewLinuxInterfaceStateNotification builds up new linux interface notification object
func NewLinuxInterfaceStateNotification(ifType string, ifState netlink.LinkOperState, attrs *netlink.LinkAttrs) *LinuxInterfaceStateNotification {
	return &LinuxInterfaceStateNotification{
		interfaceType:  ifType,
		interfaceState: ifState,
		attributes:     attrs,
	}
}

// Subscribe to linux default namespace
func (c *LinuxInterfaceStateUpdater) subscribeInterfaceState() error {
	if !c.stateWatcherRunning {
		c.stateWatcherRunning = true
		err := netlink.LinkSubscribe(c.ifWatcherNotifCh, c.ifWatcherDoneCh)
		if err != nil {
			return errors.Errorf("failed to subscribe link: %v", err)
		}
	}
	return nil
}

// Watch linux interfaces and send events to processing
func (c *LinuxInterfaceStateUpdater) watchLinuxInterfaces(ctx context.Context) {
	c.log.Debugf("Watching on linux link notifications")

	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case linkNotif := <-c.ifWatcherNotifCh:
			c.processLinkNotification(linkNotif)

		case <-ctx.Done():
			close(c.ifWatcherDoneCh)
			close(c.ifStateChan)
			return
		}
	}
}

// Prepare notification and send it to the state channel
func (c *LinuxInterfaceStateUpdater) processLinkNotification(link netlink.Link) {
	if link == nil || link.Attrs() == nil {
		return
	}

	c.cfgLock.Lock()
	defer c.cfgLock.Unlock()

	select {
	// Prepare and send linux link notification
	case c.ifStateChan <- NewLinuxInterfaceStateNotification(link.Type(), link.Attrs().OperState, link.Attrs()):
		// Notification sent
	default:
		c.log.Warn("Unable to send to the linux if state notification channel - buffer is full.")
	}
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *LinuxInterfaceStateUpdater) LogError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *errors.Error:
		c.log.WithField("logger", c.log).Errorf(string(err.Error() + "\n" + string(err.(*errors.Error).Stack())))
	default:
		c.log.Error(err)
	}
	return err
}
