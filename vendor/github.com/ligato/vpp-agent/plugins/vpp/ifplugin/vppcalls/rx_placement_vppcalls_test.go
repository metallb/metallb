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
	"testing"

	ifApi "github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"

	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	. "github.com/onsi/gomega"
)

func TestSetRxPlacement(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&ifApi.SwInterfaceSetRxPlacementReply{})

	err := ifHandler.SetRxPlacement(1, &interfaces.Interfaces_Interface_RxPlacementSettings{
		Queue:  1,
		Worker: 2,
		IsMain: true,
	})

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*ifApi.SwInterfaceSetRxPlacement)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.QueueID).To(BeEquivalentTo(1))
	Expect(vppMsg.WorkerID).To(BeEquivalentTo(uint32(2)))
	Expect(vppMsg.IsMain).To(BeEquivalentTo(uint32(1)))
}

func TestSetRxPlacementRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&ifApi.SwInterfaceSetRxPlacementReply{
		Retval: 1,
	})

	err := ifHandler.SetRxPlacement(1, &interfaces.Interfaces_Interface_RxPlacementSettings{
		Queue:  1,
		Worker: 2,
	})

	Expect(err).ToNot(BeNil())
}

func TestSetRxPlacementError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&ifApi.SwInterfaceSetRxPlacement{})

	err := ifHandler.SetRxPlacement(1, &interfaces.Interfaces_Interface_RxPlacementSettings{
		Queue:  1,
		Worker: 2,
	})

	Expect(err).ToNot(BeNil())
}
