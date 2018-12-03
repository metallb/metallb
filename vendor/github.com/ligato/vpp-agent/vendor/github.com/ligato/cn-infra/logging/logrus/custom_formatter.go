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
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

// CustomFormatter allows to turn off logging of some fields.
type CustomFormatter struct {

	// ShowTimestamp decides whether timestamp field should be part of the log
	ShowTimestamp bool

	// ShowLoc decides whether location of the log origin should be part of the log
	ShowLoc bool

	// ShowTag decides if the tag field should be part of the log
	ShowTag bool
}

const (
	fieldKeyMsg       = "msg"
	fieldKeyLevel     = "level"
	fieldKeyTime      = "time"
	fieldKeyComponent = "component"
	fieldKeyProcess   = "process"
)

var compulsoryKeys = []string{fieldKeyTime, fieldKeyLevel, fieldKeyComponent, fieldKeyProcess}

func (f *CustomFormatter) logLevelToString(level log.Level) string {
	switch level {
	case log.DebugLevel:
		return "DEBUG"
	case log.InfoLevel:
		return "INFO"
	case log.WarnLevel:
		return "WARN"
	case log.ErrorLevel:
		return "ERROR"
	case log.FatalLevel:
		return "FATAL"
	case log.PanicLevel:
		return "panic"
	}

	return "unknown"
}

func (f *CustomFormatter) isInSlice(slice []string, key string) bool {
	for i := range slice {
		if slice[i] == key {
			return true
		}
	}
	return false
}

func (f *CustomFormatter) ignoredKey(key string) bool {
	return f.isInSlice(compulsoryKeys, key) || (!f.ShowTag && key == tagKey) || (!f.ShowLoc && key == locKey)
}

// Format formats the given log entry.
func (f *CustomFormatter) Format(entry *log.Entry) ([]byte, error) {
	buffer := &bytes.Buffer{}
	compulsoryFields := map[string]interface{}{}

	compulsoryFields[fieldKeyTime] = entry.Time.Format(time.RFC3339)
	compulsoryFields[fieldKeyMsg] = entry.Message
	compulsoryFields[fieldKeyLevel] = f.logLevelToString(entry.Level)
	if comp, found := entry.Data[fieldKeyComponent]; found {
		compulsoryFields[fieldKeyComponent] = fmt.Sprint(comp)
	} else {
		compulsoryFields[fieldKeyComponent] = "component"
	}

	compulsoryFields[fieldKeyProcess] = os.Getpid()

	// Print header (timestamp can be turned off)
	if f.ShowTimestamp {
		fmt.Fprint(buffer, compulsoryFields[fieldKeyTime])
		buffer.WriteByte(' ')
	}
	for _, key := range compulsoryKeys[1:] {
		fmt.Fprint(buffer, compulsoryFields[key])
		buffer.WriteByte(' ')
	}

	// Print explicit key-value pairs
	for k, v := range entry.Data {
		if !f.ignoredKey(k) {
			f.appendKeyValue(buffer, k, v)
		}
	}

	buffer.WriteString("message=")
	buffer.WriteByte('"')
	fmt.Fprint(buffer, compulsoryFields[fieldKeyMsg])
	buffer.WriteByte('"')
	buffer.WriteByte('\n')

	return buffer.Bytes(), nil
}

func (f *CustomFormatter) appendKeyValue(buffer *bytes.Buffer, key string, value interface{}) {

	buffer.WriteString(key)
	buffer.WriteByte('=')
	f.appendValue(buffer, value)
	buffer.WriteString(", ")
}

func (f *CustomFormatter) appendValue(buffer *bytes.Buffer, value interface{}) {
	switch value := value.(type) {
	case string:
		fmt.Fprintf(buffer, "\"%v\"", value)
	case error:
		buffer.WriteString(value.Error())
	default:
		fmt.Fprint(buffer, value)
	}
}
