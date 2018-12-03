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

package logrus

import (
	"fmt"
	"os"
	"regexp"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/ligato/cn-infra/logging"
)

// DefaultRegistry is a default logging registry
//var DefaultRegistry logging.Registry

var initialLogLvl = logrus.InfoLevel

func init() {
	if lvl, err := logrus.ParseLevel(os.Getenv("INITIAL_LOGLVL")); err == nil {
		initialLogLvl = lvl
		if err := setLevel(defaultLogger, lvl); err != nil {
			defaultLogger.Warnf("setting initial log level to %v failed: %v", lvl.String(), err)
		} else {
			defaultLogger.Debugf("initial log level: %v", lvl.String())
		}
	}
	logging.DefaultRegistry = NewLogRegistry()
}

// NewLogRegistry is a constructor
func NewLogRegistry() logging.Registry {
	registry := &logRegistry{
		loggers:      new(sync.Map),
		logLevels:    make(map[string]logrus.Level),
		defaultLevel: initialLogLvl,
	}
	// put default logger
	registry.putLoggerToMapping(defaultLogger)
	return registry
}

// logRegistry contains logger map and rwlock guarding access to it
type logRegistry struct {
	// loggers holds mapping of logger instances indexed by their names
	loggers *sync.Map
	// logLevels store map of log levels for logger names
	logLevels map[string]logrus.Level
	// defaultLevel is used if logger level is not set
	defaultLevel logrus.Level
	// logging hooks
	hooks []logrus.Hook
}

var validLoggerName = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`).MatchString

func checkLoggerName(name string) error {
	if !validLoggerName(name) {
		return fmt.Errorf("logger name can contain only alphanum characters, dash and comma")
	}
	return nil
}

// NewLogger creates new named Logger instance. Name can be subsequently used to
// refer the logger in registry.
func (lr *logRegistry) NewLogger(name string) logging.Logger {
	if existingLogger := lr.getLoggerFromMapping(name); existingLogger != nil {
		panic(fmt.Errorf("logger with name '%s' already exists", name))
	}
	if err := checkLoggerName(name); err != nil {
		panic(err)
	}

	logger := NewLogger(name)

	// set initial logger level
	if lvl, ok := lr.logLevels[name]; ok {
		setLevel(logger, lvl)
	} else {
		setLevel(logger, lr.defaultLevel)
	}

	lr.putLoggerToMapping(logger)

	// add all defined hooks
	for _, hook := range lr.hooks {
		logger.std.AddHook(hook)
	}

	return logger
}

// ListLoggers returns a map (loggerName => log level)
func (lr *logRegistry) ListLoggers() map[string]string {
	list := make(map[string]string)

	var wasErr error

	lr.loggers.Range(func(k, v interface{}) bool {
		key, ok := k.(string)
		if !ok {
			wasErr = fmt.Errorf("cannot cast log map key to string")
			// false stops the iteration
			return false
		}
		value, ok := v.(*Logger)
		if !ok {
			wasErr = fmt.Errorf("cannot cast log value to Logger obj")
			return false
		}
		list[key] = value.GetLevel().String()
		return true
	})

	// throw panic outside of logger.Range()
	if wasErr != nil {
		panic(wasErr)
	}

	return list
}

func setLevel(logVal logging.Logger, lvl logrus.Level) error {
	if logVal == nil {
		return fmt.Errorf("logger %q not found", logVal)
	}
	switch lvl {
	case logrus.DebugLevel:
		logVal.SetLevel(logging.DebugLevel)
	case logrus.InfoLevel:
		logVal.SetLevel(logging.InfoLevel)
	case logrus.WarnLevel:
		logVal.SetLevel(logging.WarnLevel)
	case logrus.ErrorLevel:
		logVal.SetLevel(logging.ErrorLevel)
	case logrus.PanicLevel:
		logVal.SetLevel(logging.PanicLevel)
	case logrus.FatalLevel:
		logVal.SetLevel(logging.FatalLevel)
	}
	return nil
}

// SetLevel modifies log level of selected logger in the registry
func (lr *logRegistry) SetLevel(logger, level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	if logger == "default" {
		lr.defaultLevel = lvl
		return nil
	}
	lr.logLevels[logger] = lvl
	logVal := lr.getLoggerFromMapping(logger)
	if logVal != nil {
		defaultLogger.Debugf("setting logger level: %v -> %v", logVal.GetName(), lvl.String())
		return setLevel(logVal, lvl)
	}
	return nil
}

// GetLevel returns the currently set log level of the logger
func (lr *logRegistry) GetLevel(logger string) (string, error) {
	logVal := lr.getLoggerFromMapping(logger)
	if logVal == nil {
		return "", fmt.Errorf("logger %s not found", logger)
	}
	return logVal.GetLevel().String(), nil
}

// Lookup returns a logger instance identified by name from registry
func (lr *logRegistry) Lookup(loggerName string) (logger logging.Logger, found bool) {
	loggerInt, found := lr.loggers.Load(loggerName)
	if !found {
		return nil, false
	}
	logger, ok := loggerInt.(*Logger)
	if ok {
		return logger, found
	}
	panic(fmt.Errorf("cannot cast log value to Logger obj"))
}

// ClearRegistry removes all loggers except the default one from registry
func (lr *logRegistry) ClearRegistry() {
	var wasErr error

	// range over logger map and store keys
	lr.loggers.Range(func(k, v interface{}) bool {
		key, ok := k.(string)
		if !ok {
			wasErr = fmt.Errorf("cannot cast log map key to string")
			// false stops the iteration
			return false
		}
		if key != DefaultLoggerName {
			lr.loggers.Delete(key)
		}
		return true
	})

	if wasErr != nil {
		panic(wasErr)
	}
}

// putLoggerToMapping writes logger into map of named loggers
func (lr *logRegistry) putLoggerToMapping(logger *Logger) {
	lr.loggers.Store(logger.name, logger)
}

// getLoggerFromMapping returns a logger by its name
func (lr *logRegistry) getLoggerFromMapping(logger string) *Logger {
	loggerVal, found := lr.loggers.Load(logger)
	if !found {
		return nil
	}
	log, ok := loggerVal.(*Logger)
	if ok {
		return log
	}
	panic("cannot cast log value to Logger obj")

}

// HookConfigs stores hook configs provided by log manager
// and applies hook to existing loggers
func (lr *logRegistry) AddHook(hook logrus.Hook) {
	defaultLogger.Infof("adding hook %q to registry", hook)
	lr.hooks = append(lr.hooks, hook)

	lgs := lr.ListLoggers()
	for lg := range lgs {
		logger, found := lr.Lookup(lg)
		if found {
			logger.AddHook(hook)
		}
	}
}
