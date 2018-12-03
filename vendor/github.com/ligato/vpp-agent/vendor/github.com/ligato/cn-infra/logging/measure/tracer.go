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

package measure

//go:generate protoc --proto_path=model/apitrace --gogo_out=model/apitrace model/apitrace/apitrace.proto

import (
	"sync"
	"time"

	"github.com/ligato/cn-infra/logging/measure/model/apitrace"
)

// Tracer allows to measure, store and list measured time entries.
type Tracer interface {
	// LogTime puts measured time to the table and resets the time.
	LogTime(msgName string, start time.Time)
	// Get all trace entries stored
	Get() *apitrace.Trace
	// Clear removes entries from the log database
	Clear()
}

// NewTracer creates new tracer object
func NewTracer(msgName string) Tracer {
	return &tracer{
		msgName:   msgName,
		nextIndex: 1,
	}
}

// Inner structure handling database and measure results
type tracer struct {
	sync.Mutex

	msgName string
	// Entry index, used in database as key and increased after every entry. Never resets since the tracer object is
	// created or the database is cleared
	nextIndex uint64
	// Time database, uses index as key and entry as value
	timedb []*entry
}

// Single time entry
type entry struct {
	index      uint64
	msgName    string
	startTime  time.Time
	loggedTime time.Duration
}

func (t *tracer) LogTime(msgName string, start time.Time) {
	if t == nil {
		return
	}

	t.Lock()
	defer t.Unlock()

	// Store time
	t.timedb = append(t.timedb, &entry{
		index:      t.nextIndex,
		msgName:    msgName,
		startTime:  start,
		loggedTime: time.Since(start),
	})
	t.nextIndex++
}

func (t *tracer) Get() *apitrace.Trace {
	t.Lock()
	defer t.Unlock()

	trace := &apitrace.Trace{
		TracedEntries: make([]*apitrace.Trace_Entry, 0),
		EntryStats:    make([]*apitrace.Trace_EntryStats, 0),
	}

	average := make(map[string][]time.Duration) // message name -> measured times
	for _, entry := range t.timedb {
		// Add to total
		trace.OverallDuration += uint64(entry.loggedTime)
		// Add to trace data
		trace.TracedEntries = append(trace.TracedEntries, &apitrace.Trace_Entry{
			Index:     entry.index,
			MsgName:   entry.msgName,
			StartTime: uint64(entry.startTime.Nanosecond()),
			Duration:  uint64(entry.loggedTime.Nanoseconds()),
		})
		// Add to map for average data
		average[entry.msgName] = append(average[entry.msgName], entry.loggedTime)
	}

	// Prepare list of average times
	for msgName, times := range average {
		var total time.Duration
		for _, timeVal := range times {
			total += timeVal
		}
		averageTime := total.Nanoseconds() / int64(len(times))

		trace.EntryStats = append(trace.EntryStats, &apitrace.Trace_EntryStats{
			MsgName:         msgName,
			AverageDuration: uint64(averageTime),
		})
	}
	return trace
}

func (t *tracer) Clear() {
	t.Lock()
	defer t.Unlock()

	t.timedb = make([]*entry, 0)
}
