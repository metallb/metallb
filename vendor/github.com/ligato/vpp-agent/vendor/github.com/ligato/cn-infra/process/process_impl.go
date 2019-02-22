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
	"strings"
	"syscall"
	"time"

	"github.com/ligato/cn-infra/process/status"
	"github.com/pkg/errors"
)

// Marked defines that the process should be always restarted
const infiniteRestarts = -1

func (p *Process) startProcess() (*os.Process, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.Errorf("failed to get rooted path name for: %v", err)
	}
	var attr = os.ProcAttr{
		Dir:   wd,
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, nil, nil},
	}
	// Syscall if process should be detached from parent
	if p.options != nil && p.options.detach {
		attr.Sys = &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0,
		}
	}
	// The actual command should be also as a first argument
	pArgs := append([]string{p.cmd}, p.options.args...)
	process, err := os.StartProcess(p.cmd, pArgs, &attr)
	if err != nil {
		return nil, errors.Errorf("failed to start new process (cmd: %s): %v", p.cmd, err)
	}
	p.startTime = time.Now()

	p.sh.ReadStatusFromPID(process.Pid)

	return process, nil
}

func (p *Process) stopProcess() (err error) {
	if p.process == nil {
		return errors.Errorf("asked to stop non-existing process instance")
	}

	if err = p.process.Signal(syscall.SIGTERM); err != nil && !strings.Contains(err.Error(), alreadyFinished) {
		return errors.Errorf("process termination unsuccessful: %v", err)
	}

	p.startTime = time.Time{}
	return nil
}

func (p *Process) forceStopProcess() (err error) {
	if p.process != nil {
		return errors.Errorf("asked to force-stop non-existing process instance")
	}

	if err = p.process.Signal(syscall.SIGKILL); err != nil && !strings.Contains(err.Error(), alreadyFinished) {
		return errors.Errorf("process forced termination unsuccessful: %v", err)
	}
	if err = p.process.Release(); err != nil {
		return errors.Errorf("resource release failed: %v", err)
	}

	p.startTime = time.Time{}
	return nil
}

func (p *Process) isAlive() bool {
	if p.process == nil {
		return false
	}
	osProcess, err := os.FindProcess(p.process.Pid)
	if err != nil {
		return false
	}
	err = osProcess.Signal(syscall.Signal(0))
	if err != nil && (strings.Contains(err.Error(), noSuchProcess) || strings.Contains(err.Error(), alreadyFinished)) {
		return false
	}
	// Error can be not nil and process may still exits (for example if process is alive but not owned by caller)
	return true
}

// Delete stops the process and internal watcher
func (p *Process) delete() error {
	if p.process == nil {
		return nil
	}

	// Close the process watcher
	if p.cancelChan != nil {
		close(p.cancelChan)
	}

	p.log.Debugf("Process %s deleted", p.name)
	return nil
}

// Periodically tries to 'ping' process. If the process is unresponsive, marks it as terminated. Otherwise the process
// status is updated. If process status was changed, notification is sent. In addition, terminated processes are
// restarted if allowed by policy, and dead processes are cleaned up.
func (p *Process) watch() {
	p.log.Debugf("Process %s watcher started", p.name)
	// TODO make it configurable
	ticker := time.NewTicker(1 * time.Second)

	var last status.ProcessStatus
	var numRestarts int32
	var autoTerm bool
	if p.options != nil {
		numRestarts = p.options.restart
		autoTerm = p.options.autoTerm
	}

	for {
		select {
		case <-ticker.C:
			var current status.ProcessStatus
			if !p.isAlive() {
				current = status.Terminated
			} else {
				pStatus, err := p.ReadStatus(p.GetPid())
				if err != nil {
					p.log.Warn(err)
				}
				if pStatus.State == "" {
					current = status.Unavailable
				} else {
					current = pStatus.State
				}
			}

			if current != last {
				if p.GetNotificationChan() != nil {
					p.options.notifyChan <- current
				}
				if current == status.Terminated {
					if numRestarts > 0 || numRestarts == infiniteRestarts {
						go func() {
							var err error
							if p.process, err = p.startProcess(); err != nil {
								p.log.Error("attempt to restart process %s failed: %v", p.name, err)
							}
						}()
						numRestarts--
					} else {
						p.log.Debugf("no more attempts to restart process %s", p.name)
					}
				}
				if current == status.Zombie && autoTerm {
					p.log.Debugf("Terminating zombie process %d", p.GetPid())
					if _, err := p.Wait(); err != nil {
						p.log.Warnf("failed to terminate dead process: %s", p.GetPid(), err)
					}
				}
			}
			last = current
		case <-p.cancelChan:
			ticker.Stop()
			if p.GetNotificationChan() != nil {
				close(p.options.notifyChan)
			}

			p.log.Debugf("Process %s watcher stopped", p.name)

			return
		}
	}
}
