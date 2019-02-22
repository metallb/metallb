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
	"strings"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
)

func (plugin *Plugin) changePropagateRequest(dataChng datasync.ProtoWatchResp) error {
	var err error
	key := dataChng.GetKey()

	plugin.Log.WithField("revision", dataChng.GetRevision()).
		Debugf("Processing change for key: %q", key)

	if strings.HasPrefix(key, interfaces.InterfaceKeyPrefix()) {
		var value, prevValue interfaces.LinuxInterfaces_Interface
		err = dataChng.GetValue(&value)
		if err != nil {
			return err
		}
		var diff bool
		diff, err = dataChng.GetPrevValue(&prevValue)
		if err == nil {
			err = plugin.dataChangeIface(diff, &value, &prevValue, dataChng.GetChangeType())
		}
	} else if strings.HasPrefix(key, l3.StaticArpKeyPrefix()) {
		var value, prevValue l3.LinuxStaticArpEntries_ArpEntry
		err = dataChng.GetValue(&value)
		if err != nil {
			return err
		}
		var diff bool
		diff, err = dataChng.GetPrevValue(&prevValue)
		if err == nil {
			err = plugin.dataChangeArp(diff, &value, &prevValue, dataChng.GetChangeType())
		}
	} else if strings.HasPrefix(key, l3.StaticRouteKeyPrefix()) {
		var value, prevValue l3.LinuxStaticRoutes_Route
		err = dataChng.GetValue(&value)
		if err != nil {
			return err
		}
		var diff bool
		diff, err = dataChng.GetPrevValue(&prevValue)
		if err == nil {
			err = plugin.dataChangeRoute(diff, &value, &prevValue, dataChng.GetChangeType())
		}
	} else {
		plugin.Log.Warn("ignoring change ", dataChng) //NOT ERROR!
	}
	return err
}

// DataChangeIface propagates data change to the ifConfigurator.
func (plugin *Plugin) dataChangeIface(diff bool, value *interfaces.LinuxInterfaces_Interface, prevValue *interfaces.LinuxInterfaces_Interface,
	changeType datasync.Op) error {
	plugin.Log.Debug("dataChangeIface ", diff, " ", changeType, " ", value, " ", prevValue)

	var err error
	if datasync.Delete == changeType {
		err = plugin.ifConfigurator.DeleteLinuxInterface(prevValue)
	} else if diff {
		err = plugin.ifConfigurator.ModifyLinuxInterface(value, prevValue)
	} else {
		err = plugin.ifConfigurator.ConfigureLinuxInterface(value)
	}
	return plugin.ifConfigurator.LogError(err)
}

// DataChangeArp propagates data change to the arpConfigurator
func (plugin *Plugin) dataChangeArp(diff bool, value *l3.LinuxStaticArpEntries_ArpEntry, prevValue *l3.LinuxStaticArpEntries_ArpEntry,
	changeType datasync.Op) error {
	plugin.Log.Debug("dataChangeArp ", diff, " ", changeType, " ", value, " ", prevValue)

	var err error
	if datasync.Delete == changeType {
		err = plugin.arpConfigurator.DeleteLinuxStaticArpEntry(prevValue)
	} else if diff {
		err = plugin.arpConfigurator.ModifyLinuxStaticArpEntry(value, prevValue)
	} else {
		err = plugin.arpConfigurator.ConfigureLinuxStaticArpEntry(value)
	}
	return plugin.arpConfigurator.LogError(err)
}

// DataChangeRoute propagates data change to the routeConfigurator
func (plugin *Plugin) dataChangeRoute(diff bool, value *l3.LinuxStaticRoutes_Route, prevValue *l3.LinuxStaticRoutes_Route,
	changeType datasync.Op) error {
	plugin.Log.Debug("dataChangeRoute ", diff, " ", changeType, " ", value, " ", prevValue)

	var err error
	if datasync.Delete == changeType {
		err = plugin.routeConfigurator.DeleteLinuxStaticRoute(prevValue)
	} else if diff {
		err = plugin.routeConfigurator.ModifyLinuxStaticRoute(value, prevValue)
	} else {
		err = plugin.routeConfigurator.ConfigureLinuxStaticRoute(value)
	}
	return plugin.routeConfigurator.LogError(err)
}
