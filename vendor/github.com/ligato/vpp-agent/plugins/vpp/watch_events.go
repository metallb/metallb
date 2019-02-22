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
	"strings"

	"github.com/ligato/cn-infra/datasync"
	linux_ifaceidx "github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"golang.org/x/net/context"
)

// WatchEvents goroutine is used to watch for changes in the northbound configuration & NameToIdxMapping notifications.
func (plugin *Plugin) watchEvents(ctx context.Context) {
	plugin.wg.Add(1)
	defer plugin.wg.Done()

	runWithMutex := func(fn func()) {
		if plugin.WatchEventsMutex != nil {
			plugin.WatchEventsMutex.Lock()
			defer plugin.WatchEventsMutex.Unlock()
		}
		fn()
	}

	for {
		select {
		case e := <-plugin.resyncConfigChan:
			runWithMutex(func() {
				plugin.onResyncEvent(e)
			})

		case e := <-plugin.resyncStatusChan:
			runWithMutex(func() {
				plugin.onStatusResyncEvent(e)
			})

		case e := <-plugin.changeChan:
			runWithMutex(func() {
				plugin.onChangeEvent(e)
			})

		case e := <-plugin.ifIdxWatchCh:
			runWithMutex(func() {
				plugin.onVppIfaceEvent(e)
			})

		case e := <-plugin.linuxIfIdxWatchCh:
			runWithMutex(func() {
				plugin.onLinuxIfaceEvent(e)
			})

		case e := <-plugin.bdIdxWatchCh:
			runWithMutex(func() {
				plugin.onVppBdEvent(e)
			})

		case <-ctx.Done():
			plugin.Log.Debug("Stop watching events")
			return
		}
	}
}

func (plugin *Plugin) onResyncEvent(e datasync.ResyncEvent) {
	req := plugin.resyncParseEvent(e)
	var err error
	if plugin.resyncStrategy == skipResync {
		// skip resync
		plugin.Log.Info("skip VPP resync strategy chosen, VPP resync is omitted")
	} else if plugin.resyncStrategy == optimizeColdStart {
		// optimize resync
		err = plugin.resyncConfigPropageOptimizedRequest(req)
	} else {
		// full resync
		err = plugin.resyncConfigPropageFullRequest(req)
	}
	e.Done(err)
}

func (plugin *Plugin) onStatusResyncEvent(e datasync.ResyncEvent) {
	var wasError error
	for key, vals := range e.GetValues() {
		plugin.Log.Debugf("trying to delete obsolete status for key %v begin ", key)
		if strings.HasPrefix(key, interfaces.StatePrefix) {
			var keys []string
			for {
				x, stop := vals.GetNext()
				if stop {
					break
				}
				keys = append(keys, x.GetKey())
			}
			if len(keys) > 0 {
				err := plugin.resyncIfStateEvents(keys)
				if err != nil {
					wasError = err
				}
			}
		} else if strings.HasPrefix(key, l2.BdStatePrefix) {
			var keys []string
			for {
				x, stop := vals.GetNext()
				if stop {
					break
				}
				keys = append(keys, x.GetKey())
			}
			if len(keys) > 0 {
				err := plugin.resyncBdStateEvents(keys)
				if err != nil {
					wasError = err
				}
			}
		}
	}
	e.Done(wasError)
}

func (plugin *Plugin) onChangeEvent(e datasync.ChangeEvent) {
	// For asynchronous calls only: if changePropagateRequest ends up without errors,
	// the dataChng.Done is called in particular vppcall, otherwise the dataChng.Done is called here.

	var err error

	for _, dataChng := range e.GetChanges() {
		callback := func(cbErr error) {
			if cbErr != nil {
				err = cbErr
			}
		}
		callbackCalled, err := plugin.changePropagateRequest(dataChng, callback)
		plugin.errorChannel <- ErrCtx{dataChng, err}
		if !callbackCalled {
			callback(err)
		}
	}

	// When the request propagation is complete, send the error context (even if the error is nil).
	e.Done(err)
}

