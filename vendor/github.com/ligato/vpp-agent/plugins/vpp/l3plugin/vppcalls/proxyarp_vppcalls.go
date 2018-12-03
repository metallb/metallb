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

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
)

// EnableProxyArpInterface implements proxy arp handler.
func (h *ProxyArpVppHandler) EnableProxyArpInterface(swIfIdx uint32) error {
	return h.vppAddDelProxyArpInterface(swIfIdx, true)
}

// DisableProxyArpInterface implements proxy arp handler.
func (h *ProxyArpVppHandler) DisableProxyArpInterface(swIfIdx uint32) error {
	return h.vppAddDelProxyArpInterface(swIfIdx, false)
}

// AddProxyArpRange implements proxy arp handler.
func (h *ProxyArpVppHandler) AddProxyArpRange(firstIP, lastIP []byte) error {
	return h.vppAddDelProxyArpRange(firstIP, lastIP, true)
}

// DeleteProxyArpRange implements proxy arp handler.
func (h *ProxyArpVppHandler) DeleteProxyArpRange(firstIP, lastIP []byte) error {
	return h.vppAddDelProxyArpRange(firstIP, lastIP, false)
}

// vppAddDelProxyArpInterface adds or removes proxy ARP interface entry according to provided input
func (h *ProxyArpVppHandler) vppAddDelProxyArpInterface(swIfIdx uint32, enable bool) error {
	req := &ip.ProxyArpIntfcEnableDisable{}
	if enable {
		req.EnableDisable = 1
	} else {
		req.EnableDisable = 0
	}
	req.SwIfIndex = swIfIdx

	// Send message
	reply := &ip.ProxyArpIntfcEnableDisableReply{}
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}
	if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	h.log.Debugf("interface %v enabled for proxy arp: %v", req.SwIfIndex, enable)

	return nil
}

// vppAddDelProxyArpRange adds or removes proxy ARP range according to provided input
func (h *ProxyArpVppHandler) vppAddDelProxyArpRange(firstIP, lastIP []byte, isAdd bool) error {
	req := &ip.ProxyArpAddDel{}
	if isAdd {
		req.IsAdd = 1
	} else {
		req.IsAdd = 0
	}
	req.Proxy = ip.ProxyArp{
		LowAddress: firstIP,
		HiAddress:  lastIP,
	}
	reply := &ip.ProxyArpAddDelReply{}

	// Send message
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	} else if reply.Retval != 0 {
		return fmt.Errorf("%s returned %d", reply.GetMessageName(), reply.Retval)
	}

	h.log.Debugf("proxy arp range: %v - %v added: %v", req.Proxy.LowAddress, req.Proxy.HiAddress, isAdd)

	return nil
}
