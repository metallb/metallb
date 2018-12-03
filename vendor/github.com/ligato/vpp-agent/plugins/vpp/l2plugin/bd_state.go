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

package l2plugin

import (
	"context"
	"sync"
	"time"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/go-errors/errors"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	l2_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// BridgeDomainStateNotification contains bridge domain state object with all data published to ETCD.
type BridgeDomainStateNotification struct {
	State *l2.BridgeDomainState_BridgeDomain
}

// BridgeDomainStateUpdater holds all data required to handle bridge domain state.
type BridgeDomainStateUpdater struct {
	log    logging.Logger
	mx     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// In-memory mappings
	ifIndexes ifaceidx.SwIfIndex
	bdIndexes l2idx.BDIndex

	// State publisher
	publishBdState func(notification *BridgeDomainStateNotification)
	bdState        map[uint32]*l2.BridgeDomainState_BridgeDomain

	// VPP channel
	vppCh govppapi.Channel

	// Notification subscriptions
	vppNotifSubs            govppapi.SubscriptionCtx
	vppCountersSubs         govppapi.SubscriptionCtx
	vppCombinedCountersSubs govppapi.SubscriptionCtx
	notificationChan        chan BridgeDomainStateMessage // Injected, do not close here
	bdIdxChan               chan l2idx.BdChangeDto
}

// Init bridge domain state updater.
func (c *BridgeDomainStateUpdater) Init(ctx context.Context, logger logging.PluginLogger, goVppMux govppmux.API, bdIndexes l2idx.BDIndex, swIfIndexes ifaceidx.SwIfIndex,
	notificationChan chan BridgeDomainStateMessage, publishBdState func(notification *BridgeDomainStateNotification)) (err error) {
	// Logger
	c.log = logger.NewLogger("l2-bd-state")

	// Mappings
	c.bdIndexes = bdIndexes
	c.ifIndexes = swIfIndexes

	// State publisher
	c.notificationChan = notificationChan
	c.publishBdState = publishBdState
	c.bdState = make(map[uint32]*l2.BridgeDomainState_BridgeDomain)

	// VPP channel
	if c.vppCh, err = goVppMux.NewAPIChannel(); err != nil {
		return errors.Errorf("failed to create API channel: %v", err)
	}

	// Name-to-index watcher
	c.bdIdxChan = make(chan l2idx.BdChangeDto, 100)
	bdIndexes.WatchNameToIdx("bdplugin_bdstate", c.bdIdxChan)

	var childCtx context.Context
	childCtx, c.cancel = context.WithCancel(ctx)

	// Bridge domain notification watcher
	go c.watchVPPNotifications(childCtx)

	c.log.Info("Initializing bridge domain state updater")

	return nil
}

// watchVPPNotifications watches for delivery of notifications from VPP.
func (c *BridgeDomainStateUpdater) watchVPPNotifications(ctx context.Context) {
	c.wg.Add(1)
	defer c.wg.Done()

	if c.notificationChan != nil {
		c.log.Debugf("bridge domain state updater started to watch VPP notification")
	} else {
		c.LogError(errors.Errorf("bridge domain state updater failed to start watching VPP notifications"))
		return
	}

	for {
		select {
		case notif, ok := <-c.notificationChan:
			if !ok {
				continue
			}
			bdName := notif.Name
			switch msg := notif.Message.(type) {
			case *l2_api.BridgeDomainDetails:
				bdState, err := c.processBridgeDomainDetailsNotification(msg, bdName)
				if err != nil {
					// Log error but continue watching
					c.LogError(errors.Errorf("bridge domain state updater failed to process notification for %s: %v",
						bdName, err))
					continue
				}
				if bdState != nil {
					c.publishBdState(&BridgeDomainStateNotification{
						State: bdState,
					})
				}
			default:
				c.log.Debugf("L2Plugin: Ignoring unknown VPP notification: %v", msg)
			}

		case bdIdxDto := <-c.bdIdxChan:
			bdIdxDto.Done()

		case <-ctx.Done():
			// Stop watching for notifications.
			return
		}
	}
}

func (c *BridgeDomainStateUpdater) processBridgeDomainDetailsNotification(msg *l2_api.BridgeDomainDetails, name string) (*l2.BridgeDomainState_BridgeDomain, error) {
	bdState := &l2.BridgeDomainState_BridgeDomain{}
	// Delete case.
	if msg.BdID == 0 {
		if name == "" {
			return nil, errors.Errorf("failed to process bridge domain notification: invalid data")
		}
		// Mark index to 0 to be removed, but pass name so that the key can be constructed.
		bdState.Index = 0
		bdState.InternalName = name
		return bdState, nil
	}
	bdState.Index = msg.BdID
	name, _, found := c.bdIndexes.LookupName(msg.BdID)
	if !found {
		return nil, errors.Errorf("failed to process bridge domain notification: not found in the mapping")
	}
	bdState.InternalName = name
	bdState.InterfaceCount = msg.NSwIfs
	name, _, found = c.ifIndexes.LookupName(msg.BviSwIfIndex)
	if found {
		bdState.BviInterface = name
		bdState.BviInterfaceIndex = msg.BviSwIfIndex
	} else {
		bdState.BviInterface = "not_set"
	}
	bdState.L2Params = getBridgeDomainStateParams(msg)
	bdState.Interfaces = c.getBridgeDomainInterfaces(msg)
	bdState.LastChange = time.Now().Unix()

	return bdState, nil
}

func (c *BridgeDomainStateUpdater) getBridgeDomainInterfaces(msg *l2_api.BridgeDomainDetails) []*l2.BridgeDomainState_BridgeDomain_Interfaces {
	var bdStateInterfaces []*l2.BridgeDomainState_BridgeDomain_Interfaces
	for _, swIfaceDetails := range msg.SwIfDetails {
		bdIfaceState := &l2.BridgeDomainState_BridgeDomain_Interfaces{}
		name, _, found := c.ifIndexes.LookupName(swIfaceDetails.SwIfIndex)
		if !found {
			c.log.Warnf("Interface name with index %d not found for bridge domain status", swIfaceDetails.SwIfIndex)
			bdIfaceState.Name = "unknown"
		} else {
			bdIfaceState.Name = name
		}
		bdIfaceState.SwIfIndex = swIfaceDetails.SwIfIndex
		bdIfaceState.SplitHorizonGroup = uint32(swIfaceDetails.Shg)
		bdStateInterfaces = append(bdStateInterfaces, bdIfaceState)
	}
	return bdStateInterfaces
}

func getBridgeDomainStateParams(msg *l2_api.BridgeDomainDetails) *l2.BridgeDomainState_BridgeDomain_L2Params {
	params := &l2.BridgeDomainState_BridgeDomain_L2Params{}
	params.Flood = intToBool(msg.Flood)
	params.UnknownUnicastFlood = intToBool(msg.UuFlood)
	params.Forward = intToBool(msg.Forward)
	params.Learn = intToBool(msg.Learn)
	params.ArpTermination = intToBool(msg.ArpTerm)
	params.MacAge = uint32(msg.MacAge)
	return params
}

func intToBool(num uint8) bool {
	if num == 1 {
		return true
	}
	return false
}

// LogError prints error if not nil, including stack trace. The same value is also returned, so it can be easily propagated further
func (c *BridgeDomainStateUpdater) LogError(err error) error {
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
