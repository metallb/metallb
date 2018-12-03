//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package agent

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/measure"
	"github.com/ligato/cn-infra/utils/once"
	"github.com/namsral/flag"
)

var agentLogger = logging.DefaultRegistry.NewLogger("agent")

// Variables set by the compiler using ldflags
var (
	// BuildVersion describes version for the build. It is usually set using `git describe --always --tags --dirty`.
	BuildVersion = "v0.0.0-dev"
	// BuildDate describes time of the build.
	BuildDate string
	// CommitHash describes commit hash for the build.
	CommitHash string
)

// Agent implements startup & shutdown procedures for plugins.
type Agent interface {
	// Run is a blocking call which starts the agent with all of its plugins,
	// waits for a signal from OS (SIGINT, SIGTERM by default), context cancellation or
	// close of quit channel (can be set via options) and then stops the agent.
	// Returns nil if all the plugins were intialized and closed successfully.
	Run() error
	// Start starts the agent with all the plugins, calling their Init() and optionally AfterInit().
	// Returns nil if all the plugins were initialized successfully.
	Start() error
	// Stop stops the agent with all the plugins, calling their Close().
	// Returns nil if all the plugins were closed successfully.
	Stop() error
	// Options returns all agent's options configured via constructor.
	Options() Options

	// Wait waits until agent is stopped  and returns same error as Stop().
	Wait() error
	// After returns a channel that is closed before the agents is stopped.
	// Note: It is not certain the all plugins are stopped, see Error()..
	After() <-chan struct{}
	// Error returns an error that occurret when the agent was stopped.
	// Note: This essentially just calls Stop()..
	Error() error
}

// NewAgent creates a new agent using given options and registers all flags
// defined for plugins via config.ForPlugin.
func NewAgent(opts ...Option) Agent {
	options := newOptions(opts...)

	if !flag.Parsed() {
		config.DefineDirFlag()
		for _, p := range options.Plugins {
			name := p.String()
			infraLogger.Debugf("registering flags for: %q", name)
			config.DefineFlagsFor(name)
		}
		flag.Parse()
	}

	return &agent{
		opts:   options,
		tracer: measure.NewTracer("agent-plugins"),
	}
}

type agent struct {
	opts Options

	stopCh chan struct{}

	startOnce once.ReturnError
	stopOnce  once.ReturnError

	tracer measure.Tracer
}

// Options returns the Options the agent was created with
func (a *agent) Options() Options {
	return a.opts
}

// Start starts the agent.  Start will return as soon as the Agent is ready.  The Agent continues
// running after Start returns.
func (a *agent) Start() error {
	return a.startOnce.Do(a.starter)
}

// Stop the Agent.  Calls close on all Plugins
func (a *agent) Stop() error {
	return a.stopOnce.Do(a.stopper)
}

// Run runs the agent.  Run will not return until a SIGINT, SIGTERM, or SIGKILL is received
func (a *agent) Run() error {
	if err := a.Start(); err != nil {
		return err
	}
	return a.Wait()
}

func (a *agent) starter() error {
	agentLogger.WithFields(logging.Fields{
		"CommitHash": CommitHash,
		"BuildDate":  BuildDate,
	}).Infof("Starting agent version: %v", BuildVersion)

	// If we want to properly handle cleanup when a SIG comes in *during*
	// agent startup (ie, clean up after its finished) we need to register
	// for the signal before we start() the agent
	sig := make(chan os.Signal, 1)
	if len(a.opts.QuitSignals) > 0 {
		signal.Notify(sig, a.opts.QuitSignals...)
	}

	started := make(chan struct{})

	if timeout := a.opts.StartTimeout; timeout > 0 {
		go func() {
			select {
			case <-started:
				// agent started
			case <-time.After(timeout):
				agentLogger.Errorf("Agent failed to start before timeout (%v)", timeout)
				os.Exit(1)
			}
		}()
	}

	// If the agent started, we have things to clean up if here is a SIG
	// So fire off a goroutine to do that

	t := time.Now()

	if err := a.start(); err != nil {
		signal.Stop(sig)
		return err
	}
	close(started)

	agentLogger.Infof("Agent started with %d plugins (took %v)",
		len(a.opts.Plugins), time.Since(t).Round(time.Millisecond))

	a.stopCh = make(chan struct{}) // If we are started, we have a stopCh to signal stopping

	go func() {
		var quit <-chan struct{}
		if a.opts.Context != nil {
			quit = a.opts.Context.Done()
		}
		// Wait for signal or agent stop
		select {
		case <-a.opts.QuitChan:
			agentLogger.Info("Quit channel closed, stopping.")
		case <-quit:
			agentLogger.Info("Context canceled, stopping.")
		case s := <-sig:
			agentLogger.Infof("Signal %v received, stopping.", s)
		case <-a.stopCh:
			// agent stopped
		}
		// Doesn't hurt to call Stop twice, its idempotent because of the
		// stopOnce
		a.Stop()
		signal.Stop(sig)
	}()

	return nil
}

