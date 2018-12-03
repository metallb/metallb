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

package vppcalls

import (
	"fmt"

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/session"
)

// AddAppNamespace adds app namespace.
func (h *L4VppHandler) AddAppNamespace(secret uint64, swIfIdx, ip4FibID, ip6FibID uint32, id []byte) (appNsIdx uint32, err error) {
	req := &session.AppNamespaceAddDel{
		SwIfIndex:      swIfIdx,
		Secret:         secret,
		IP4FibID:       ip4FibID,
		IP6FibID:       ip6FibID,
		NamespaceID:    id,
		NamespaceIDLen: uint8(len(id)),
	}
	reply := &session.AppNamespaceAddDelReply{}

	if err = h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, err
	} else if reply.Retval != 0 {
		return 0, fmt.Errorf("%s returned %v", reply.GetMessageName(), reply.Retval)
	}

	return reply.AppnsIndex, nil
}
