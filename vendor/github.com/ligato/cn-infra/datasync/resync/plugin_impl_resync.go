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

	regOrder      []string
	registrations map[string]Registration
	access        sync.Mutex
}

// Deps groups dependencies injected into the plugin so that they are
// logically separated from other plugin fields.
type Deps struct {
	infra.PluginName // inject
	Log              logging.PluginLogger
}

// Init initializes variables.
func (p *Plugin) Init() (err error) {
	p.registrations = make(map[string]Registration)

	//p.waingForResync = make(map[core.PluginName]*PluginEvent)
	//p.waingForResyncChan = make(chan *PluginEvent)
	//go p.watchWaingForResync()

	return nil
}

// AfterInit method starts the resync.
func (p *Plugin) AfterInit() (err error) {
	p.startResync()

	return nil
}

// Close TODO set flag that ignore errors => not start Resync while agent is stopping
// TODO kill existing Resync timeout while agent is stopping
func (p *Plugin) Close() error {
	//TODO close error report channel

	p.access.Lock()
	defer p.access.Unlock()

	p.registrations = make(map[string]Registration)

	return nil
}

// Register function is supposed to be called in Init() by all VPP Agent plugins.
// The plugins are supposed to load current state of their objects when newResync() is called.
// The actual CreateNewObjects(), DeleteObsoleteObjects() and ModifyExistingObjects() will be orchestrated
// to ensure their proper order. If an error occurs during Resync, then new Resync is planned.
func (p *Plugin) Register(resyncName string) Registration {
	p.access.Lock()
	defer p.access.Unlock()

	if _, found := p.registrations[resyncName]; found {
		p.Log.WithField("resyncName", resyncName).
			Panic("You are trying to register same resync twice")
		return nil
	}
	// ensure that resync is triggered in the same order as the plugins were registered
	p.regOrder = append(p.regOrder, resyncName)

	reg := NewRegistration(resyncName, make(chan StatusEvent, 0)) /*Zero to have back pressure*/
	p.registrations[resyncName] = reg

	return reg
}

// DoResync can be used to start resync procedure outside of after init
func (p *Plugin) DoResync() {
	p.startResync()
}

// Call callback on plugins to create/delete/modify objects.
func (p *Plugin) startResync() {
	p.Log.Info("Resync order", p.regOrder)

	startResyncTime := time.Now()

	for _, regName := range p.regOrder {
		if reg, found := p.registrations[regName]; found {
			startPartTime := time.Now()

			p.startSingleResync(regName, reg)

			took := time.Since(startPartTime)
			p.Log.WithField("durationInNs", took.Nanoseconds()).
				Infof("Resync of %v took %v", regName, took)
		}
	}

	took := time.Since(startResyncTime)
	p.Log.WithField("durationInNs", took.Nanoseconds()).Info("Resync took ", took)

	// TODO check if there ReportError (if not than report) if error occurred even during Resync
}
func (p *Plugin) startSingleResync(resyncName string, reg Registration) {
	started := newStatusEvent(Started)
	reg.StatusChan() <- started

	select {
	case <-started.ReceiveAck():
	case <-time.After(singleResyncTimeout):
		p.Log.WithField("regName", resyncName).Warn("Timeout of ACK")
	}
}
