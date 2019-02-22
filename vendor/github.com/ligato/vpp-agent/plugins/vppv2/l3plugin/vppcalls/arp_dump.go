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
	"fmt"
	"net"

	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	l3binapi "github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
)

// ArpDetails holds info about ARP entry as a proto model
type ArpDetails struct {
	Arp  *l3.ARPEntry
	Meta *ArpMeta
}

// ArpMeta contains interface index of the ARP interface
type ArpMeta struct {
	SwIfIndex uint32
}

// DumpArpEntries implements arp handler.
func (h *ArpVppHandler) DumpArpEntries() ([]*ArpDetails, error) {
	var entries []*ArpDetails
	reqCtx := h.callsChannel.SendMultiRequest(&l3binapi.IPNeighborDump{
		SwIfIndex: 0xffffffff, // Send multirequest to get all ARP entries
	})

	for {
		arpDetails := &l3binapi.IPNeighborDetails{}
		stop, err := reqCtx.ReceiveReply(arpDetails)
		if stop {
			break
		}
		if err != nil {
			h.log.Error(err)
			return nil, err
		}

		// ARP interface
		ifName, _, exists := h.ifIndexes.LookupBySwIfIndex(arpDetails.SwIfIndex)
		if !exists {
			h.log.Warnf("ARP dump: interface name not found for index %d", arpDetails.SwIfIndex)
		}
		// IP & MAC address
		var ip string
		if arpDetails.IsIPv6 == 1 {
			ip = fmt.Sprintf("%s", net.IP(arpDetails.IPAddress).To16().String())
		} else {
			ip = fmt.Sprintf("%s", net.IP(arpDetails.IPAddress[:4]).To4().String())
		}

		// ARP entry
		arp := &l3.ARPEntry{
			Interface:   ifName,
			IpAddress:   ip,
			PhysAddress: net.HardwareAddr(arpDetails.MacAddress).String(),
			Static:      uintToBool(arpDetails.IsStatic),
		}
		// ARP meta
		meta := &ArpMeta{
			SwIfIndex: arpDetails.SwIfIndex,
		}

		entries = append(entries, &ArpDetails{
			Arp:  arp,
			Meta: meta,
		})
	}

	return entries, nil
}
