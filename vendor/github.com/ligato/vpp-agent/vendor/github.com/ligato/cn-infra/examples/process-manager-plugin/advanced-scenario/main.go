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
	// advanced process handing (attach running process, read status file)
	if err := p.advancedExample(); err != nil {
		p.Log.Errorf("simple process manager example failed with error: %v", err)
	}
	close(p.finished)
	return
}

func (p *PMExample) advancedExample() error {
	p.Log.Infof("starting advanced process manager example")
	var err error
	// Let's prepare the application as in basic scenario with some additional options which are 'Detach'
	// and 'Restarts'. Spawned process is by default a child process of the caller. It means that the child
	// process will be automatically terminated together with the parent. Option 'Detach' allows to detach
	// the process from parent and keeps is running.
	// Option 'Restarts' defines a number of automatic restarts if given process is terminated.
	cmd := filepath.Join("../", "test-process", "test-process")
	notifyChan := make(chan status.ProcessStatus)
	pr := p.PM.NewProcess("test-pr", cmd, process.Args("-max-uptime=60"), process.Notify(notifyChan),
		process.Detach(), process.Restarts(1))

	// Start the watcher as before and ensure the process is running
	var state status.ProcessStatus
	go p.runWatcher("watcher-old", &state, notifyChan)
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

	// The example cannot simulate parent restart, so we use delete to 'forget' the process and keep it running. But first,
	// we need to remember the process ID
	pid := pr.GetPid()
	p.Log.Infof("PID: %d", pid)

	// Make sure the process is known to plugin
	prInst := p.PM.GetProcessByName(pr.GetName())
	if prInst == nil {
		return errors.Errorf("expected process instance is nil")
	}

	// Now delete the process
	p.Log.Infof("Deleting process...")
	if err = p.PM.Delete(pr.GetName()); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)

	// And make sure the process is NOT known to plugin
	prInst = p.PM.GetProcessByName(pr.GetName())
	if prInst != nil {
		return errors.Errorf("process expected to be removed still exists")
	}

	// Since we know the PID, the plugin can reattach to the same instance it created with a new name. From
	// the plugin perspective, attaching the process is just another way of creating it, so all the options have to be
	// re-defined. Note: it is possible to attach to process without command or arguments, but it is not possible
	// to start such a process instance. The notify channel need to be re-initialized and new watcher started, because
	// the previous one was closed by the Delete().
	p.Log.Infof("Reattaching process...")
	notifyChan = make(chan status.ProcessStatus)
	go p.runWatcher("watcher-new", &state, notifyChan)
	if pr, err = p.PM.AttachProcess("test-pr-attached", cmd, pid, process.Args("-max-uptime=60"), process.Notify(notifyChan),
		process.Detach(), process.Restarts(1)); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)

	// Make sure the process is known again to plugin
	prInst = p.PM.GetProcessByName(pr.GetName())
	if prInst == nil {
		return errors.Errorf("expected process instance is nil")
	}
	p.Log.Infof("success!")
	p.Log.Infof("reattached PID: %d", pid)

	// Since the restart count is set to 1, process will be restarted if terminated. It does not matter how, so let's
	// just stop it.
	if _, err = pr.StopAndWait(); err != nil {
		return err
	}
	p.Log.Infof("Let's wait while the process is stopped and restarted")
	time.Sleep(3 * time.Second)
	if state == status.Sleeping || state == status.Running || state == status.Idle {
		p.Log.Infof("success!")
	} else {
		return errors.Errorf("failed to re-start the test process within timeout")
	}
	// Process was stopped and auto-started again with new PID
	p.Log.Infof("PID after auto-restart: %d", pr.GetPid())

	// Every process creates a status file within /proc/<pid>/status with a plenty of information about the process
	// state, CPU or memory usage, etc. The process watcher periodically reads the status data for current state to
	// propagate changes. To read status, use ReadStatus().
	prStatus, err := pr.ReadStatus(pr.GetPid())
	if err != nil {
		return err
	}
	// Some example status data
	p.Log.Infof("Threads: %d", prStatus.Threads)
	p.Log.Infof("Allowed CPUs: %s", prStatus.CpusAllowed)
	p.Log.Infof("Parent process ID: %d", prStatus.PPid)
	p.Log.Infof("Total program size: %s", prStatus.VMSize)

	// Stop and delete the process. It will not be run again, since maximum of restarts was set to 1.
	p.Log.Infof("Stopping and removing the process...")
	if _, err := pr.StopAndWait(); err != nil {
		return err
	}
	if err := p.PM.Delete(pr.GetName()); err != nil {
		return err
	}
	p.Log.Infof("done")
	p.Log.Infof("advanced process manager example is completed")

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
