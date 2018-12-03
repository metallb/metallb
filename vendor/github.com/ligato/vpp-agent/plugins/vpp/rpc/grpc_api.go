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

	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
)

// GRPCService allows to send VPP notifications to external GRPC endpoints
type GRPCService interface {

	// updateNotification stores VPP notifications/statistic data. The notification can be read by any client
	// connected to the notification service server
	// todo make type independent
	UpdateNotifications(ctx context.Context, notification *interfaces.InterfaceNotification)
}
