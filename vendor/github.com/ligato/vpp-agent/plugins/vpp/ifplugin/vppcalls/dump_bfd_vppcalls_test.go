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

package vppcalls_test

import (
	"net"
	"testing"

	bfd_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	. "github.com/onsi/gomega"
)

// TestDumpBfdUDPSessions tests BFD udp session dump
func TestDumpBfdUDPSessions(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPSessionDetails{
		SwIfIndex: 1,
		LocalAddr: net.ParseIP("10.0.0.1"),
		PeerAddr:  net.ParseIP("20.0.0.1"),
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	bfdConfig, err := bfdHandler.DumpBfdSessions()

	Expect(err).To(BeNil())
	Expect(bfdConfig.Session).To(HaveLen(1))
}

// TestDumpBfdUDPSessions tests BFD udp session dump where the result is filtered
// according to session authentication key ID
func TestDumpBfdUDPSessionsWithID(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	// Authenticated wiht ID 1
	ctx.MockVpp.MockReply(
		&bfd_api.BfdUDPSessionDetails{
			SwIfIndex:       1,
			LocalAddr:       net.ParseIP("10.0.0.1"),
			PeerAddr:        net.ParseIP("20.0.0.1"),
			IsAuthenticated: 1,
			BfdKeyID:        1,
		},
		// Authenticated with ID 2 (filtered)
		&bfd_api.BfdUDPSessionDetails{
			SwIfIndex:       2,
			LocalAddr:       net.ParseIP("10.0.0.2"),
			PeerAddr:        net.ParseIP("20.0.0.2"),
			IsAuthenticated: 1,
			BfdKeyID:        2,
		},
		// Not authenticated
		&bfd_api.BfdUDPSessionDetails{
			SwIfIndex:       3,
			LocalAddr:       net.ParseIP("10.0.0.3"),
			PeerAddr:        net.ParseIP("20.0.0.3"),
			IsAuthenticated: 0,
		})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	bfdConfig, err := bfdHandler.DumpBfdUDPSessionsWithID(1)

	Expect(err).To(BeNil())
	Expect(bfdConfig.Session).To(HaveLen(1))
}

// TestDumpBfdKeys tests BFD key dump
func TestDumpBfdKeys(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthKeysDetails{
		ConfKeyID: 1,
		UseCount:  0,
		AuthType:  4,
	},
		&bfd_api.BfdAuthKeysDetails{
			ConfKeyID: 2,
			UseCount:  1,
			AuthType:  5,
		})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	bfdConfig, err := bfdHandler.DumpBfdAuthKeys()

	Expect(err).To(BeNil())
	Expect(bfdConfig.AuthKeys).To(HaveLen(2))
}
