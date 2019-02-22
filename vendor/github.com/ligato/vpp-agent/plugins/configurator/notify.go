//  Copyright (c) 2019 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package configurator

import (
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	rpc "github.com/ligato/vpp-agent/api/configurator"
)

// Maximum number of messages stored in the buffer. Buffer is always filled from left
// to right (it means that if the buffer is full, a new entry is written to the index 0)
const bufferSize = 1000

// notifyService forwards GRPC messages to external servers.
type notifyService struct {
	log logging.Logger

	// VPP notifications available for clients
	mx     sync.RWMutex
	buffer [bufferSize]rpc.NotificationResponse
	curIdx uint32
}

// Notify returns all required VPP notifications (or those available in the buffer) in the same order as they were received
func (svc *notifyService) Notify(from *rpc.NotificationRequest, server rpc.Configurator_NotifyServer) error {
	svc.mx.RLock()
	defer svc.mx.RUnlock()

	// Copy requested index locally
	fromIdx := from.Idx

	// Check if requested index overflows buffer length
	if svc.curIdx-from.Idx > bufferSize {
		fromIdx = svc.curIdx - bufferSize
	}

	// Start from requested index until the most recent entry
	for i := fromIdx; i < svc.curIdx; i++ {
		entry := svc.buffer[i%bufferSize]
		if err := server.Send(&entry); err != nil {
			svc.log.Error("Send notification error: %v", err)
			return err
		}
	}

	return nil
}

// Pushes new notification to the buffer. The order of notifications is preserved.
func (svc *notifyService) pushNotification(notification *rpc.Notification) {
	// notification is cloned to ensure it does not get changed after storing
	notifCopy := proto.Clone(notification).(*rpc.Notification)

	svc.mx.Lock()
	defer svc.mx.Unlock()

	// Notification index starts with 1
	notif := rpc.NotificationResponse{
		NextIdx:      svc.curIdx + 1,
		Notification: notifCopy,
	}
	svc.buffer[svc.curIdx%bufferSize] = notif
	svc.curIdx++
}
