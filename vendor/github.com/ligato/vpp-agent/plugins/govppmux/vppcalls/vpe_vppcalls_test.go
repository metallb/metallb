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

package vppcalls_test

import (
	"testing"

	"github.com/ligato/vpp-agent/plugins/govppmux/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

func TestGetBuffers(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	const reply = `Thread             Name                 Index       Size        Alloc       Free       #Alloc       #Free  
     0                       default           0        2048    576k       42.75k        256         19    
     0                 lacp-ethernet           1         256    1.13m        27k         512         12    
     0               marker-ethernet           2         256    1.11g         0           0           0    
     0                       ip4 arp           3         256      0           0           0           0    
     0        ip6 neighbor discovery           4         256      0           0           0           0    
     0                  cdp-ethernet           5         256      0           0           0           0    
     0                 lldp-ethernet           6         256      0           0           0           0    
     0           replication-recycle           7        1024      0           0           0           0    
     0                       default           8        2048      0           0           0           0    `
	ctx.MockVpp.MockReply(&vpe.CliInbandReply{
		Reply:  []byte(reply),
		Length: uint32(len(reply)),
	})

	info, err := vppcalls.GetBuffersInfo(ctx.MockChannel)

	Expect(err).ShouldNot(HaveOccurred())
	Expect(info.Items).To(HaveLen(9))
	Expect(info.Items[0]).To(Equal(vppcalls.BuffersItem{
		ThreadID: 0,
		Name:     "default",
		Index:    0,
		Size:     2048,
		Alloc:    576000,
		Free:     42750,
		NumAlloc: 256,
		NumFree:  19,
	}))
	Expect(info.Items[1]).To(Equal(vppcalls.BuffersItem{
		ThreadID: 0,
		Name:     "lacp-ethernet",
		Index:    1,
		Size:     256,
		Alloc:    1130000,
		Free:     27000,
		NumAlloc: 512,
		NumFree:  12,
	}))
	Expect(info.Items[2]).To(Equal(vppcalls.BuffersItem{
		ThreadID: 0,
		Name:     "marker-ethernet",
		Index:    2,
		Size:     256,
		Alloc:    1110000000,
		Free:     0,
		NumAlloc: 0,
		NumFree:  0,
	}))
}

func TestGetRuntime(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	const reply = `Thread 0 vpp_main (lcore 0)
Time 21.5, average vectors/node 0.00, last 128 main loops 0.00 per node 0.00
  vector rates in 0.0000e0, out 0.0000e0, drop 0.0000e0, punt 0.0000e0
             Name                 State         Calls          Vectors        Suspends         Clocks       Vectors/Call  
acl-plugin-fa-cleaner-process  event wait                0               0               1          3.12e4            0.00
api-rx-from-ring                any wait                 0               0              31          8.61e6            0.00
avf-process                    event wait                0               0               1          7.79e3            0.00
bfd-process                    event wait                0               0               1          6.80e3            0.00
cdp-process                     any wait                 0               0               1          1.78e8            0.00
dhcp-client-process             any wait                 0               0               1          2.59e3            0.00
dns-resolver-process            any wait                 0               0               1          3.35e3            0.00
fib-walk                        any wait                 0               0              11          1.08e4            0.00
flow-report-process             any wait                 0               0               1          1.64e3            0.00
flowprobe-timer-process         any wait                 0               0               1          1.16e4            0.00
igmp-timer-process             event wait                0               0               1          1.81e4            0.00
ikev2-manager-process           any wait                 0               0              22          5.47e3            0.00
ioam-export-process             any wait                 0               0               1          3.26e3            0.00
ip-route-resolver-process       any wait                 0               0               1          1.69e3            0.00
ip4-reassembly-expire-walk      any wait                 0               0               3          4.27e3            0.00
ip6-icmp-neighbor-discovery-ev  any wait                 0               0              22          4.48e3            0.00
ip6-reassembly-expire-walk      any wait                 0               0               3          6.88e3            0.00
l2fib-mac-age-scanner-process  event wait                0               0               1          3.94e3            0.00
lacp-process                   event wait                0               0               1          1.35e8            0.00
lisp-retry-service              any wait                 0               0              11          9.68e3            0.00
lldp-process                   event wait                0               0               1          1.49e8            0.00
memif-process                  event wait                0               0               1          2.67e4            0.00
nat-det-expire-walk               done                   1               0               0          5.42e3            0.00
nat64-expire-walk              event wait                0               0               1          5.87e4            0.00
rd-cp-process                   any wait                 0               0          614363          3.93e2            0.00
send-rs-process                 any wait                 0               0               1          3.22e3            0.00
startup-config-process            done                   1               0               1          1.33e4            0.00
udp-ping-process                any wait                 0               0               1          3.69e4            0.00
unix-cli-127.0.0.1:38448         active                  0               0              23          6.72e7            0.00
unix-epoll-input                 polling           8550283               0               0          3.77e3            0.00
vhost-user-process              any wait                 0               0               1          2.48e3            0.00
vhost-user-send-interrupt-proc  any wait                 0               0               1          1.43e3            0.00
vpe-link-state-process         event wait                0               0               1          1.58e3            0.00
vpe-oam-process                 any wait                 0               0              11          9.20e3            0.00
vxlan-gpe-ioam-export-process   any wait                 0               0               1          1.59e4            0.00
wildcard-ip4-arp-publisher-pro event wait                0               0               1          1.03e4            0.00
---------------
Thread 1 vpp_wk_0 (lcore 1)
Time 21.5, average vectors/node 0.00, last 128 main loops 0.00 per node 0.00
  vector rates in 0.0000e0, out 0.0000e0, drop 0.0000e0, punt 0.0000e0
             Name                 State         Calls          Vectors        Suspends         Clocks       Vectors/Call  
unix-epoll-input                 polling          15251181               0               0          3.67e3            0.00
---------------
Thread 2 vpp_wk_1 (lcore 2)
Time 21.5, average vectors/node 0.00, last 128 main loops 0.00 per node 0.00
  vector rates in 0.0000e0, out 0.0000e0, drop 0.0000e0, punt 0.0000e0
             Name                 State         Calls          Vectors        Suspends         Clocks       Vectors/Call  
unix-epoll-input                 polling          20563870               0               0          3.56e3            0.00
`
	ctx.MockVpp.MockReply(&vpe.CliInbandReply{
		Reply:  []byte(reply),
		Length: uint32(len(reply)),
	})

	info, err := vppcalls.GetRuntimeInfo(ctx.MockChannel)

	Expect(err).ShouldNot(HaveOccurred())
	Expect(info.Threads).To(HaveLen(3))
	Expect(info.Threads[0].Items).To(HaveLen(36))
	Expect(info.Threads[0].Items[0]).To(Equal(vppcalls.RuntimeItem{
		Name:           "acl-plugin-fa-cleaner-process",
		State:          "event wait",
		Calls:          0,
		Vectors:        0,
		Suspends:       1,
		Clocks:         3.12e4,
		VectorsPerCall: 0,
	}))
}

func TestGetMemory(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	const reply = `Thread 0 vpp_main
22991 objects, 19199k of 24937k used, 5196k free, 5168k reclaimed, 361k overhead, 1048572k capacity
Thread 1 vpp_wk_0
22991 objects, 19199k of 24937k used, 5196k free, 5168k reclaimed, 361k overhead, 1048572k capacity
Thread 2 vpp_wk_1
22991 objects, 19199k of 24937k used, 5196k free, 5168k reclaimed, 361k overhead, 1048572k capacity
`
	ctx.MockVpp.MockReply(&vpe.CliInbandReply{
		Reply:  []byte(reply),
		Length: uint32(len(reply)),
	})

	info, err := vppcalls.GetMemory(ctx.MockChannel)

	Expect(err).ShouldNot(HaveOccurred())
	Expect(info.Threads).To(HaveLen(3))
	Expect(info.Threads[0]).To(Equal(vppcalls.MemoryThread{
		ID:        0,
		Name:      "vpp_main",
		Objects:   22991,
		Used:      19199000,
		Total:     24937000,
		Free:      5196000,
		Reclaimed: 5168000,
		Overhead:  361000,
		Capacity:  1048572000,
	}))
	Expect(info.Threads[1]).To(Equal(vppcalls.MemoryThread{
		ID:        1,
		Name:      "vpp_wk_0",
		Objects:   22991,
		Used:      19199000,
		Total:     24937000,
		Free:      5196000,
		Reclaimed: 5168000,
		Overhead:  361000,
		Capacity:  1048572000,
	}))
}

func TestGetNodeCounters(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	defer ctx.TeardownTestCtx()

	const reply = `   Count                    Node                  Reason
        32            ipsec-output-ip4            IPSec policy protect
        32               esp-encrypt              ESP pkts received
        64             ipsec-input-ip4            IPSEC pkts received
        32             ip4-icmp-input             unknown type
        32             ip4-icmp-input             echo replies sent
        14             ethernet-input             l3 mac mismatch
         1                arp-input               ARP replies sent
         4                ip4-input               ip4 spoofed local-address packet drops
         2             memif1/1-output            interface is down
         1                cdp-input               good cdp packets (processed)
`
	ctx.MockVpp.MockReply(&vpe.CliInbandReply{
		Reply:  []byte(reply),
		Length: uint32(len(reply)),
	})

	info, err := vppcalls.GetNodeCounters(ctx.MockChannel)

	Expect(err).ShouldNot(HaveOccurred())
	Expect(info.Counters).To(HaveLen(10))
	Expect(info.Counters[0]).To(Equal(vppcalls.NodeCounter{
		Count:  32,
		Node:   "ipsec-output-ip4",
		Reason: "IPSec policy protect",
	}))
	Expect(info.Counters[6]).To(Equal(vppcalls.NodeCounter{
		Count:  1,
		Node:   "arp-input",
		Reason: "ARP replies sent",
	}))
	Expect(info.Counters[7]).To(Equal(vppcalls.NodeCounter{
		Count:  4,
		Node:   "ip4-input",
		Reason: "ip4 spoofed local-address packet drops",
	}))
	Expect(info.Counters[8]).To(Equal(vppcalls.NodeCounter{
		Count:  2,
		Node:   "memif1/1-output",
		Reason: "interface is down",
	}))
	Expect(info.Counters[9]).To(Equal(vppcalls.NodeCounter{
		Count:  1,
		Node:   "cdp-input",
		Reason: "good cdp packets (processed)",
	}))
}
