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

package resync

import (
	"time"
)

// newStatusEvent is a constructor.
func newStatusEvent(status Status) *statusEvent {
	return &statusEvent{status: status, ackChan: make(chan time.Time)}
}

// StatusEvent is propagated to Plugins using GOLANG channel.
type statusEvent struct {
	status  Status
	ackChan chan time.Time
}

// Status gets the status.
func (event *statusEvent) ResyncStatus() Status {
	return event.status
}

// Ack - see the comment in interface chngapi.StatusEvent.Ack().
func (event *statusEvent) Ack() {
	event.ackChan <- time.Now()
}

// ReceiveAck allows waiting until Plugin calls the Ack().
func (event *statusEvent) ReceiveAck() chan time.Time {
	return event.ackChan
}
