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

package resync

import (
	"sync"
	"time"

	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
)

const (
	singleResyncTimeout = time.Second * 5
)

// Plugin implements Plugin interface, therefore it can be loaded with other plugins.
type Plugin struct {
	Deps

	mu            sync.Mutex
	regOrder      []string
	registrations map[string]*registration
}

// Deps groups dependencies injected into the plugin so that they are
// logically separated from other plugin fields.
type Deps struct {
	infra.PluginName
	Log logging.PluginLogger
}

// Init initializes variables.
func (p *Plugin) Init() error {
	p.registrations = make(map[string]*registration)
	return nil
}

// AfterInit method starts the resync.
func (p *Plugin) AfterInit() error {
	return nil
}

// Close TODO set flag that ignore errors => not start Resync while agent is stopping
// TODO kill existing Resync timeout while agent is stopping
func (p *Plugin) Close() error {
	//TODO close error report channel

	p.mu.Lock()
	defer p.mu.Unlock()

	p.registrations = make(map[string]*registration)

	return nil
}

// Register function is supposed to be called in Init() by all VPP Agent plugins.
// The plugins are supposed to load current state of their objects when newResync() is called.
// The actual CreateNewObjects(), DeleteObsoleteObjects() and ModifyExistingObjects() will be orchestrated
// to ensure their proper order. If an error occurs during Resync, then new Resync is planned.
func (p *Plugin) Register(resyncName string) Registration {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, found := p.registrations[resyncName]; found {
		p.Log.WithField("resyncName", resyncName).
			Panic("You are trying to register same resync twice")
		return nil
	}
	// ensure that resync is triggered in the same order as the plugins were registered
	p.regOrder = append(p.regOrder, resyncName)

	reg := newRegistration(resyncName, make(chan StatusEvent))
	p.registrations[resyncName] = reg

	return reg
}

// DoResync can be used to start resync procedure outside of after init
func (p *Plugin) DoResync() {
	p.startResync()
}

// Call callback on plugins to create/delete/modify objects.
func (p *Plugin) startResync() {
	if len(p.regOrder) == 0 {
		p.Log.Infof("No registrations, skipping resync")
		return
	}
	p.Log.Infof("Starting resync for: %+v", p.regOrder)

	resyncStart := time.Now()

	for _, regName := range p.regOrder {
		if reg, found := p.registrations[regName]; found {
			t := time.Now()
			p.startSingleResync(regName, reg)

			p.Log.Infof("Resync for %v took %v", regName, time.Since(t))
		}
	}

	p.Log.Infof("Resync complete (took: %v)", time.Since(resyncStart))

	// TODO check if there ReportError (if not than report) if error occurred even during Resync
}
func (p *Plugin) startSingleResync(resyncName string, reg *registration) {
	started := newStatusEvent(Started)

	reg.statusChan <- started

	select {
	case <-started.ReceiveAck():
		// ack
	case <-time.After(singleResyncTimeout):
		p.Log.WithField("regName", resyncName).Warn("Timeout of ACK")
	}
}
