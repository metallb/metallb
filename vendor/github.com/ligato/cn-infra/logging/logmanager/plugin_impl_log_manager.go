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

package logmanager

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/unrolled/render"
)

// LoggerData encapsulates parameters of a logger represented as strings.
type LoggerData struct {
	Logger string `json:"logger"`
	Level  string `json:"level"`
}

// Variable names in logger registry URLs
const (
	loggerVarName = "logger"
	levelVarName  = "level"
)

// Plugin allows to manage log levels of the loggers.
type Plugin struct {
	Deps

	*Config
}

// Deps groups dependencies injected into the plugin so that they are
// logically separated from other plugin fields.
type Deps struct {
	infra.PluginDeps
	ServiceLabel servicelabel.ReaderAPI
	LogRegistry  logging.Registry
	HTTP         rest.HTTPHandlers
}

// Init does nothing
func (p *Plugin) Init() error {
	if p.Cfg != nil {
		if p.Config == nil {
			p.Config = NewConf()
		}

		_, err := p.Cfg.LoadValue(p.Config)
		if err != nil {
			return err
		}
		p.Log.Debugf("logs config: %+v", p.Config)

		// Handle default log level. Prefer value from environmental variable
		defaultLogLvl := os.Getenv("INITIAL_LOGLVL")
		if defaultLogLvl == "" {
			defaultLogLvl = p.Config.DefaultLevel
		}
		if defaultLogLvl != "" {
			if err := p.LogRegistry.SetLevel("default", defaultLogLvl); err != nil {
				p.Log.Warnf("setting default log level failed: %v", err)
			} else {
				// All loggers created up to this point were created with initial log level set (defined
				// via INITIAL_LOGLVL env. variable with value 'info' by default), so at first, let's set default
				// log level for all of them.
				for loggerName := range p.LogRegistry.ListLoggers() {
					logger, exists := p.LogRegistry.Lookup(loggerName)
					if !exists {
						continue
					}
					logger.SetLevel(logging.ParseLogLevel(defaultLogLvl))
				}
			}
		}

		// Handle config file log levels
		for _, logCfgEntry := range p.Config.Loggers {
			// Put log/level entries from configuration file to the registry.
			if err := p.LogRegistry.SetLevel(logCfgEntry.Name, logCfgEntry.Level); err != nil {
				// Intentionally just log warn & not propagate the error (it is minor thing to interrupt startup)
				p.Log.Warnf("setting log level %s for logger %s failed: %v",
					logCfgEntry.Level, logCfgEntry.Name, err)
			}
		}
		if len(p.Config.Hooks) > 0 {
			p.Log.Info("configuring log hooks")
			for hookName, hookConfig := range p.Config.Hooks {
				if err := p.addHook(hookName, hookConfig); err != nil {
					p.Log.Warnf("configuring log hook %s failed: %v", hookName, err)
				}
			}
		}
	}

	return nil
}

// AfterInit is called at plugin initialization. It register the following handlers:
// - List all registered loggers:
//   > curl -X GET http://localhost:<port>/log/list
// - Set log level for a registered logger:
//   > curl -X PUT http://localhost:<port>/log/<logger-name>/<log-level>
func (p *Plugin) AfterInit() error {
	if p.HTTP != nil {
		p.HTTP.RegisterHTTPHandler(fmt.Sprintf("/log/{%s}/{%s:debug|info|warn|error|fatal|panic}",
			loggerVarName, levelVarName), p.logLevelHandler, "PUT")
		p.HTTP.RegisterHTTPHandler("/log/list", p.listLoggersHandler, "GET")
	}
	return nil
}

// Close is called at plugin cleanup phase.
func (p *Plugin) Close() error {
	return nil
}

// ListLoggers lists all registered loggers.
func (p *Plugin) listLoggers() (loggers []LoggerData) {
	for logger, lvl := range p.LogRegistry.ListLoggers() {
		loggers = append(loggers, LoggerData{
			Logger: logger,
			Level:  lvl,
		})
	}
	return loggers
}

// setLoggerLogLevel modifies the log level of the all loggers in a plugin
func (p *Plugin) setLoggerLogLevel(name string, level string) error {
	p.Log.Debugf("SetLogLevel name %q, level %q", name, level)

	return p.LogRegistry.SetLevel(name, level)
}

// logLevelHandler processes requests to set log level on loggers in a plugin
func (p *Plugin) logLevelHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		p.Log.Infof("Path: %s", req.URL.Path)

		vars := mux.Vars(req)
		if vars == nil {
			formatter.JSON(w, http.StatusNotFound, struct{}{})
			return
		}
		err := p.setLoggerLogLevel(vars[loggerVarName], vars[levelVarName])
		if err != nil {
			formatter.JSON(w, http.StatusNotFound,
				struct{ Error string }{err.Error()})
			return
		}

		formatter.JSON(w, http.StatusOK, LoggerData{
			Logger: vars[loggerVarName],
			Level:  vars[levelVarName],
		})
	}
}

// listLoggersHandler processes requests to list all registered loggers
func (p *Plugin) listLoggersHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		formatter.JSON(w, http.StatusOK, p.listLoggers())
	}
}
