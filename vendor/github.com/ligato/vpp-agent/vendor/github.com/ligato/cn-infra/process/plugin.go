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

	"github.com/ligato/cn-infra/process/status"
	"github.com/ligato/cn-infra/process/template"
	"github.com/ligato/cn-infra/process/template/model/process"

	"github.com/ligato/cn-infra/infra"
	"github.com/pkg/errors"
)

// API defines methods to create, delete or manage processes
type API interface {
	// NewProcess creates new process instance with name, command to start and other options (arguments, policy).
	// New process is not immediately started, process instance comprises from a set of methods to manage.
	NewProcess(name, cmd string, options ...POption) ManagerAPI
	// Starts process from template file
	NewProcessFromTemplate(tmp *process.Template) ManagerAPI
	// Attach to existing process using its process ID. The process is stored under the provided name. Error
	// is returned if process does not exits
	AttachProcess(name, cmd string, pid int, options ...POption) (ManagerAPI, error)
	// GetProcessByName returns existing process instance using name
	GetProcessByName(name string) ManagerAPI
	// GetProcessByName returns existing process instance using PID
	GetProcessByPID(pid int) ManagerAPI
	// GetAll returns all processes known to plugin
	GetAllProcesses() []ManagerAPI
	// Delete removes process from the memory. Delete cancels process watcher, but does not stop the running instance
	// (possible to attach later). Note: no process-related templates are removed
	Delete(name string) error
	// GetTemplate returns process template object with given name fom provided path. Returns nil if does not exists
	// or error if the reader is not available
	GetTemplate(name string) (*process.Template, error)
	// GetAllTemplates returns all templates available from given path. Returns empty list if
	// the reader is not available
	GetAllTemplates() ([]*process.Template, error)
}

// Plugin implements API to manage processes. There are two options to add a process to manage, start it as a new one
// or attach to an existing process. In both cases, the process is stored internally as known to the plugin.
type Plugin struct {
	// Reader handles process templates (optional, can be nil)
	tReader *template.Reader
	// All known process instances
	processes []*Process

	Deps
}

// Deps define process dependencies
type Deps struct {
	infra.PluginDeps
}

// Config contains information about the path where process templates are stored
type Config struct {
	TemplatePath string `json:"template-path"`
}

// Init reads plugin config file for process template path. If exists, plugin initializes template reader, reads
// all existing templates and initializes them. Those marked as 'run on startup' are immediately started
func (p *Plugin) Init() error {
	p.Log.Debugf("Initializing process manager plugin")

	templatePath, err := p.getPMConfig()
	if err != nil {
		return err
	}

	if templatePath != "" {
		if p.tReader, err = template.NewTemplateReader(templatePath, p.Log); err != nil {
			return nil
		}
		templates, err := p.tReader.GetAllTemplates()
		if err != nil {
			return err
		}
		for _, tmp := range templates {
			pr := p.NewProcessFromTemplate(tmp)
			if pr == nil {
				continue
			}
			if tmp.POptions != nil && tmp.POptions.RunOnStartup {
				if err = pr.Start(); err != nil {
					p.Log.Errorf("failed to start template process %s: %v", tmp.Name, err)
					continue
				}
			}
		}
	}

	return nil
}

// Close stops all process watcher. Processes are either kept running (if detached) or terminated automatically
// if thay are child processes of the application
func (p *Plugin) Close() error {
	for _, pr := range p.processes {
		close(pr.cancelChan)
	}
	return nil
}

// String returns string representation of the plugin
func (p *Plugin) String() string {
	return p.PluginName.String()
}

// AttachProcess attaches to existing process and reads its status
func (p *Plugin) AttachProcess(name string, cmd string, pid int, options ...POption) (ManagerAPI, error) {
	pr, err := os.FindProcess(pid)
	if err != nil {
		return nil, errors.Errorf("cannot attach to process with PID %d: %v", pid, err)
	}
	attachedPr := &Process{
		log:        p.Log,
		name:       name,
		cmd:        cmd,
		options:    &POptions{},
		process:    pr,
		sh:         &status.Reader{Log: p.Log},
		cancelChan: make(chan struct{}),
	}
	for _, option := range options {
		option(attachedPr.options)
	}
	p.processes = append(p.processes, attachedPr)

	attachedPr.status, err = attachedPr.sh.ReadStatusFromPID(attachedPr.GetPid())
	if err != nil {
		p.Log.Warnf("failed to read process (PID %d) status: %v", pid, err)
	}

	go attachedPr.watch()

	if attachedPr.options.template {
		p.writeAsTemplate(attachedPr)
	}

	return attachedPr, nil
}

// NewProcess creates a new process and saves its template if required
func (p *Plugin) NewProcess(name, cmd string, options ...POption) ManagerAPI {
	newPr := &Process{
		log:        p.Log,
		name:       name,
		cmd:        cmd,
		options:    &POptions{},
		sh:         &status.Reader{Log: p.Log},
		status:     &status.File{},
		cancelChan: make(chan struct{}),
	}
	for _, option := range options {
		option(newPr.options)
	}
	p.processes = append(p.processes, newPr)

	go newPr.watch()

	if newPr.options.template {
		p.writeAsTemplate(newPr)
	}

	return newPr
}

