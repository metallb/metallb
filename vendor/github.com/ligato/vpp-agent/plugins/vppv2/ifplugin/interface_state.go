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

package ifplugin

import (
	"bytes"
	"context"
	"net"
	"os"
	"sync"
	"time"

	"git.fd.io/govpp.git/adapter"
	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/pkg/errors"

	intf "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
)

// PeriodicPollingPeriod between statistics reads
// TODO  should be configurable
var PeriodicPollingPeriod = 1 * time.Second

// statType is the specific interface statistics type.
type statType string

// interface stats prefix as defined by the VPP
const ifPrefix = "/if/"

// interface statistics matching patterns
const (
	Drop    statType = ifPrefix + "drops"
	Punt             = ifPrefix + "punt"
	IPv4             = ifPrefix + "ip4"
	IPv6             = ifPrefix + "ip6"
	RxNoBuf          = ifPrefix + "rx-no-buf"
	RxMiss           = ifPrefix + "rx-miss"
	RxError          = ifPrefix + "rx-error"
	TxError          = ifPrefix + "tx-error"
	Rx               = ifPrefix + "rx"
	Tx               = ifPrefix + "tx"
)

const (
	megabit = 1000000 // One megabit in bytes
)

// InterfaceStateUpdater holds state data of all VPP interfaces.
type InterfaceStateUpdater struct {
	log logging.Logger

	kvScheduler    kvs.KVScheduler
	swIfIndexes    ifaceidx.IfaceMetadataIndex
	publishIfState func(notification *intf.InterfaceNotification)

	ifState map[uint32]*intf.InterfaceState // swIfIndex to state data map
	access  sync.Mutex                      // lock for the state data map

	goVppMux govppmux.StatsAPI

	vppCh                   govppapi.Channel
	vppNotifSubs            govppapi.SubscriptionCtx
	vppCountersSubs         govppapi.SubscriptionCtx
	vppCombinedCountersSubs govppapi.SubscriptionCtx
	notifChan               chan govppapi.Message
	ifMetaChan              chan ifaceidx.IfaceMetadataDto

	cancel context.CancelFunc // cancel can be used to cancel all goroutines and their jobs inside of the plugin
	wg     sync.WaitGroup     // wait group that allows to wait until all goroutines of the plugin have finished
}

// Init members (channels, maps...) and start go routines
func (c *InterfaceStateUpdater) Init(ctx context.Context, logger logging.PluginLogger, kvScheduler kvs.KVScheduler,
	goVppMux govppmux.StatsAPI, swIfIndexes ifaceidx.IfaceMetadataIndex,
	publishIfState func(notification *intf.InterfaceNotification)) (err error) {

	// Logger
	c.log = logger.NewLogger("if-state")

	// Mappings
	c.swIfIndexes = swIfIndexes

	c.kvScheduler = kvScheduler
	c.publishIfState = publishIfState
	c.ifState = make(map[uint32]*intf.InterfaceState)

	// VPP channel
	c.goVppMux = goVppMux
	c.vppCh, err = c.goVppMux.NewAPIChannel()
	if err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	c.ifMetaChan = make(chan ifaceidx.IfaceMetadataDto, 100)
	swIfIndexes.WatchInterfaces("ifplugin_ifstate", c.ifMetaChan)
	c.notifChan = make(chan govppapi.Message, 100)

	// Create child context
	var childCtx context.Context
	childCtx, c.cancel = context.WithCancel(ctx)

	// Watch for incoming notifications
	c.wg.Add(1)
	go c.watchVPPNotifications(childCtx)

	c.log.Info("Interface state updater initialized")

	return nil
}

// AfterInit subscribes for watching VPP notifications on previously initialized channel
func (c *InterfaceStateUpdater) AfterInit() error {
	err := c.subscribeVPPNotifications()
	if err != nil {
		return err
	}
	return nil
}

// subscribeVPPNotifications subscribes for interface state notifications from VPP.
func (c *InterfaceStateUpdater) subscribeVPPNotifications() error {
	var err error
	// subscribe for receiving SwInterfaceEvents notifications
	if c.vppNotifSubs, err = c.vppCh.SubscribeNotification(c.notifChan, &interfaces.SwInterfaceEvent{}); err != nil {
		return errors.Errorf("failed to subscribe VPP notification (sw_interface_event): %v", err)
	}

	wantIfEventsReply := &interfaces.WantInterfaceEventsReply{}
	// enable interface state notifications from VPP
	err = c.vppCh.SendRequest(&interfaces.WantInterfaceEvents{
		PID:           uint32(os.Getpid()),
		EnableDisable: 1,
	}).ReceiveReply(wantIfEventsReply)
	if err != nil {
		return errors.Errorf("failed to get interface events: %v", err)
	}

	return nil
}

