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

package pluginstatusmap

import (
	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging/logrus"
)

// PluginStatusIdxMap provides map of plugin names to plugin status.
// Other plugins can watch changes to this map.
type PluginStatusIdxMap interface {
	// GetMapping returns internal read-only mapping with Value
	// of type interface{}.
	GetMapping() idxmap.NamedMapping

	// GetValue looks up previously stored status by plugin name in the mapping.
	GetValue(pluginName string) (data *status.PluginStatus, exists bool)

	// WatchNameToIdx allows to subscribe for watching changes in pluginStatusMap
	// mapping.
	WatchNameToIdx(subscriber infra.PluginName, pluginChannel chan PluginStatusEvent)
}

// PluginStatusIdxMapRW exposes not only PluginStatusIdxMap but also write methods.
type PluginStatusIdxMapRW interface {
	PluginStatusIdxMap

	// RegisterName adds new item into name-to-index mapping.
	Put(pluginName string, pluginStatus *status.PluginStatus)

	// UnregisterName removes an item identified by name from mapping
	Delete(pluginName string) (data *status.PluginStatus, exists bool)
}

// NewPluginStatusMap is a constructor for PluginStatusIdxMapRW.
func NewPluginStatusMap(owner infra.PluginName) PluginStatusIdxMapRW {
	return &pluginStatusMap{
		mapping: mem.NewNamedMapping(logrus.DefaultLogger(), "plugin_status", IndexPluginStatus),
	}
}

// pluginStatusMap is a type-safe implementation of PluginStatusIdxMap(RW).
type pluginStatusMap struct {
	mapping idxmap.NamedMappingRW
}

// PluginStatusEvent represents an item sent through the watch channel
// in PluginStatusMap.WatchNameToIdx().
// In contrast to NameToIdxDto it contains a typed Value.
type PluginStatusEvent struct {
	idxmap.NamedMappingEvent
	Value *status.PluginStatus
}

const (
	stateIndexKey = "stateKey"
)

// GetMapping returns internal read-only mapping.
// It is used in tests to inspect the content of the pluginStatusMap.
func (swi *pluginStatusMap) GetMapping() idxmap.NamedMapping {
	return swi.mapping
}

// RegisterName adds new item into the name-to-index mapping.
func (swi *pluginStatusMap) Put(pluginName string, pluginStatus *status.PluginStatus) {
	swi.mapping.Put(pluginName, pluginStatus)
}

// IndexPluginStatus creates indexes for plugin states and records the state
// passed as untyped data.
func IndexPluginStatus(data interface{}) map[string][]string {
	logrus.DefaultLogger().Debug("IndexPluginStatus ", data)

	indexes := map[string][]string{}
	pluginStatus, ok := data.(*status.PluginStatus)
	if !ok || pluginStatus == nil {
		return indexes
	}

	state := pluginStatus.State
	if state != 0 {
		indexes[stateIndexKey] = []string{state.String()}
	}
	return indexes
}

// UnregisterName removes an item identified by name from mapping
func (swi *pluginStatusMap) Delete(pluginName string) (data *status.PluginStatus, exists bool) {
	meta, exists := swi.mapping.Delete(pluginName)
	return swi.castdata(meta), exists
}

// GetValue looks up previously stored item identified by index in mapping.
func (swi *pluginStatusMap) GetValue(pluginName string) (data *status.PluginStatus, exists bool) {
	meta, exists := swi.mapping.GetValue(pluginName)
	if exists {
		data = swi.castdata(meta)
	}
	return data, exists
}

// LookupNameByIP returns names of items that contains given IP address in Value
func (swi *pluginStatusMap) LookupByState(state status.OperationalState) []string {
	return swi.mapping.ListNames(stateIndexKey, state.String())
}

func (swi *pluginStatusMap) castdata(meta interface{}) *status.PluginStatus {
	if pluginStatus, ok := meta.(*status.PluginStatus); ok {
		return pluginStatus
	}

	return nil
}

// WatchNameToIdx allows to subscribe for watching changes in pluginStatusMap mapping
func (swi *pluginStatusMap) WatchNameToIdx(subscriber infra.PluginName, pluginChannel chan PluginStatusEvent) {
	swi.mapping.Watch(string(subscriber), func(event idxmap.NamedMappingGenericEvent) {
		pluginChannel <- PluginStatusEvent{
			NamedMappingEvent: event.NamedMappingEvent,
			Value:             swi.castdata(event.Value),
		}
	})
}
