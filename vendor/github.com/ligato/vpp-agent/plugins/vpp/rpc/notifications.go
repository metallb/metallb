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

package rpc

import (
	"context"
	"sync"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/rpc"
)

// Maximum number of messages stored in the buffer. Buffer is always filled from left
// to right (it means that if the buffer is full, a new entry is written to the index 0)
const bufferSize = 100

// NotificationSvc forwards GRPC messages to external servers.
type NotificationSvc struct {
	mx sync.RWMutex

	log logging.Logger

	// VPP notifications available for clients
	nBuffer [bufferSize]*rpc.NotificationsResponse
	nIdx    uint32
}

// Get returns all required VPP notifications (or those available in the buffer) in the same order as they were received
func (svc *NotificationSvc) Get(from *rpc.NotificationRequest, server rpc.NotificationService_GetServer) error {
	svc.mx.RLock()
	defer svc.mx.RUnlock()

	// Copy requested index locally
	fromIdx := from.Idx

	// Check if requested index overflows buffer length
	if svc.nIdx-from.Idx > bufferSize {
		fromIdx = svc.nIdx - bufferSize
	}

	// Start from requested index until the most recent entry
	for i := fromIdx; i < svc.nIdx; i++ {
		entry := svc.nBuffer[i%bufferSize]
		if err := server.Send(entry); err != nil {
			svc.log.Error("Send notification error: %v", err)
			return err
		}
	}

	return nil
}

// Adds new notification to the pool. The order of notifications is preserved
func (svc *NotificationSvc) updateNotifications(ctx context.Context, notification *interfaces.InterfaceNotification) {
	svc.mx.Lock()
	defer svc.mx.Unlock()

	// Notification index starts with 1
	svc.nBuffer[svc.nIdx%bufferSize] = &rpc.NotificationsResponse{
		NextIdx: svc.nIdx + 1,
		NIf:     notification,
	}
	svc.nIdx++
}
