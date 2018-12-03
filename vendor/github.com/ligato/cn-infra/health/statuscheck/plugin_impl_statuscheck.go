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

package statuscheck

import (
	"context"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
)

var (
	// PeriodicWriteTimeout is frequency of periodic writes of state data into ETCD.
	PeriodicWriteTimeout = time.Second * 10
	// PeriodicProbingTimeout is frequency of periodic plugin state probing.
	PeriodicProbingTimeout = time.Second * 5
)

// Plugin struct holds all plugin-related data.
type Plugin struct {
	Deps

	access sync.Mutex // lock for the Plugin data

	agentStat     *status.AgentStatus             // overall agent status
	interfaceStat *status.InterfaceStats          // interfaces' overall status
	pluginStat    map[string]*status.PluginStatus // plugin's status
	pluginProbe   map[string]PluginStateProbe     // registered status probes

	ctx    context.Context
	cancel context.CancelFunc // cancel can be used to cancel all goroutines and their jobs inside of the plugin
	wg     sync.WaitGroup     // wait group that allows to wait until all goroutines of the plugin have finished
}

// Deps lists the dependencies of statuscheck plugin.
type Deps struct {
	infra.PluginName                            // inject
	Log              logging.PluginLogger       // inject
	Transport        datasync.KeyProtoValWriter // inject (optional)
}

// Init prepares the initial status data.
func (p *Plugin) Init() error {
	// write initial status data into ETCD
	p.agentStat = &status.AgentStatus{
		State:        status.OperationalState_INIT,
		BuildVersion: agent.BuildVersion,
		BuildDate:    agent.BuildDate,
		CommitHash:   agent.CommitHash,
		StartTime:    time.Now().Unix(),
		LastChange:   time.Now().Unix(),
	}

	// initial empty interface status
	p.interfaceStat = &status.InterfaceStats{}

	// init pluginStat map
	p.pluginStat = make(map[string]*status.PluginStatus)

	// init map with plugin state probes
	p.pluginProbe = make(map[string]PluginStateProbe)

	// prepare context for all go routines
	p.ctx, p.cancel = context.WithCancel(context.Background())

	return nil
}

// AfterInit starts go routines for periodic probing and periodic updates.
// Initial state data are published via the injected transport.
func (p *Plugin) AfterInit() error {
	p.access.Lock()
	defer p.access.Unlock()

	// do periodic status probing for plugins that have provided the probe function
	go p.periodicProbing(p.ctx)

	// do periodic updates of the state data in ETCD
	go p.periodicUpdates(p.ctx)

	p.publishAgentData()

	// transition to OK state if there are no plugins
	if len(p.pluginStat) == 0 {
		p.agentStat.State = status.OperationalState_OK
		p.agentStat.LastChange = time.Now().Unix()
		p.publishAgentData()
	}

	return nil
}

// Close stops go routines for periodic probing and periodic updates.
func (p *Plugin) Close() error {
	p.cancel()
	p.wg.Wait()

	return nil
}

// Register a plugin for status change reporting.
func (p *Plugin) Register(pluginName infra.PluginName, probe PluginStateProbe) {
	p.access.Lock()
	defer p.access.Unlock()

	stat := &status.PluginStatus{
		State:      status.OperationalState_INIT,
		LastChange: time.Now().Unix(),
	}
	p.pluginStat[string(pluginName)] = stat

	if probe != nil {
		p.pluginProbe[string(pluginName)] = probe
	}

	// write initial status data into ETCD
	p.publishPluginData(pluginName, stat)

	p.Log.Infof("Plugin %v: status check probe registered", pluginName)
}

// ReportStateChange can be used to report a change in the status of a previously registered plugin.
func (p *Plugin) ReportStateChange(pluginName infra.PluginName, state PluginState, lastError error) {
	p.reportStateChange(pluginName, state, lastError)
}

// ReportStateChangeWithMeta can be used to report a change in the status of a previously registered plugin and report
// the specific metadata state
func (p *Plugin) ReportStateChangeWithMeta(pluginName infra.PluginName, state PluginState, lastError error, meta proto.Message) {
	p.reportStateChange(pluginName, state, lastError)

	switch data := meta.(type) {
	case *status.InterfaceStats_Interface:
		p.reportInterfaceStateChange(data)
	default:
		p.Log.Debug("Unknown type of status metadata")
	}
}

