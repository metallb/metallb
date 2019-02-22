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

package process

import (
	"os"
	"time"

	"github.com/ligato/cn-infra/process/status"

	"github.com/ligato/cn-infra/logging"

	"github.com/pkg/errors"
)

// Process-related errors
const (
	noSuchProcess   = "no such process"
	alreadyFinished = "process already finished"
)

// ManagerAPI defines methods to manage a given process
// TODO update doc
type ManagerAPI interface {
	// Start starts the process. Depending on the procedure result, the status is set to 'running' or 'failed'. Start
	// also stores *os.Process in the instance for future use.
	Start() error
	// Restart briefly stops and starts the process. If the process is not running, it is started.
	Restart() error
	// Stop sends the termination signal to the process. The status is set to 'stopped' (or 'failed' if not successful).
	// Attempt to stop a non-existing process instance results in error
	Stop() error
	// Stop sends the termination signal to the process. The status is set to 'stopped' (or 'failed' if not successful).
	// Attempt to stop a non-existing process instance results in error
	StopAndWait() (*os.ProcessState, error)
	// Kill immediately terminates the process and releases all resources associated with it. Attempt to kill
	// a non-existing process instance results in error
	Kill() error
	// Wait for process to exit and return its process state describing its status and error (if any)
	Wait() (*os.ProcessState, error)
	// Signal allows user to send a user-defined signal
	Signal(signal os.Signal)
	// IsAlive returns true if process is alive, or false if not or if the inner instance does not exist.
	IsAlive() bool
	// GetNotification returns channel to watch process availability/status.
	GetNotificationChan() <-chan status.ProcessStatus
	// GetName returns process name
	GetName() string
	// GetInstanceName returns process name from status
	GetInstanceName() string
	// GetPid returns process ID, or zero if process instance does not exist
	GetPid() int
	// ReadStatus reads and returns all current plugin-defined process state data
	ReadStatus(pid int) (*status.File, error)
	// GetCommand returns process command
	GetCommand() string
	// GetArguments returns process arguments if set
	GetArguments() []string
	// GetStartTime returns time when the process was started
	GetStartTime() time.Time
	// GetUptime returns time elapsed since the process started
	GetUptime() time.Duration
}

// Process is wrapper around the os.Process
type Process struct {
	log logging.Logger

	// Process identification name
	name string

	// Command used to start the process. Field is empty if process was attached.
	cmd string

	// Process options provided when the process was crated/attached
	options *POptions

	// Status file is filled when the process is running and updated when necessary.
	sh     *status.Reader
	status *status.File

	// OS process instance, created on startup or obtained from running process
	process *os.Process

	// Other process-related fields not included in status
	cancelChan chan struct{}
	startTime  time.Time
}

// Start a process with defined arguments. Every process is watched for liveness and status changes
func (p *Process) Start() (err error) {
	if p.process, err = p.startProcess(); err != nil {
		return err
	}
	p.log.Debugf("New process %s was started (PID: %d)", p.GetName(), p.GetPid())

	return nil
}

// IsAlive checks whether the process is running sending zero signal. Only a simple check, does not return error
func (p *Process) IsAlive() bool {
	return p.isAlive()
}

// Restart the process, or start it if it is not running
func (p *Process) Restart() (err error) {
	if p.process == nil {
		p.log.Warn("Attempt to restart non-running process, starting it")
		p.process, err = p.startProcess()
		return err
	}
	if p.isAlive() {
		if _, err = p.StopAndWait(); err != nil {
			p.log.Warnf("Cannot stop process %s due to error, trying force stop... (err: %v)", p.GetName(), err)
			if err = p.forceStopProcess(); err != nil {
				return err
			}
		}
	}
	p.process, err = p.startProcess()
	p.log.Debugf("Process %s was restarted (PID: %d)", p.GetName(), p.GetPid())
	return err
}

// Stop sends the SIGTERM signal to stop given process
func (p *Process) Stop() error {
	if err := p.stopProcess(); err != nil {
		return err
	}
	p.log.Debugf("Process %s was stopped (last PID: %d)", p.GetName(), p.GetPid())
	return nil
}

// StopAndWait sends the SIGTERM signal to stop given process and waits until it is completed
func (p *Process) StopAndWait() (*os.ProcessState, error) {
	if err := p.stopProcess(); err != nil {
		return nil, err
	}
	state, err := p.Wait()
	if err != nil {
		return nil, errors.Errorf("process exit with error: %v", err)
	}
	p.log.Debugf("Process %s was stopped (last PID: %d)", p.GetName(), p.GetPid())
	return state, nil
}

// Kill sends the SIGKILL signal to force stop given process
func (p *Process) Kill() error {
	if err := p.forceStopProcess(); err != nil {
		return err
	}
	p.log.Debugf("Process %s was forced to stop (last PID: %d)", p.GetName(), p.GetPid())
	return nil
}

// Wait for the process to exit, and then returns its state.
func (p *Process) Wait() (*os.ProcessState, error) {
	if p.process == nil {
		return nil, errors.Errorf("process %s was not started yet", p.GetName())
	}
	return p.process.Wait()
}

// Signal sends custom signal to the process
func (p *Process) Signal(signal os.Signal) {
	p.process.Signal(signal)
}

// GetName returns plugin-wide process name
func (p *Process) GetName() string {
	return p.name
}

// GetInstanceName returns process name of the instance
func (p *Process) GetInstanceName() string {
	return p.status.Name
}

// GetPid returns process ID
func (p *Process) GetPid() int {
	if p.process != nil {
		return p.process.Pid
	}
	return p.status.Pid
}

// GetCommand returns command used to start process. May be empty for attached processes
func (p *Process) GetCommand() string {
	return p.cmd
}

// GetArguments returns arguments process was started with, if any. May be empty also for attached processes
func (p *Process) GetArguments() []string {
	if p.options.args == nil {
		return []string{}
	}
	return p.options.args
}

// ReadStatus updates actual process status and returns status file
func (p *Process) ReadStatus(pid int) (statusFile *status.File, err error) {
	p.status, err = p.sh.ReadStatusFromPID(pid)
	if err != nil {
		return &status.File{}, errors.Errorf("failed to read status file for process ID %d: %v", pid, err)
	}
	return p.status, nil
}

// GetNotificationChan returns channel listening on notifications about process status changes
func (p *Process) GetNotificationChan() <-chan status.ProcessStatus {
	if p.options != nil && p.options.notifyChan != nil {
		return p.options.notifyChan
	}
	return nil
}

// GetStartTime returns process start timestamp
func (p *Process) GetStartTime() time.Time {
	return p.startTime
}

// GetUptime returns process uptime since the last start
func (p *Process) GetUptime() time.Duration {
	if p.startTime.Nanosecond() == 0 {
		return 0
	}
	return time.Since(p.startTime)
}
