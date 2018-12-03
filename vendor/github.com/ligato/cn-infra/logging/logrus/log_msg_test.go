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
	"bytes"
	"fmt"
	"testing"

	lg "github.com/sirupsen/logrus"

	"github.com/onsi/gomega"
	"log/syslog"
	syslog2 "github.com/sirupsen/logrus/hooks/syslog"
)

func TestEntryPanicln(t *testing.T) {
	gomega.RegisterTestingT(t)

	errBoom := fmt.Errorf("boom time")

	defer func() {
		p := recover()
		gomega.Expect(p).NotTo(gomega.BeNil())

		switch pVal := p.(type) {
		case *lg.Entry:
			gomega.Expect("kaboom").To(gomega.BeEquivalentTo(pVal.Message))
			gomega.Expect(errBoom).To(gomega.BeEquivalentTo(pVal.Data["err"]))
		default:
			t.Fatalf("want type *LogMsg, got %T: %#v", pVal, pVal)
		}
	}()

	logger := NewLogger("testLogger")
	logger.std.Out = &bytes.Buffer{}
	entry := NewEntry(logger)
	entry.WithField("err", errBoom).Panicln("kaboom")
}

func TestEntryPanicf(t *testing.T) {
	errBoom := fmt.Errorf("boom again")

	defer func() {
		p := recover()
		gomega.Expect(p).NotTo(gomega.BeNil())

		switch pVal := p.(type) {
		case *lg.Entry:
			gomega.Expect("kaboom true").To(gomega.BeEquivalentTo(pVal.Message))
			gomega.Expect(errBoom).To(gomega.BeEquivalentTo(pVal.Data["err"]))
		default:
			t.Fatalf("want type *LogMsg, got %T: %#v", pVal, pVal)
		}
	}()

	logger := NewLogger("testLogger")
	logger.std.Out = &bytes.Buffer{}
	entry := NewEntry(logger)
	entry.WithField("err", errBoom).Panicf("kaboom %v", true)
}

func TestAddHook(t *testing.T) {
	gomega.RegisterTestingT(t)

	logRegistry := NewLogRegistry()
	lgA := logRegistry.NewLogger("logger")
	gomega.Expect(lgA).NotTo(gomega.BeNil())

	hook, _ := syslog2.NewSyslogHook(
		"",
		"",
		syslog.LOG_INFO, "")

	lgA.AddHook(hook)
	lgA.Info("Test Hook")

}