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

package vppcalls

import (
	"fmt"

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// SetIPScanNeighbor implements ip neigh  handler.
func (h *IPNeighHandler) SetIPScanNeighbor(data *l3.IPScanNeighbor) error {
	req := &ip.IPScanNeighborEnableDisable{
		Mode:           uint8(data.Mode),
		ScanInterval:   uint8(data.ScanInterval),
		MaxProcTime:    uint8(data.MaxProcTime),
		MaxUpdate:      uint8(data.MaxUpdate),
		ScanIntDelay:   uint8(data.ScanIntDelay),
		StaleThreshold: uint8(data.StaleThreshold),
	}
	reply := &ip.IPScanNeighborEnableDisableReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}
