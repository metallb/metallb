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

package messaging

import (
	"time"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
)

// DefaultMsgTimeout for delivery of notification
const DefaultMsgTimeout = 2 * time.Second

// ToProtoMsgChan allows to receive messages through channel instead of callback.
func ToProtoMsgChan(ch chan ProtoMessage, opts ...interface{}) func(ProtoMessage) {

	timeout, logger := ParseOpts(opts...)

	return func(msg ProtoMessage) {
		select {
		case ch <- msg:
		case <-time.After(timeout):
			logger.Warn("Unable to deliver message")
		}
	}
}

// ToProtoMsgErrChan allows to receive error messages through channel instead
// of callback.
func ToProtoMsgErrChan(ch chan ProtoMessageErr, opts ...interface{}) func(ProtoMessageErr) {

	timeout, logger := ParseOpts(opts...)

	return func(msg ProtoMessageErr) {
		select {
		case ch <- msg:
		case <-time.After(timeout):
			logger.Warn("Unable to deliver message")
		}
	}
}

// ParseOpts returns timeout and logger to be used based on the given options.
func ParseOpts(opts ...interface{}) (time.Duration, logging.Logger) {
	timeout := DefaultMsgTimeout
	var logger logging.Logger = logrus.DefaultLogger()

	for _, opt := range opts {
		switch opt.(type) {
		case *WithLoggerOpt:
			logger = opt.(*WithLoggerOpt).logger
		case *WithTimeoutOpt:
			timeout = opt.(*WithTimeoutOpt).timeout
		}
	}
	return timeout, logger

}

// WithTimeoutOpt defines the maximum time allocated to deliver a notification.
type WithTimeoutOpt struct {
	timeout time.Duration
}

// WithTimeout creates an option for ToChan function that defines a timeout for
// notification delivery.
func WithTimeout(timeout time.Duration) *WithTimeoutOpt {
	return &WithTimeoutOpt{timeout: timeout}
}

// WithLoggerOpt defines a logger that logs if delivery of notification is
// unsuccessful.
type WithLoggerOpt struct {
	logger logging.Logger
}

// WithLogger creates an option for ToChan function that specifies a logger
// to be used.
func WithLogger(logger logging.Logger) *WithLoggerOpt {
	return &WithLoggerOpt{logger: logger}
}
