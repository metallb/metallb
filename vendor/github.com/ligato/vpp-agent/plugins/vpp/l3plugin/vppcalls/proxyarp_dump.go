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
	"net"

	l3binapi "github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// ProxyArpRangesDetails holds info about proxy ARP range as a proto modeled data
type ProxyArpRangesDetails struct {
	Range *l3.ProxyArpRanges_RangeList_Range
}

// DumpProxyArpRanges implements proxy arp handler.
func (h *ProxyArpVppHandler) DumpProxyArpRanges() (pArpRngs []*ProxyArpRangesDetails, err error) {
	reqCtx := h.callsChannel.SendMultiRequest(&l3binapi.ProxyArpDump{})

	for {
		proxyArpDetails := &l3binapi.ProxyArpDetails{}
		stop, err := reqCtx.ReceiveReply(proxyArpDetails)
		if stop {
			break
		}
		if err != nil {
			h.log.Error(err)
			return nil, err
		}

		pArpRngs = append(pArpRngs, &ProxyArpRangesDetails{
			Range: &l3.ProxyArpRanges_RangeList_Range{
				FirstIp: fmt.Sprintf("%s", net.IP(proxyArpDetails.Proxy.LowAddress[:4]).To4().String()),
				LastIp:  fmt.Sprintf("%s", net.IP(proxyArpDetails.Proxy.HiAddress[:4]).To4().String()),
			},
		})
	}

	return pArpRngs, nil
}

// ProxyArpInterfaceDetails holds info about proxy ARP interfaces as a proto modeled data
type ProxyArpInterfaceDetails struct {
	Interface *l3.ProxyArpInterfaces_InterfaceList_Interface
	Meta      *ProxyArpInterfaceMeta
}

// ProxyArpInterfaceMeta contains interface vpp index
type ProxyArpInterfaceMeta struct {
	SwIfIndex uint32
}

// DumpProxyArpInterfaces implements proxy arp handler.
func (h *ProxyArpVppHandler) DumpProxyArpInterfaces() (pArpIfs []*ProxyArpInterfaceDetails, err error) {
	reqCtx := h.callsChannel.SendMultiRequest(&l3binapi.ProxyArpIntfcDump{})

	for {
		proxyArpDetails := &l3binapi.ProxyArpIntfcDetails{}
		stop, err := reqCtx.ReceiveReply(proxyArpDetails)
		if stop {
			break
		}
		if err != nil {
			h.log.Error(err)
			return nil, err
		}

		// Interface
		ifName, _, exists := h.ifIndexes.LookupName(proxyArpDetails.SwIfIndex)
		if !exists {
			h.log.Warnf("Proxy ARP interface dump: missing name for interface index %d", proxyArpDetails.SwIfIndex)
		}

		// Create entry
		pArpIfs = append(pArpIfs, &ProxyArpInterfaceDetails{
			Interface: &l3.ProxyArpInterfaces_InterfaceList_Interface{
				Name: ifName,
			},
			Meta: &ProxyArpInterfaceMeta{
				SwIfIndex: proxyArpDetails.SwIfIndex,
			},
		})

	}

	return pArpIfs, nil
}