// NewProcessFromTemplate creates a new process from template file
func (p *Plugin) NewProcessFromTemplate(tmp *process.Template) ManagerAPI {
	newTmpPr, err := p.templateToProcess(tmp)
	if err != nil {
		p.Log.Errorf("cannot create a process from template: %v", err)
		return nil
	}
	p.processes = append(p.processes, newTmpPr)

	go newTmpPr.watch()

	return newTmpPr
}

// GetProcessByName uses process name to find a desired instance
func (p *Plugin) GetProcessByName(name string) ManagerAPI {
	for _, pr := range p.processes {
		if pr.name == name {
			return pr
		}
	}
	return nil
}

// GetProcessByPID uses process ID to find a desired instance
func (p *Plugin) GetProcessByPID(pid int) ManagerAPI {
	for _, pr := range p.processes {
		if pr.status.Pid == pid {
			return pr
		}
	}
	return nil
}

// GetAllProcesses returns all processes known to plugin
func (p *Plugin) GetAllProcesses() []ManagerAPI {
	var processes []ManagerAPI
	for _, pr := range p.processes {
		processes = append(processes, pr)
	}
	return processes
}

// Delete releases the process resources and removes it from the plugin cache
func (p *Plugin) Delete(name string) error {
	var updated []*Process
	for _, pr := range p.processes {
		if pr.name == name {
			if err := pr.delete(); err != nil {
				return err
			}
		} else {
			updated = append(updated, pr)
		}
	}

	p.processes = updated

	return nil
}

// GetTemplate returns template with given name
func (p *Plugin) GetTemplate(name string) (*process.Template, error) {
	if p.tReader == nil {
		return nil, errors.Errorf("cannot read process template %s: reader is nil (no path was defined)", name)
	}
	templates, err := p.tReader.GetAllTemplates()
	if err != nil {
		return nil, err
	}
	for _, tmp := range templates {
		if tmp.Name == name {
			return tmp, nil
		}
	}
	return nil, nil
}

// GetAllTemplates returns all templates
func (p *Plugin) GetAllTemplates() ([]*process.Template, error) {
	if p.tReader == nil {
		return nil, errors.Errorf("cannot read process templates: reader is nil (no path was defined)")
	}
	return p.tReader.GetAllTemplates()
}

// Reads plugin config file
func (p *Plugin) getPMConfig() (path string, err error) {
	var pmConfig Config
	found, err := p.Cfg.LoadValue(&pmConfig)
	if err != nil {
		return path, errors.Errorf("failed to read process manager config file: %v", err)
	}
	if found {
		return pmConfig.TemplatePath, nil
	}
	return path, nil
}

// Writes process as template to the filesystem. Errors are logged but not returned
func (p *Plugin) writeAsTemplate(pr *Process) {
	tmp, err := p.processToTemplate(pr)
	if err != nil {
		p.Log.Errorf("cannot create a template from process: %v", err)
		return
	}
	if p.tReader == nil {
		p.Log.Warnf("process %s should write a new template, but reader (template path) is not defined",
			pr.name)
		return
	}
	if err = p.tReader.WriteTemplate(tmp, template.DefaultMode); err != nil {
		p.Log.Warnf("failed to write template %s: %v", tmp.Name, err)
		return
	}
}

// Create a template object from process. A name and a command are mandatory
func (p *Plugin) processToTemplate(pr *Process) (*process.Template, error) {
	if pr.name == "" {
		return nil, errors.Errorf("cannot create template from process, missing name")
	}
	if pr.cmd == "" {
		return nil, errors.Errorf("cannot create template from process, missing command")
	}

	pOptions := &process.TemplatePOptions{
		Args:         pr.options.args,
		Restart:      pr.options.restart,
		Detach:       pr.options.detach,
		RunOnStartup: pr.options.runOnStartup,
		Notify: func(notifyChan chan status.ProcessStatus) bool {
			if notifyChan == nil {
				return false
			}
			return true
		}(pr.options.notifyChan),
		AutoTerminate: pr.options.autoTerm,
	}

	return &process.Template{
		Name:     pr.name,
		Cmd:      pr.cmd,
		POptions: pOptions,
	}, nil
}

// Create a process object from template. A name and a command are mandatory
func (p *Plugin) templateToProcess(tmp *process.Template) (*Process, error) {
	if tmp.Name == "" {
		return nil, errors.Errorf("cannot create process from template, missing name")
	}
	if tmp.Cmd == "" {
		return nil, errors.Errorf("cannot create process from template, missing command")
	}

	pOptions := &POptions{}
	if tmp.POptions != nil {
		pOptions.args = tmp.POptions.Args
		pOptions.detach = tmp.POptions.Detach
		pOptions.restart = tmp.POptions.Restart
		pOptions.runOnStartup = tmp.POptions.RunOnStartup
		pOptions.notifyChan = func(notify bool) chan status.ProcessStatus {
			if notify {
				return make(chan status.ProcessStatus)
			}
			return nil
		}(tmp.POptions.Notify)
		pOptions.autoTerm = tmp.POptions.AutoTerminate
	}

	return &Process{
		log:        p.Log,
		name:       tmp.Name,
		cmd:        tmp.Cmd,
		options:    pOptions,
		sh:         &status.Reader{Log: p.Log},
		status:     &status.File{},
		cancelChan: make(chan struct{}),
	}, nil
}