func (a *agent) start() error {
	agentLogger.Infof("Starting agent with %d plugins", len(a.opts.Plugins))

	// Init plugins
	for _, plugin := range a.opts.Plugins {
		t := time.Now()

		agentLogger.Debugf("-> Init(): %v", plugin)
		if err := plugin.Init(); err != nil {
			return err
		}

		a.tracer.LogTime(fmt.Sprintf("%v.Init", plugin), t)
	}

	// AfterInit plugins
	for _, plugin := range a.opts.Plugins {
		t := time.Now()

		if postPlugin, ok := plugin.(infra.PostInit); ok {
			agentLogger.Debugf("-> AfterInit(): %v", plugin)
			if err := postPlugin.AfterInit(); err != nil {
				return err
			}
		} else {
			agentLogger.Debugf("-- AfterInit(): %v (not used)", plugin)
		}

		a.tracer.LogTime(fmt.Sprintf("%v.AfterInit", plugin), t)
	}

	if printPluginStartDurations && infraLogger.GetLevel() >= logging.DebugLevel {
		var b strings.Builder
		b.WriteString("plugin start durations:\n")
		for _, entry := range a.tracer.Get().GetTracedEntries() {
			dur := "<1ms"
			if d := time.Duration(entry.Duration); d > time.Millisecond {
				dur = d.Round(time.Millisecond).String()
			}
			b.WriteString(fmt.Sprintf(" - %v: %v\n", entry.MsgName, dur))
		}
		fmt.Fprintf(os.Stdout, b.String())
	}

	return nil
}

func (a *agent) stopper() error {
	agentLogger.Infof("Stopping agent")

	stopped := make(chan struct{})
	defer close(stopped)

	if timeout := a.opts.StopTimeout; timeout > 0 {
		go func() {
			select {
			case <-stopped:
				// agent stopped
			case <-time.After(timeout):
				agentLogger.Errorf("agent failed to stop before timeout (%v)", timeout)
				os.Exit(1)
			}
		}()
	}

	if err := a.stop(); err != nil {
		return err
	}

	agentLogger.Info("Agent stopped")

	return nil
}

func (a *agent) stop() error {
	if a.stopCh == nil {
		err := errors.New("attempted to stop an agent that was not Started")
		agentLogger.Error(err)
		return err
	}
	agentLogger.Debugf("stopping %d plugins", len(a.opts.Plugins))

	defer close(a.stopCh)

	// Close plugins in reverse order
	for i := len(a.opts.Plugins) - 1; i >= 0; i-- {
		p := a.opts.Plugins[i]
		agentLogger.Debugf("-> Close(): %v", p)
		if err := p.Close(); err != nil {
			return err
		}
	}

	return nil
}

// Wait will not return until a SIGINT, SIGTERM, or SIGKILL is received
// Or the Agent is Stopped
// All Plugins are Closed() before Wait returns
func (a *agent) Wait() error {
	if a.stopCh == nil {
		err := errors.New("attempted to wait on an agent that wasn't Started")
		agentLogger.Error(err)
		return err
	}
	<-a.stopCh

	// If we get here, a.Stop() has already been called, and we are simply
	// retrieving the error if any squirreled away by stopOnce
	return a.Stop()
}

// After returns a channel that will be closed when the agent is Stopped.
// To retrieve any error from the agent stopping call Error() on the agent
// The normal pattern of use is:
//
// agent := NewAgent(options...)
// agent.Start()
// select {
// case <-agent.After() // Will wait till the agent is stopped
// ...
// }
// err := agent.Error() // Will return any error from the agent being stopped
//
func (a *agent) After() <-chan struct{} {
	if a.stopCh != nil {
		return a.stopCh
	}
	// The agent didn't start, so we can't return a.stopCh
	// because *only* a.start() should allocate that
	// we won't return a nil channel, because nil channels
	// block forever.
	// Since the normal pattern is to call a.After() so you
	// can select till the agent is done and a.Stop() to
	// retrieve the error, returning a closed channel will preserve that
	// usage, as a.Stop() returns an error complaining that the agent
	// never started.
	ch := make(chan struct{})
	close(ch)
	return ch
}

// Error returns any error that occurred when the agent was Stopped
func (a *agent) Error() error {
	// a.Stop() returns whatever error occurred when stopping the agent
	// This is because of stopOnce
	// If you try to retrieve an error before the agent is started, you will get
	// an error complaining the agent isn't started.
	return a.Stop()
}
