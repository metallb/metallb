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

package keyval

import (
	"time"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging"
)

// BytesWatcher defines API for monitoring changes in datastore.
type BytesWatcher interface {
	// Watch starts subscription for changes associated with the selected keys.
	// Watch events will be delivered to callback (not channel) <respChan>.
	// Channel <closeChan> can be used to close watching on respective key
	Watch(respChan func(BytesWatchResp), closeChan chan string, keys ...string) error
}

// BytesWatchResp represents a notification about data change.
// It is sent through the respChan callback.
type BytesWatchResp interface {
	BytesKvPair
	datasync.WithChangeType
	datasync.WithRevision
}

// ToChan creates a callback that can be passed to the Watch function in order
// to receive notifications through a channel. If the notification cannot be
// delivered until timeout, it is dropped.
func ToChan(respCh chan BytesWatchResp, opts ...interface{}) func(dto BytesWatchResp) {
	return func(dto BytesWatchResp) {
		select {
		case respCh <- dto:
			// success
		case <-time.After(datasync.DefaultNotifTimeout):
			logging.DefaultLogger.Warn("Unable to deliver notification")
		}
	}
}