// Close unsubscribes from interface state notifications from VPP & GOVPP channel
func (c *InterfaceStateUpdater) Close() error {
	c.cancel()
	c.wg.Wait()

	if c.vppNotifSubs != nil {
		if err := c.vppNotifSubs.Unsubscribe(); err != nil {
			return errors.Errorf("failed to unsubscribe interface state notification on close: %v", err)
		}
	}
	if c.vppCountersSubs != nil {
		if err := c.vppCountersSubs.Unsubscribe(); err != nil {
			return errors.Errorf("failed to unsubscribe interface state counters on close: %v", err)
		}
	}
	if c.vppCombinedCountersSubs != nil {
		if err := c.vppCombinedCountersSubs.Unsubscribe(); err != nil {
			return errors.Errorf("failed to unsubscribe interface state combined counters on close: %v", err)
		}
	}

	if err := safeclose.Close(c.vppCh, c.notifChan); err != nil {
		return errors.Errorf("failed to safe close interface state: %v", err)
	}

	return nil
}

// watchVPPNotifications watches for delivery of notifications from VPP.
func (c *InterfaceStateUpdater) watchVPPNotifications(ctx context.Context) {
	defer c.wg.Done()

	if c.notifChan != nil {
		c.log.Debug("Interface state VPP notification watcher started")
	} else {
		c.log.Warn("Interface state VPP notification does not start: the channel c nil")
		return
	}

	// Periodically read VPP counters and combined counters for VPP statistics
	go c.startReadingCounters(ctx)

	for {
		select {
		case msg := <-c.notifChan:
			// if the notification is a result of a configuration change,
			// make sure the associated transaction has already finalized
			c.kvScheduler.TransactionBarrier()

			switch notif := msg.(type) {
			case *interfaces.SwInterfaceEvent:
				c.processIfStateEvent(notif)
			default:
				c.log.Debugf("Ignoring unknown VPP notification: %s, %v",
					msg.GetMessageName(), msg)
			}

		case ifMetaDto := <-c.ifMetaChan:
			if ifMetaDto.Del {
				c.setIfStateDeleted(ifMetaDto.Metadata.SwIfIndex, ifMetaDto.Name)
			} else if !ifMetaDto.Update {
				// process new interface (no way to filter by swIfIndex, need to dump all of them)
				req := &interfaces.SwInterfaceDump{}
				reqCtx := c.vppCh.SendMultiRequest(req)

				for {
					msg := &interfaces.SwInterfaceDetails{}
					stop, err := reqCtx.ReceiveReply(msg)
					if stop {
						break
					}
					if err != nil {
						c.log.Warnf("failed to receive interface dump details: %v", err)
						continue
					}
					if msg.SwIfIndex != ifMetaDto.Metadata.SwIfIndex {
						// not the added interface
						continue
					}
					c.updateIfStateDetails(msg)
				}
			}

		case <-ctx.Done():
			// stop watching for notifications and periodic statistics reader
			c.log.Debug("Interface state VPP notification watcher stopped")
			return
		}
	}
}

// startReadingCounters periodically reads statistics for all interfaces
func (c *InterfaceStateUpdater) startReadingCounters(ctx context.Context) {
	for {
		select {
		case <-time.After(PeriodicPollingPeriod):
			c.doInterfaceStatsRead()
		case <-ctx.Done():
			c.log.Debug("Interface state VPP periodic polling stopped")
			return
		}
	}
}

// doInterfaceStatsRead dumps statistics using interface filter and processes them
func (c *InterfaceStateUpdater) doInterfaceStatsRead() {
	c.access.Lock()
	defer c.access.Unlock()

	statEntries, err := c.goVppMux.DumpStats(ifPrefix)
	if err != nil {
		// TODO add some counter to prevent it log forever
		c.log.Errorf("failed to read statistics data: %v", err)
	}
	for _, statEntry := range statEntries {
		switch data := statEntry.Data.(type) {
		case adapter.SimpleCounterStat:
			c.processSimpleCounterStat(statType(statEntry.Name), data)
		case adapter.CombinedCounterStat:
			c.processCombinedCounterStat(statType(statEntry.Name), data)
		}
	}
}