func (plugin *Plugin) onVppIfaceEvent(e ifaceidx.SwIfIdxDto) {
	if e.IsDelete() {
		if err := plugin.aclConfigurator.ResolveDeletedInterface(e.Name, e.Idx); err != nil {
			plugin.aclConfigurator.LogError(err)
		}
		if err := plugin.arpConfigurator.ResolveDeletedInterface(e.Name, e.Idx); err != nil {
			plugin.arpConfigurator.LogError(err)
		}
		if err := plugin.proxyArpConfigurator.ResolveDeletedInterface(e.Name); err != nil {
			plugin.proxyArpConfigurator.LogError(err)
		}
		if err := plugin.bdConfigurator.ResolveDeletedInterface(e.Name); err != nil {
			plugin.bdConfigurator.LogError(err)
		}
		plugin.fibConfigurator.ResolveDeletedInterface(e.Name, e.Idx, func(err error) {
			if err != nil {
				plugin.fibConfigurator.LogError(err)
			}
		})
		if err := plugin.xcConfigurator.ResolveDeletedInterface(e.Name); err != nil {
			plugin.xcConfigurator.LogError(err)
		}
		if err := plugin.appNsConfigurator.ResolveDeletedInterface(e.Name, e.Idx); err != nil {
			plugin.appNsConfigurator.LogError(err)
		}
		if err := plugin.stnConfigurator.ResolveDeletedInterface(e.Name); err != nil {
			plugin.stnConfigurator.LogError(err)
		}
		if err := plugin.routeConfigurator.ResolveDeletedInterface(e.Name, e.Idx); err != nil {
			plugin.routeConfigurator.LogError(err)
		}
		if err := plugin.natConfigurator.ResolveDeletedInterface(e.Name, e.Idx); err != nil {
			plugin.natConfigurator.LogError(err)
		}
		if err := plugin.ipSecConfigurator.ResolveDeletedInterface(e.Name, e.Idx); err != nil {
			plugin.ipSecConfigurator.LogError(err)
		}
		// TODO propagate error
	} else if e.IsUpdate() {
		// Nothing to do here
	} else {
		// Keep order.
		if err := plugin.aclConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.aclConfigurator.LogError(err)
		}
		if err := plugin.arpConfigurator.ResolveCreatedInterface(e.Name); err != nil {
			plugin.arpConfigurator.LogError(err)
		}
		if err := plugin.proxyArpConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.proxyArpConfigurator.LogError(err)
		}
		if err := plugin.bdConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.bdConfigurator.LogError(err)
		}
		plugin.fibConfigurator.ResolveCreatedInterface(e.Name, e.Idx, func(err error) {
			if err != nil {
				plugin.fibConfigurator.LogError(err)
			}
		})
		if err := plugin.xcConfigurator.ResolveCreatedInterface(e.Name); err != nil {
			plugin.xcConfigurator.LogError(err)
		}
		if err := plugin.appNsConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.appNsConfigurator.LogError(err)
		}
		if err := plugin.stnConfigurator.ResolveCreatedInterface(e.Name); err != nil {
			plugin.stnConfigurator.LogError(err)
		}
		if err := plugin.routeConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.routeConfigurator.LogError(err)
		}
		if err := plugin.natConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.natConfigurator.LogError(err)
		}
		if err := plugin.ipSecConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.ipSecConfigurator.LogError(err)
		}
		// TODO propagate error
	}
	e.Done()
}

func (plugin *Plugin) onLinuxIfaceEvent(e linux_ifaceidx.LinuxIfIndexDto) {
	var hostIfName string
	if e.Metadata != nil && e.Metadata.Data != nil && e.Metadata.Data.HostIfName != "" {
		hostIfName = e.Metadata.Data.HostIfName
	}
	var err error
	if !e.IsDelete() {
		err = plugin.ifConfigurator.ResolveCreatedLinuxInterface(e.Name, hostIfName, e.Idx)
	} else {
		err = plugin.ifConfigurator.ResolveDeletedLinuxInterface(e.Name, hostIfName, e.Idx)
	}
	plugin.ifConfigurator.LogError(err)
	e.Done()
}

func (plugin *Plugin) onVppBdEvent(e l2idx.BdChangeDto) {
	if e.IsDelete() {
		plugin.fibConfigurator.ResolveDeletedBridgeDomain(e.Name, e.Idx, func(err error) {
			if err != nil {
				plugin.fibConfigurator.LogError(err)
			}
		})
		// TODO propagate error
	} else if e.IsUpdate() {
		plugin.fibConfigurator.ResolveUpdatedBridgeDomain(e.Name, e.Idx, func(err error) {
			if err != nil {
				plugin.fibConfigurator.LogError(err)
			}
		})
		// TODO propagate error
	} else {
		plugin.fibConfigurator.ResolveCreatedBridgeDomain(e.Name, e.Idx, func(err error) {
			if err != nil {
				plugin.fibConfigurator.LogError(err)
			}
		})
		// TODO propagate error
	}
	e.Done()
}
