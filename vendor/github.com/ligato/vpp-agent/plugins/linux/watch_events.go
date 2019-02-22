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

package linux

import (
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	ifaceVPP "github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"golang.org/x/net/context"
)

// WatchEvents goroutine is used to watch for changes in the northbound configuration.
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
		case e := <-plugin.resyncChan:
			runWithMutex(func() {
				plugin.onResyncEvent(e)
			})

		case e := <-plugin.changeChan:
			runWithMutex(func() {
				plugin.onChangeEvent(e)
			})

		case ms := <-plugin.msChan:
			runWithMutex(func() {
				plugin.nsHandler.HandleMicroservices(ms)
			})

		case e := <-plugin.ifIndexesWatchChan:
			runWithMutex(func() {
				plugin.onLinuxIfaceEvent(e)
			})

		case e := <-plugin.vppIfIndexesWatchChan:
			runWithMutex(func() {
				plugin.onVppIfaceEvent(e)
			})

		case <-ctx.Done():
			plugin.Log.Debug("Stop watching events")
			return
		}
	}
}

func (plugin *Plugin) onResyncEvent(e datasync.ResyncEvent) {
	req := plugin.resyncParseEvent(e)
	err := plugin.resyncPropageRequest(req)
	e.Done(err)
}

func (plugin *Plugin) onChangeEvent(e datasync.ChangeEvent) {
	var err error
	for _, dataChng := range e.GetChanges() {
		chngErr := plugin.changePropagateRequest(dataChng)
		if chngErr != nil {
			err = chngErr
		}
	}
	e.Done(err)
}

func (plugin *Plugin) onLinuxIfaceEvent(e ifaceidx.LinuxIfIndexDto) {
	if e.IsDelete() {
		if err := plugin.arpConfigurator.ResolveDeletedInterface(e.Name, e.Idx); err != nil {
			plugin.arpConfigurator.LogError(err)
		}
		if err := plugin.routeConfigurator.ResolveDeletedInterface(e.Name, e.Idx); err != nil {
			plugin.routeConfigurator.LogError(err)
		}
	} else {
		if err := plugin.arpConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.arpConfigurator.LogError(err)
		}
		if err := plugin.routeConfigurator.ResolveCreatedInterface(e.Name, e.Idx); err != nil {
			plugin.routeConfigurator.LogError(err)
		}
	}
	e.Done()
}

func (plugin *Plugin) onVppIfaceEvent(e ifaceVPP.SwIfIdxDto) {
	if e.IsDelete() {
		if err := plugin.ifConfigurator.ResolveDeletedVPPInterface(e.Metadata); err != nil {
			plugin.ifConfigurator.LogError(err)
		}
	} else {
		if err := plugin.ifConfigurator.ResolveCreatedVPPInterface(e.Metadata); err != nil {
			plugin.ifConfigurator.LogError(err)
		}
	}
	e.Done()
}
