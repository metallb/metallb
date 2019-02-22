// Copyright (c) 2018 Cisco and/or its affiliates.
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

// Test application just keeps running indefinitely, or for given time is defined
// via parameter creating a running process. The purpose is to serve as a test
// application for process manager example.

package main

import (
	"log"
	"path/filepath"
	"time"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/process"
	"github.com/ligato/cn-infra/process/status"
	"github.com/pkg/errors"
)

const pluginName = "process-manager-example"

func main() {
	pmPlugin := process.DefaultPlugin
	example := &PMExample{
		Log:      logging.ForPlugin(pluginName),
		PM:       &pmPlugin,
		finished: make(chan struct{}),
	}

	a := agent.NewAgent(
		agent.AllPlugins(example),
		agent.QuitOnClose(example.finished),
	)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

// PMExample demonstrates the usage of the process manager plugin.
type PMExample struct {
	Log logging.PluginLogger
	PM  process.API

	finished chan struct{}
}

// Init starts the example
func (p *PMExample) Init() error {
	go p.runExample()
	return nil
}

// Close frees the plugin resources.
func (p *PMExample) Close() error {
	return nil
}

// String returns name of the plugin.
func (p *PMExample) String() string {
	return pluginName
}

// Runs example with step by step description
func (p *PMExample) runExample() {
	// Process templates (create a template, start process using template)
	if err := p.templateExample(); err != nil {
		p.Log.Errorf("simple process manager example failed with error: %v", err)
	}
	close(p.finished)
	return
}

func (p *PMExample) templateExample() error {
	p.Log.Infof("starting template example")

	// Template example requires to have a template path defined in process manager config, and this config needs to be
	// provided to the example
	if _, err := p.PM.GetAllTemplates(); err != nil {
		p.Log.Warnf("template example aborted, no config file was provided for the process manager plugin")
	}

	// A template represents a process configuration defined by the model inside the process manager plugin.
	// The template allows to define all the setup items from previous examples (arguments, watcher, restarts, etc.).
	// The plugin API defines a method NewProcessFromTemplate(<template>). The template object can be programmed
	// manually as a *process.Template object. But main reason for templates to exist is that they are stored
	// in the filesystem as JSON objects, thus can persist an application restart.
	// To create a template file, the proto model has to be used. However template can be also generated from
	// new/attached process with options. There is an option 'Template' allowing it.
	// Lets create a new process with template. The template file will be created in path defined in plugin config.
	// The option has a run-on-startup parameter which can be set to true. If so, all the template processes will be
	// also started in the process manager init phase and available as soon as the application using it is loaded.
	cmd := filepath.Join("../", "test-process", "test-process")
	notifyChan := make(chan status.ProcessStatus)
	pr := p.PM.NewProcess("test-pr", cmd, process.Args("-max-uptime=60"), process.Notify(notifyChan),
		process.Template(false))

	// Start the watcher as before and ensure the process is running
	var state status.ProcessStatus
	go p.runWatcher("watcher-process", &state, notifyChan)
	if err := pr.Start(); err != nil {
		return err
	}
	p.Log.Infof("Let's wait for process to start")
	time.Sleep(2 * time.Second)
	if state == status.Sleeping || state == status.Running || state == status.Idle {
		p.Log.Infof("success!")
	} else {
		return errors.Errorf("failed to start the test process within timeout")
	}

	// Lets verify the JSON template file exists
	template, err := p.PM.GetTemplate(pr.GetName())
	if err != nil {
		return err
	}
	if template == nil {
		return errors.Errorf("expected template does not exists")
	}
	p.Log.Infof("template for test process was created")

	// Now we have the process template, so lets stop and remove the running process, in order to start it again with it
	p.Log.Infof("terminating running process...")
	if _, err := pr.StopAndWait(); err != nil {
		return err
	}
	prName := pr.GetName()
	if err := p.PM.Delete(prName); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	if prInst := p.PM.GetProcessByName(prName); prInst != nil {
		return errors.Errorf("expected terminated process instance is still running")
	}

	// Re-crate the plugin-process instance using template file and start it as usually with new watcher
	pr = p.PM.NewProcessFromTemplate(template)
	go p.runWatcher("watcher-template", &state, notifyChan)
	if err := pr.Start(); err != nil {
		return err
	}
	p.Log.Infof("Let's wait for template process to start")
	time.Sleep(2 * time.Second)
	if state == status.Sleeping || state == status.Running || state == status.Idle {
		p.Log.Infof("success!")
	} else {
		return errors.Errorf("failed to start the test process within timeout")
	}

	p.Log.Infof("template example finished")
	return nil
}

// Starts the process watcher using provided chanel. This watcher uses name since it is started multiple times
// during the example so it will be easier to distinguish the output.
func (p *PMExample) runWatcher(name string, state *status.ProcessStatus, notifyChan chan status.ProcessStatus) {
	for {
		select {
		case currentState, ok := <-notifyChan:
			if !ok {
				p.Log.Infof("===>(%s) process watcher ended", name)
				return
			}
			*state = currentState
			p.Log.Infof("===>(%s) received test process state: %s", name, currentState)
		}
	}
}
