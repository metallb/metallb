//  Copyright (c) 2018 Cisco and/or its affiliates.
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

package vppcalls

import (
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/pkg/errors"
)

// EnableProxyArpInterface implements proxy arp handler.
func (h *ProxyArpVppHandler) EnableProxyArpInterface(ifName string) error {
	return h.vppAddDelProxyArpInterface(ifName, true)
}

// DisableProxyArpInterface implements proxy arp handler.
func (h *ProxyArpVppHandler) DisableProxyArpInterface(ifName string) error {
	return h.vppAddDelProxyArpInterface(ifName, false)
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
func (h *ProxyArpVppHandler) vppAddDelProxyArpInterface(ifName string, enable bool) error {
	meta, found := h.ifIndexes.LookupByName(ifName)
	if !found {
		return errors.Errorf("interface %s not found", ifName)
	}

	req := &ip.ProxyArpIntfcEnableDisable{
		EnableDisable: boolToUint(enable),
		SwIfIndex:     meta.SwIfIndex,
	}

	reply := &ip.ProxyArpIntfcEnableDisableReply{}
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	h.log.Debugf("interface %v enabled for proxy arp: %v", req.SwIfIndex, enable)

	return nil
}

// vppAddDelProxyArpRange adds or removes proxy ARP range according to provided input
func (h *ProxyArpVppHandler) vppAddDelProxyArpRange(firstIP, lastIP []byte, isAdd bool) error {
	req := &ip.ProxyArpAddDel{
		IsAdd: boolToUint(isAdd),
		Proxy: ip.ProxyArp{
			VrfID:      0, // TODO: add support for VRF
			LowAddress: firstIP,
			HiAddress:  lastIP,
		},
	}

	reply := &ip.ProxyArpAddDelReply{}
	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	h.log.Debugf("proxy arp range: %v - %v added: %v", req.Proxy.LowAddress, req.Proxy.HiAddress, isAdd)

	return nil
}
