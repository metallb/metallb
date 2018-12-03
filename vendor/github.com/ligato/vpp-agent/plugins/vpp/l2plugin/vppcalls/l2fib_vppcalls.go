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
	"net"

	"github.com/ligato/cn-infra/logging"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
)

// FibLogicalReq groups multiple fields so that all of them do
// not enumerate in one function call (request, reply/callback).
type FibLogicalReq struct {
	IsAdd    bool
	MAC      string
	BDIdx    uint32
	SwIfIdx  uint32
	BVI      bool
	Static   bool
	callback func(error)
}

// Add implements fib handler.
func (h *FibVppHandler) Add(mac string, bdID uint32, ifIdx uint32, bvi bool, static bool, callback func(error)) error {
	h.log.Debug("Adding L2 FIB table entry, mac: ", mac)

	h.requestChan <- &FibLogicalReq{
		IsAdd:    true,
		MAC:      mac,
		BDIdx:    bdID,
		SwIfIdx:  ifIdx,
		BVI:      bvi,
		Static:   static,
		callback: callback,
	}
	return nil
}

// Delete implements fib handler.
func (h *FibVppHandler) Delete(mac string, bdID uint32, ifIdx uint32, callback func(error)) error {
	h.log.Debug("Removing L2 fib table entry, mac: ", mac)

	h.requestChan <- &FibLogicalReq{
		IsAdd:    false,
		MAC:      mac,
		BDIdx:    bdID,
		SwIfIdx:  ifIdx,
		callback: callback,
	}
	return nil
}

// WatchFIBReplies implements fib handler.
func (h *FibVppHandler) WatchFIBReplies() {
	for {
		select {
		case r := <-h.requestChan:
			h.log.Debug("VPP L2FIB request: ", r)
			err := h.l2fibAddDel(r.MAC, r.BDIdx, r.SwIfIdx, r.BVI, r.Static, r.IsAdd)
			if err != nil {
				h.log.WithFields(logging.Fields{"mac": r.MAC, "ifIdx": r.SwIfIdx, "bdIdx": r.BDIdx}).
					Error("Static fib entry add/delete failed:", err)
			} else {
				h.log.WithFields(logging.Fields{"mac": r.MAC, "ifIdx": r.SwIfIdx, "bdIdx": r.BDIdx}).
					Debug("Static fib entry added/deleted.")
			}
			r.callback(err)
		}
	}
}

func (h *FibVppHandler) l2fibAddDel(macstr string, bdIdx, swIfIdx uint32, bvi, static, isAdd bool) (err error) {
	var mac []byte
	if macstr != "" {
		mac, err = net.ParseMAC(macstr)
		if err != nil {
			return err
		}
	}

	req := &l2ba.L2fibAddDel{
		IsAdd:     boolToUint(isAdd),
		Mac:       mac,
		BdID:      bdIdx,
		SwIfIndex: swIfIdx,
		BviMac:    boolToUint(bvi),
		StaticMac: boolToUint(static),
	}
	reply := &l2ba.L2fibAddDelReply{}

	if err := h.asyncCallsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	return nil
}
