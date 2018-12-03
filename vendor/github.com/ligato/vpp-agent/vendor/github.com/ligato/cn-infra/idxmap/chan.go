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

package idxmap

import (
	"time"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
)

// DefaultNotifTimeout for delivery of notification
const DefaultNotifTimeout = 2 * time.Second

// ToChan creates a callback that can be passed to the Watch function
// in order to receive notifications through a channel. If the notification
// can not be delivered until timeout, it is dropped.
func ToChan(ch chan NamedMappingGenericEvent, opts ...interface{}) func(dto NamedMappingGenericEvent) {

	timeout := DefaultNotifTimeout
	var logger logging.Logger = logrus.DefaultLogger()

	/*for _, opt := range opts {
		switch opt.(type) {
		case *core.WithLoggerOpt:
			logger = opt.(*core.WithLoggerOpt).Logger
		case *core.WithTimeoutOpt:
			timeout = opt.(*core.WithTimeoutOpt).Timeout
		}
	}*/

	return func(dto NamedMappingGenericEvent) {
		select {
		case ch <- dto:
		case <-time.After(timeout):
			logger.Warn("Unable to deliver notification")
		}
	}
}
