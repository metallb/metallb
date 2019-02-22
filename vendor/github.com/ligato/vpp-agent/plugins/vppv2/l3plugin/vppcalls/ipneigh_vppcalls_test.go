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
	"testing"

	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

func TestGetIPScanNeighbor(t *testing.T) {
	tests := []struct {
		name     string
		cliReply string
		expected l3.IPScanNeighbor
	}{
		{
			name:     "mode-disabled",
			cliReply: `IP neighbor scan disabled - current time is 3575.2641 sec`,
			expected: l3.IPScanNeighbor{
				Mode: l3.IPScanNeighbor_DISABLED,
			},
		},
		{
			name: "mode-ipv4",
			cliReply: `IP neighbor scan enabled for IPv4 neighbors - current time is 2583.4566 sec
   Full_scan_interval: 1 min  Stale_purge_threshod: 4 min
   Max_process_time: 20 usec  Max_updates 10  Delay_to_resume_after_max_limit: 1 msec`,
			expected: l3.IPScanNeighbor{
				Mode:           l3.IPScanNeighbor_IPv4,
				ScanInterval:   1,
				MaxProcTime:    20,
				MaxUpdate:      10,
				ScanIntDelay:   1,
				StaleThreshold: 4,
			},
		},
		{
			name: "mode-both",
			cliReply: `IP neighbor scan enabled for IPv4 and IPv6 neighbors - current time is 95.6033 sec
   Full_scan_interval: 3 min  Stale_purge_threshod: 5 min
   Max_process_time: 200 usec  Max_updates 10  Delay_to_resume_after_max_limit: 100 msec`,
			expected: l3.IPScanNeighbor{
				Mode:           l3.IPScanNeighbor_BOTH,
				ScanInterval:   3,
				MaxProcTime:    200,
				MaxUpdate:      10,
				ScanIntDelay:   100,
				StaleThreshold: 5,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := vppcallmock.SetupTestCtx(t)
			defer ctx.TeardownTestCtx()

			ctx.MockVpp.MockReply(&vpe.CliInbandReply{
				Reply:  []byte(test.cliReply),
				Length: uint32(len(test.cliReply)),
			})

			handler := NewIPNeighVppHandler(ctx.MockChannel, nil)

			ipNeigh, err := handler.GetIPScanNeighbor()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ipNeigh.Mode).To(Equal(test.expected.Mode))
			Expect(ipNeigh.ScanInterval).To(Equal(test.expected.ScanInterval))
			Expect(ipNeigh.ScanIntDelay).To(Equal(test.expected.ScanIntDelay))
			Expect(ipNeigh.StaleThreshold).To(Equal(test.expected.StaleThreshold))
			Expect(ipNeigh.MaxUpdate).To(Equal(test.expected.MaxUpdate))
			Expect(ipNeigh.MaxProcTime).To(Equal(test.expected.MaxProcTime))
		})
	}
}
