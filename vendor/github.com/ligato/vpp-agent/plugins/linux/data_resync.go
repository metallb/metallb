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
	"fmt"
	"strings"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
)

// DataResyncReq is used to transfer expected configuration of the Linux network stack to the plugins.
type DataResyncReq struct {
	// Interfaces is a list af all interfaces that are expected to be in Linux after RESYNC.
	Interfaces []*interfaces.LinuxInterfaces_Interface
	// ARPs is a list af all arp entries that are expected to be in Linux after RESYNC.
	ARPs []*l3.LinuxStaticArpEntries_ArpEntry
	// Routes is a list af all routes that are expected to be in Linux after RESYNC.
	Routes []*l3.LinuxStaticRoutes_Route
}

// NewDataResyncReq is a constructor of object requirements which are expected to be re-synced.
func NewDataResyncReq() *DataResyncReq {
	return &DataResyncReq{
		// Interfaces is a list af all interfaces that are expected to be in Linux after RESYNC.
		Interfaces: []*interfaces.LinuxInterfaces_Interface{},
		// ARPs is a list af all arp entries that are expected to be in Linux after RESYNC.
		ARPs: []*l3.LinuxStaticArpEntries_ArpEntry{},
		// Routes is a list af all routes that are expected to be in Linux after RESYNC.
		Routes: []*l3.LinuxStaticRoutes_Route{},
	}
}

// DataResync delegates resync request linuxplugin configurators.
func (plugin *Plugin) resyncPropageRequest(req *DataResyncReq) error {
	plugin.Log.Info("resync the Linux Configuration")

	// store all resync errors
	var resyncErrs []error

	if err := plugin.ifConfigurator.Resync(req.Interfaces); err != nil {
		resyncErrs = append(resyncErrs, plugin.ifConfigurator.LogError(err))
	}

	if err := plugin.arpConfigurator.Resync(req.ARPs); err != nil {
		resyncErrs = append(resyncErrs, plugin.arpConfigurator.LogError(err))
	}

	if err := plugin.routeConfigurator.Resync(req.Routes); err != nil {
		resyncErrs = append(resyncErrs, plugin.routeConfigurator.LogError(err))
	}

	// log errors if any
	if len(resyncErrs) == 0 {
		return nil
	}
	for _, err := range resyncErrs {
		plugin.Log.Error(err)
	}

	return fmt.Errorf("%v errors occured during linuxplugin resync", len(resyncErrs))
}

func (plugin *Plugin) resyncParseEvent(resyncEv datasync.ResyncEvent) *DataResyncReq {
	req := NewDataResyncReq()
	for key, resyncData := range resyncEv.GetValues() {
		plugin.Log.Debug("Received RESYNC key ", key)
		if strings.HasPrefix(key, interfaces.InterfaceKeyPrefix()) {
			plugin.resyncAppendInterface(resyncData, req)
		} else if strings.HasPrefix(key, l3.StaticArpKeyPrefix()) {
			plugin.resyncAppendARPs(resyncData, req)
		} else if strings.HasPrefix(key, l3.StaticRouteKeyPrefix()) {
			plugin.resyncAppendRoutes(resyncData, req)
		} else {
			plugin.Log.Warn("ignoring ", resyncEv)
		}
	}
	return req
}

func (plugin *Plugin) resyncAppendInterface(iterator datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if interfaceData, stop := iterator.GetNext(); stop {
			break
		} else {
			value := &interfaces.LinuxInterfaces_Interface{}
			if err := interfaceData.GetValue(value); err != nil {
				plugin.Log.Errorf("error getting value of Linux interface: %v", err)
				continue
			}
			req.Interfaces = append(req.Interfaces, value)
			num++

			plugin.Log.WithField("revision", interfaceData.GetRevision()).
				Debugf("Processing resync for key: %q", interfaceData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC Linux interface values %d", num)
}

func (plugin *Plugin) resyncAppendARPs(iterator datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if arpData, stop := iterator.GetNext(); stop {
			break
		} else {
			value := &l3.LinuxStaticArpEntries_ArpEntry{}
			if err := arpData.GetValue(value); err != nil {
				plugin.Log.Errorf("error getting value of Linux ARP: %v", err)
				continue
			}
			req.ARPs = append(req.ARPs, value)
			num++

			plugin.Log.WithField("revision", arpData.GetRevision()).
				Debugf("Processing resync for key: %q", arpData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC Linux ARP entry values %d", num)
}

func (plugin *Plugin) resyncAppendRoutes(iterator datasync.KeyValIterator, req *DataResyncReq) {
	num := 0
	for {
		if routeData, stop := iterator.GetNext(); stop {
			break
		} else {
			value := &l3.LinuxStaticRoutes_Route{}
			if err := routeData.GetValue(value); err != nil {
				plugin.Log.Errorf("error getting value of Linux ARP: %v", err)
				continue
			}
			req.Routes = append(req.Routes, value)
			num++

			plugin.Log.WithField("revision", routeData.GetRevision()).
				Debugf("Processing resync for key: %q", routeData.GetKey())
		}
	}

	plugin.Log.Debugf("Received RESYNC Linux Route values %d", num)
}

func (plugin *Plugin) subscribeWatcher() (err error) {
	plugin.Log.Debug("subscribeWatcher begin")
	plugin.ifIndexes.WatchNameToIdx(plugin.String(), plugin.ifIndexesWatchChan)
	plugin.watchDataReg, err = plugin.Watcher.
		Watch("linuxplugin", plugin.changeChan, plugin.resyncChan,
			interfaces.InterfaceKeyPrefix(),
			l3.StaticArpKeyPrefix(),
			l3.StaticRouteKeyPrefix())
	if err != nil {
		return err
	}

	plugin.Log.Debug("data watcher watch finished")

	return nil
}
