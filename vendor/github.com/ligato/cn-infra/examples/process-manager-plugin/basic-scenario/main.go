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
	// Simple process handling (start, restart, watch, delete)
	if err := p.simpleExample(); err != nil {
		p.Log.Errorf("simple process manager example failed with error: %v", err)
	}
	close(p.finished)
	return
}

func (p *PMExample) simpleExample() error {
	p.Log.Infof("starting simple process manager example")
	// Process manager plugin has internal cache for all processes created via its API. These processes
	// are considered as known to plugin. The example uses test-process - a simple application which
	// keeps running after start, so it can be handled by process manager.
	// At first, the process needs to be defined with unique name and command. Since test process allows
	// to use argument, use option 'Args()' to start the application with it and set max uptime to 60 seconds.
	// Then initialize status channel since status notifications are required, and provide it to the plugin
	// via 'Notify()'.
	cmd := filepath.Join("../", "test-process", "test-process")
	notifyChan := make(chan status.ProcessStatus)
	pr := p.PM.NewProcess("test-pr", cmd, process.Args("-max-uptime=60"), process.Notify(notifyChan))

	// The process is initialized. We can verify that via plugin manager API (prInst == pr)
	prInst := p.PM.GetProcessByName("test-pr")
	if prInst == nil {
		return errors.Errorf("expected process instance is nil")
	}

	// Since the watch channel is used, start watcher in another goroutine. The watcher will track the process status
	// during the example.
	var state status.ProcessStatus
	go p.runWatcher(&state, notifyChan)

	if err := pr.Start(); err != nil {
		return err
	}
	p.Log.Infof("Let's wait for the process to start")
	time.Sleep(2 * time.Second)
	if state == status.Sleeping || state == status.Running || state == status.Idle {
		p.Log.Infof("success!")
	} else {
		return errors.Errorf("failed to start the test process within timeout")
	}

	// Now the process is running and we can use the instance to read various process definitions. The most common:
	pid := pr.GetPid()
	p.Log.Infof("PID: %d", pid)
	p.Log.Infof("Instance name: %s", pr.GetInstanceName())
	p.Log.Infof("Start time: %d", pr.GetStartTime().Nanosecond())
	p.Log.Infof("Uptime (s): %f", pr.GetUptime().Seconds())

	// Let's try to restart the process
	if err := pr.Restart(); err != nil {
		return err
	}
	p.Log.Infof("Let's wait for process to restart")
	time.Sleep(2 * time.Second)

	// Restarted process is expected to have different process ID. We stored old PID, so let's compare it with
	// the new one.
	if pid == pr.GetPid() {
		p.Log.Warnf("PID of restarted process is the same, perhaps the restart was not successful")
	} else {
		p.Log.Infof("success!")
	}
	p.Log.Infof("new PID: %d", pid)
	p.Log.Infof("Uptime (s): %f", pr.GetUptime().Seconds())

	// Now lets stop the process
	if err := pr.Stop(); err != nil {
		return err
	}
	p.Log.Infof("Let's wait for process to stop")
	time.Sleep(2 * time.Second)

	// Important: we stopped the process using SIGTERM, which causes process to become defunct because with the current
	// setup, the example is a parent process which is still running. If we try to start it again with 'Start()',
	// the operating system spawns a new process which instance will NOT be passed to the plugin manager by os package
	// and the new process will become unmanageable.
	// There are several options what to do:
	// * use Wait(). It waits for process status and terminates it completely.
	// * stop the process with the StopAndWait() which merges the two procedures together
	// * create a process with 'AutoTermination' option. It causes that every managed process which becomes zombie
	// will be terminated automatically.
	// Since we already stopped the process, the only thing we can do is to wait
	if _, err := pr.Wait(); err != nil {
		return err
	}
	p.Log.Infof("Let's wait for process to complete")
	time.Sleep(2 * time.Second)

	// Stopped process can be started again, etc. If we want to get rid of the process completely, we have to delete
	// it via plugin API. It requires plugin name (not instance name). Deleted process is not stopped if running.
	// Delete also closes notification channel.
	prName := pr.GetName()
	if err := p.PM.Delete(prName); err != nil {
		return err
	}
	if err := p.PM.GetProcessByName(prName); err != nil {
		return errors.Errorf("process was expected to be removed, but is still exists")
	}

	p.Log.Infof("simple process manager example is completed")

	return nil
}

// Starts the process watcher using provided chanel.
func (p *PMExample) runWatcher(state *status.ProcessStatus, notifyChan chan status.ProcessStatus) {
	for {
		select {
		case currentState, ok := <-notifyChan:
			if !ok {
				p.Log.Infof("===>(watcher) process watcher ended")
				return
			}
			*state = currentState
			p.Log.Infof("===>(watcher) received test process state: %s", currentState)
		}
	}
}