// processSimpleCounterStat fills state data for every registered interface and publishes them
func (c *InterfaceStateUpdater) processSimpleCounterStat(statName statType, data adapter.SimpleCounterStat) {
	if data == nil || len(data) == 0 {
		return
	}
	// Add up counter values from all workers, sumPackets is fixed length - all the inner arrays (workers)
	// have values for all interfaces. Length is taken from the first worker (main thread) which is always present
	sumPackets := make([]adapter.Counter, len(data[0]))
	for _, worker := range data {
		for swIfIndex, partialPackets := range worker {
			sumPackets[swIfIndex] += partialPackets
		}
	}
	for swIfIndex, packets := range sumPackets {
		ifState, found := c.getIfStateDataWLookup(uint32(swIfIndex))
		if !found {
			continue
		}
		ifStats := ifState.Statistics
		switch statName {
		case Drop:
			ifStats.DropPackets = uint64(packets)
		case Punt:
			ifStats.PuntPackets = uint64(packets)
		case IPv4:
			ifStats.Ipv4Packets = uint64(packets)
		case IPv6:
			ifStats.Ipv6Packets = uint64(packets)
		case RxNoBuf:
			ifStats.InNobufPackets = uint64(packets)
		case RxMiss:
			ifStats.InMissPackets = uint64(packets)
		case RxError:
			ifStats.InErrorPackets = uint64(packets)
		case TxError:
			ifStats.OutErrorPackets = uint64(packets)
		}

		c.publishIfState(&intf.InterfaceNotification{
			Type: intf.InterfaceNotification_COUNTERS, State: ifState})
	}
}

// processCombinedCounterStat fills combined state data for every registered interface and publishes them
func (c *InterfaceStateUpdater) processCombinedCounterStat(statName statType, data adapter.CombinedCounterStat) {
	if data == nil || len(data) == 0 {
		return
	}
	// Add up counter values from all workers, sumPackets is fixed length - all the inner arrays (workers)
	// have values for all interfaces. Length is taken from the first worker (main thread) which is always present
	sumCombined := make([]adapter.CombinedCounter, len(data[0]))
	for _, worker := range data {
		for swIfIndex, partialPackets := range worker {
			sumCombined[swIfIndex].Packets += partialPackets.Packets
			sumCombined[swIfIndex].Bytes += partialPackets.Bytes
		}
	}
	for swIfIndex, combined := range sumCombined {
		ifState, found := c.getIfStateDataWLookup(uint32(swIfIndex))
		if !found {
			continue
		}
		ifStats := ifState.Statistics
		switch statName {
		case Rx:
			ifStats.InPackets = uint64(combined.Packets)
			ifStats.InBytes = uint64(combined.Bytes)
		case Tx:
			ifStats.OutPackets = uint64(combined.Packets)
			ifStats.OutBytes = uint64(combined.Bytes)
		}
		// TODO process other stats types

		c.publishIfState(&intf.InterfaceNotification{
			Type: intf.InterfaceNotification_COUNTERS, State: ifState})
	}
}

// processIfStateEvent process a VPP state event notification.
func (c *InterfaceStateUpdater) processIfStateEvent(notif *interfaces.SwInterfaceEvent) {
	c.access.Lock()
	defer c.access.Unlock()

	// update and return if state data
	ifState, found := c.updateIfStateFlags(notif)
	if !found {
		return
	}
	c.log.Debugf("Interface state notification for %s (idx: %d): %+v",
		ifState.Name, ifState.IfIndex, notif)

	// store data in ETCD
	c.publishIfState(&intf.InterfaceNotification{
		Type: intf.InterfaceNotification_UPDOWN, State: ifState})
}

// getIfStateData returns interface state data structure for the specified interface index and interface name.
// NOTE: plugin.ifStateData needs to be locked when calling this function!
func (c *InterfaceStateUpdater) getIfStateData(swIfIndex uint32, ifName string) (*intf.InterfaceState, bool) {

	ifState, ok := c.ifState[swIfIndex]

	// check also if the provided logical name c the same as the one associated
	// with swIfIndex, because swIfIndexes might be reused
	if ok && ifState.Name == ifName {
		return ifState, true
	}

	return nil, false
}

