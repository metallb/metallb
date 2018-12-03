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
	"context"

	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// Resync deletes obsolete operation status of network interfaces in DB.
// Obsolete state is one that is not part of SwIfIndex.
func (plugin *Plugin) resyncIfStateEvents(keys []string) error {
	for _, key := range keys {
		ifaceName, err := intf.ParseNameFromKey(key)
		if err != nil {
			return err
		}

		_, _, found := plugin.swIfIndexes.LookupIdx(ifaceName)
		if !found {
			err := plugin.PublishStatistics.Put(key, nil /*means delete*/)
			if err != nil {
				return err
			}
			plugin.Log.Debugf("Obsolete status for %v deleted", key)
		} else {
			plugin.Log.WithField("ifaceName", ifaceName).Debug("interface status is needed")
		}
	}

	return nil
}

// publishIfState goroutine is used to watch interface state notifications that are propagated to Messaging topic.
func (plugin *Plugin) publishIfStateEvents(ctx context.Context) {
	plugin.wg.Add(1)
	defer plugin.wg.Done()

	// store last errors to prevent repeating
	var lastPublishErr error
	var lastNotifErr error

	for {
		select {
		case ifState := <-plugin.ifStateChan:
			key := intf.InterfaceStateKey(ifState.State.Name)

			if plugin.PublishStatistics != nil {
				err := plugin.PublishStatistics.Put(key, ifState.State)
				if err != nil {
					if lastPublishErr == nil || lastPublishErr.Error() != err.Error() {
						plugin.Log.Error(err)
					}
				}
				lastPublishErr = err
			}

			// Marshall data into JSON & send kafka message.
			if plugin.ifStateNotifications != nil && ifState.Type == intf.InterfaceNotification_UPDOWN {
				err := plugin.ifStateNotifications.Put(key, ifState.State)
				if err != nil {
					if lastNotifErr == nil || lastNotifErr.Error() != err.Error() {
						plugin.Log.Error(err)
					}
				}
				lastNotifErr = err
			}

			// Send interface state data to global agent status
			if plugin.statusCheckReg {
				plugin.StatusCheck.ReportStateChangeWithMeta(plugin.PluginName, statuscheck.OK, nil, &status.InterfaceStats_Interface{
					InternalName: ifState.State.InternalName,
					Index:        ifState.State.IfIndex,
					Status:       ifState.State.AdminStatus.String(),
					MacAddress:   ifState.State.PhysAddress,
				})
			}

			// Update interface notification data enabled for external GRPC endpoints
			if plugin.GRPCSvc != nil {
				plugin.GRPCSvc.UpdateNotifications(context.Background(), ifState)
			}

		case <-ctx.Done():
			// Stop watching for state data updates.
			return
		}
	}
}

// Resync deletes old operation status of bridge domains in ETCD.
func (plugin *Plugin) resyncBdStateEvents(keys []string) error {
	for _, key := range keys {
		bdName, err := intf.ParseNameFromKey(key)
		if err != nil {
			return err
		}
		_, _, found := plugin.bdIndexes.LookupIdx(bdName)
		if !found {
			err := plugin.PublishStatistics.Put(key, nil)
			if err != nil {
				return err
			}
			plugin.Log.Debugf("Obsolete status for %v deleted", key)
		} else {
			plugin.Log.WithField("bdName", bdName).Debug("bridge domain status required")
		}
	}

	return nil
}

// PublishBdState is used to watch bridge domain state notifications.
func (plugin *Plugin) publishBdStateEvents(ctx context.Context) {
	plugin.wg.Add(1)
	defer plugin.wg.Done()

	for {
		select {
		case bdState := <-plugin.bdStateChan:
			if bdState != nil && bdState.State != nil && plugin.Publish != nil {
				key := l2.BridgeDomainStateKey(bdState.State.InternalName)
				// Remove BD state
				if bdState.State.Index == 0 && bdState.State.InternalName != "" {
					plugin.PublishStatistics.Put(key, nil)
					plugin.Log.Debugf("Bridge domain %v: state removed from ETCD", bdState.State.InternalName)
					// Write/Update BD state
				} else if bdState.State.Index != 0 {
					plugin.PublishStatistics.Put(key, bdState.State)
					plugin.Log.Debugf("Bridge domain %v: state stored in ETCD", bdState.State.InternalName)
				} else {
					plugin.Log.Warnf("Unable to process bridge domain state with Idx %v and Name %v",
						bdState.State.Index, bdState.State.InternalName)
				}
			}
		case <-ctx.Done():
			// Stop watching for state data updates.
			return
		}
	}
}