func (p *Plugin) reportStateChange(pluginName infra.PluginName, state PluginState, lastError error) {
	p.access.Lock()
	defer p.access.Unlock()

	stat, ok := p.pluginStat[string(pluginName)]
	if !ok {
		p.Log.Errorf("Unregistered plugin %s is reporting the state, ignoring.", pluginName)
		return
	}

	// update the state only if it has really changed
	changed := true
	if stateToProto(state) == stat.State {
		if lastError == nil && stat.Error == "" {
			changed = false
		}
		if lastError != nil && lastError.Error() == stat.Error {
			changed = false
		}
	}
	if !changed {
		return
	}

	p.Log.WithFields(map[string]interface{}{"plugin": pluginName, "state": state, "lastErr": lastError}).Info(
		"Agent plugin state update.")

	// update plugin state
	stat.State = stateToProto(state)
	stat.LastChange = time.Now().Unix()
	if lastError != nil {
		stat.Error = lastError.Error()
	} else {
		stat.Error = ""
	}
	p.publishPluginData(pluginName, stat)

	// update global state
	p.agentStat.State = stateToProto(state)
	p.agentStat.LastChange = time.Now().Unix()
	// Status for existing plugin
	var lastErr string
	if lastError != nil {
		lastErr = lastError.Error()
	}
	var pluginStatusExists bool
	for _, pluginStatus := range p.agentStat.Plugins {
		if pluginStatus.Name == pluginName.String() {
			pluginStatusExists = true
			pluginStatus.State = stateToProto(state)
			pluginStatus.Error = lastErr
		}
	}
	// Status for new plugin
	if !pluginStatusExists {
		p.agentStat.Plugins = append(p.agentStat.Plugins, &status.PluginStatus{
			Name:  pluginName.String(),
			State: stateToProto(state),
			Error: lastErr,
		})
	}
	p.publishAgentData()
}

func (p *Plugin) reportInterfaceStateChange(data *status.InterfaceStats_Interface) {
	p.access.Lock()
	defer p.access.Unlock()

	// Filter interfaces without internal name
	if data.InternalName == "" {
		p.Log.Debugf("Interface without internal name skipped for global status. Data: %v", data)
		return
	}

	// update only if state really changed
	var ifIndex int
	var existingData *status.InterfaceStats_Interface
	for index, ifState := range p.interfaceStat.Interfaces {
		// check if interface with the internal name already exists
		if data.InternalName == ifState.InternalName {
			ifIndex = index
			existingData = ifState
			break
		}
	}

	if existingData == nil {
		// new entry
		p.interfaceStat.Interfaces = append(p.interfaceStat.Interfaces, data)
		p.Log.Debugf("Global interface state data added: %v", data)
	} else if existingData.Index != data.Index || existingData.Status != data.Status || existingData.MacAddress != data.MacAddress {
		// updated entry - update only if state really changed
		p.interfaceStat.Interfaces = append(append(p.interfaceStat.Interfaces[:ifIndex], data), p.interfaceStat.Interfaces[ifIndex+1:]...)
		p.Log.Debug("Global interface state data updated: %v", data)
	}
}

// publishAgentData writes the current global agent state into ETCD.
func (p *Plugin) publishAgentData() error {
	p.agentStat.LastUpdate = time.Now().Unix()
	if p.Transport != nil {
		return p.Transport.Put(status.AgentStatusKey(), p.agentStat)
	}
	return nil
}

// publishPluginData writes the current plugin state into ETCD.
func (p *Plugin) publishPluginData(pluginName infra.PluginName, pluginStat *status.PluginStatus) error {
	pluginStat.LastUpdate = time.Now().Unix()
	if p.Transport != nil {
		return p.Transport.Put(status.PluginStatusKey(string(pluginName)), pluginStat)
	}
	return nil
}

// publishAllData publishes global agent + all plugins state data into ETCD.
func (p *Plugin) publishAllData() {
	p.access.Lock()
	defer p.access.Unlock()

	p.publishAgentData()
	for name, s := range p.pluginStat {
		p.publishPluginData(infra.PluginName(name), s)
	}
}

// periodicProbing does periodic status probing for all plugins
// that have registered probe functions.
func (p *Plugin) periodicProbing(ctx context.Context) {
	p.wg.Add(1)
	defer p.wg.Done()

	for {
		select {
		case <-time.After(PeriodicProbingTimeout):
			for pluginName, probe := range p.pluginProbe {
				state, lastErr := probe()
				p.ReportStateChange(infra.PluginName(pluginName), state, lastErr)
				// just check in-between probes if the plugin is closing
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

// periodicUpdates does periodic writes of state data into ETCD.
func (p *Plugin) periodicUpdates(ctx context.Context) {
	p.wg.Add(1)
	defer p.wg.Done()

	for {
		select {
		case <-time.After(PeriodicWriteTimeout):
			p.publishAllData()

		case <-ctx.Done():
			return
		}
	}
}

// getAgentState return current global operational state of the agent.
func (p *Plugin) getAgentState() status.OperationalState {
	p.access.Lock()
	defer p.access.Unlock()
	return p.agentStat.State
}

// GetAllPluginStatus returns a map containing pluginname and its status, for all plugins
func (p *Plugin) GetAllPluginStatus() map[string]*status.PluginStatus {
	//TODO - used currently, will be removed after incoporating improvements for exposing copy of map
	p.access.Lock()
	defer p.access.Unlock()

	return p.pluginStat
}

// GetInterfaceStats returns current global operational status of interfaces
func (p *Plugin) GetInterfaceStats() status.InterfaceStats {
	p.access.Lock()
	defer p.access.Unlock()

	return *p.interfaceStat
}

// GetAgentStatus return current global operational state of the agent.
func (p *Plugin) GetAgentStatus() status.AgentStatus {
	p.access.Lock()
	defer p.access.Unlock()
	return *p.agentStat
}

// stateToProto converts agent state type into protobuf agent state type.
func stateToProto(state PluginState) status.OperationalState {
	switch state {
	case Init:
		return status.OperationalState_INIT
	case OK:
		return status.OperationalState_OK
	default:
		return status.OperationalState_ERROR
	}
}