// getIfStateDataWLookup returns interface state data structure for the specified interface index (creates it if it does not exist).
// NOTE: plugin.ifStateData needs to be locked when calling this function!
func (c *InterfaceStateUpdater) getIfStateDataWLookup(ifIdx uint32) (
	*intf.InterfaceState, bool) {
	ifName, _, found := c.swIfIndexes.LookupBySwIfIndex(ifIdx)
	if !found {
		return nil, found
	}

	ifState, found := c.getIfStateData(ifIdx, ifName)
	if !found {
		ifState = &intf.InterfaceState{
			IfIndex:    ifIdx,
			Name:       ifName,
			Statistics: &intf.InterfaceState_Statistics{},
		}

		c.ifState[ifIdx] = ifState
		found = true
	}

	return ifState, found
}

// updateIfStateFlags updates the interface state data in memory from provided VPP flags message and returns updated state data.
// NOTE: plugin.ifStateData needs to be locked when calling this function!
func (c *InterfaceStateUpdater) updateIfStateFlags(vppMsg *interfaces.SwInterfaceEvent) (
	iface *intf.InterfaceState, found bool) {

	ifState, found := c.getIfStateDataWLookup(vppMsg.SwIfIndex)
	if !found {
		return nil, false
	}
	ifState.LastChange = time.Now().Unix()

	if vppMsg.Deleted == 1 {
		ifState.AdminStatus = intf.InterfaceState_DELETED
		ifState.OperStatus = intf.InterfaceState_DELETED
	} else {
		if vppMsg.AdminUpDown == 1 {
			ifState.AdminStatus = intf.InterfaceState_UP
		} else {
			ifState.AdminStatus = intf.InterfaceState_DOWN
		}
		if vppMsg.LinkUpDown == 1 {
			ifState.OperStatus = intf.InterfaceState_UP
		} else {
			ifState.OperStatus = intf.InterfaceState_DOWN
		}
	}
	return ifState, true
}

// updateIfStateDetails updates the interface state data in memory from provided VPP details message.
func (c *InterfaceStateUpdater) updateIfStateDetails(ifDetails *interfaces.SwInterfaceDetails) {
	c.access.Lock()
	defer c.access.Unlock()

	ifState, found := c.getIfStateDataWLookup(ifDetails.SwIfIndex)
	if !found {
		return
	}

	ifState.InternalName = string(bytes.SplitN(ifDetails.InterfaceName, []byte{0x00}, 2)[0])

	if ifDetails.AdminUpDown == 1 {
		ifState.AdminStatus = intf.InterfaceState_UP
	} else {
		ifState.AdminStatus = intf.InterfaceState_DOWN
	}

	if ifDetails.LinkUpDown == 1 {
		ifState.OperStatus = intf.InterfaceState_UP
	} else {
		ifState.OperStatus = intf.InterfaceState_DOWN
	}

	hwAddr := net.HardwareAddr(ifDetails.L2Address[:ifDetails.L2AddressLength])
	ifState.PhysAddress = hwAddr.String()

	ifState.Mtu = uint32(ifDetails.LinkMtu)

	switch ifDetails.LinkSpeed {
	case 1:
		ifState.Speed = 10 * megabit // 10M
	case 2:
		ifState.Speed = 100 * megabit // 100M
	case 4:
		ifState.Speed = 1000 * megabit // 1G
	case 8:
		ifState.Speed = 10000 * megabit // 10G
	case 16:
		ifState.Speed = 40000 * megabit // 40G
	case 32:
		ifState.Speed = 100000 * megabit // 100G
	default:
		ifState.Speed = 0
	}

	switch ifDetails.LinkSpeed {
	case 1:
		ifState.Duplex = intf.InterfaceState_HALF
	case 2:
		ifState.Duplex = intf.InterfaceState_FULL
	default:
		ifState.Duplex = intf.InterfaceState_UNKNOWN_DUPLEX
	}

	c.publishIfState(&intf.InterfaceNotification{
		Type: intf.InterfaceNotification_UNKNOWN, State: ifState})
}

// setIfStateDeleted marks the interface as deleted in the state data structure in memory.
func (c *InterfaceStateUpdater) setIfStateDeleted(swIfIndex uint32, ifName string) {
	c.access.Lock()
	defer c.access.Unlock()

	ifState, found := c.getIfStateData(swIfIndex, ifName)
	if !found {
		return
	}
	ifState.AdminStatus = intf.InterfaceState_DELETED
	ifState.OperStatus = intf.InterfaceState_DELETED
	ifState.LastChange = time.Now().Unix()

	// this can be post-processed by multiple plugins
	c.publishIfState(&intf.InterfaceNotification{
		Type: intf.InterfaceNotification_UNKNOWN, State: ifState})
}
