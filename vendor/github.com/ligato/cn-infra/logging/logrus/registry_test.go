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
	"testing"

	"github.com/onsi/gomega"
)

func TestListLoggers(t *testing.T) {
	logRegistry := NewLogRegistry()

	gomega.RegisterTestingT(t)
	loggers := logRegistry.ListLoggers()
	gomega.Expect(loggers).NotTo(gomega.BeNil())

	lg, found := loggers[DefaultLoggerName]
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(lg).NotTo(gomega.BeNil())
}

func TestNewLogger(t *testing.T) {
	logRegistry := NewLogRegistry()

	const loggerName = "myLogger"
	gomega.RegisterTestingT(t)
	lg := logRegistry.NewLogger(loggerName)
	gomega.Expect(lg).NotTo(gomega.BeNil())

	loggers := logRegistry.ListLoggers()
	gomega.Expect(loggers).NotTo(gomega.BeNil())

	fromRegistry, found := loggers[loggerName]
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(fromRegistry).NotTo(gomega.BeNil())
}

func TestGetSetLevel(t *testing.T) {
	logRegistry := NewLogRegistry()

	gomega.RegisterTestingT(t)
	const level = "error"
	//existing logger
	err := logRegistry.SetLevel(DefaultLoggerName, level)
	gomega.Expect(err).To(gomega.BeNil())

	loggers := logRegistry.ListLoggers()
	gomega.Expect(loggers).NotTo(gomega.BeNil())

	logger, found := loggers[DefaultLoggerName]
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(logger).NotTo(gomega.BeNil())
	gomega.Expect(loggers[DefaultLoggerName]).To(gomega.BeEquivalentTo(level))

	currentLevel, err := logRegistry.GetLevel(DefaultLoggerName)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(level).To(gomega.BeEquivalentTo(currentLevel))

	//non-existing logger
	err = logRegistry.SetLevel("unknown", level)
	gomega.Expect(err).To(gomega.BeNil()) // will be kept in logger level map in registry

	_, err = logRegistry.GetLevel("unknown")
	gomega.Expect(err).NotTo(gomega.BeNil())
}

func TestGetLoggerByName(t *testing.T) {
	logRegistry := NewLogRegistry()

	const (
		loggerA = "myLoggerA"
		loggerB = "myLoggerB"
	)
	lgA := logRegistry.NewLogger(loggerA)
	gomega.Expect(lgA).NotTo(gomega.BeNil())

	lgB := logRegistry.NewLogger(loggerB)
	gomega.Expect(lgB).NotTo(gomega.BeNil())

	returnedA, found := logRegistry.Lookup(loggerA)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(returnedA).To(gomega.BeEquivalentTo(lgA))

	returnedB, found := logRegistry.Lookup(loggerB)
	gomega.Expect(found).To(gomega.BeTrue())
	gomega.Expect(returnedB).To(gomega.BeEquivalentTo(lgB))

	unknown, found := logRegistry.Lookup("unknown")
	gomega.Expect(found).To(gomega.BeFalse())
	gomega.Expect(unknown).To(gomega.BeNil())
}

func TestClearRegistry(t *testing.T) {
	logRegistry := NewLogRegistry()

	const (
		loggerA = "loggerA"
		loggerB = "loggerB"
	)
	lgA := NewLogger(loggerA)
	gomega.Expect(lgA).NotTo(gomega.BeNil())

	lgB := NewLogger(loggerB)
	gomega.Expect(lgB).NotTo(gomega.BeNil())

	logRegistry.ClearRegistry()

	_, found := logRegistry.Lookup(loggerA)
	gomega.Expect(found).To(gomega.BeFalse())

	_, found = logRegistry.Lookup(loggerB)
	gomega.Expect(found).To(gomega.BeFalse())

	_, found = logRegistry.Lookup(DefaultLoggerName)
	gomega.Expect(found).To(gomega.BeTrue())
}
