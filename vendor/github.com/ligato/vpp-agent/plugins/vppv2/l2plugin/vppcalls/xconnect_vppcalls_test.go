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

package vppcalls_test

import (
	"testing"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/logging/logrus"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin/vppcalls"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

var inTestDataXConnect = []struct {
	receiveIfaceIndex  string
	transmitIfaceIndex string
	message            govppapi.Message
}{
	{"rxIf1", "txIf1", &l2ba.SwInterfaceSetL2XconnectReply{}},
	{"rxIf2", "txIf2", &l2ba.SwInterfaceSetL2XconnectReply{Retval: 1}},
	{"rxIf2", "txIf2", &l2ba.BridgeDomainAddDelReply{}},
}

var outTestDataXConnect = []struct {
	outData    *l2ba.SwInterfaceSetL2Xconnect
	isResultOk bool
}{
	{&l2ba.SwInterfaceSetL2Xconnect{
		RxSwIfIndex: 100,
		TxSwIfIndex: 200,
	}, true},
	{&l2ba.SwInterfaceSetL2Xconnect{
		RxSwIfIndex: 101,
		TxSwIfIndex: 201,
	}, false},
	{&l2ba.SwInterfaceSetL2Xconnect{
		RxSwIfIndex: 101,
		TxSwIfIndex: 201,
	}, false},
}

/**
scenarios:
- enabling xconnect
	- ok
	- retvalue != 0
	- returned VPP message != what is expected
*/
// TestVppSetL2XConnect tests VppSetL2XConnect method
func TestVppSetL2XConnect(t *testing.T) {
	ctx, xcHandler, ifaceIdx := xcTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifaceIdx.Put("rxIf1", &ifaceidx.IfaceMetadata{SwIfIndex: 100})
	ifaceIdx.Put("rxIf2", &ifaceidx.IfaceMetadata{SwIfIndex: 101})
	ifaceIdx.Put("txIf1", &ifaceidx.IfaceMetadata{SwIfIndex: 200})
	ifaceIdx.Put("txIf2", &ifaceidx.IfaceMetadata{SwIfIndex: 201})

	for i := 0; i < len(inTestDataXConnect); i++ {
		ctx.MockVpp.MockReply(inTestDataXConnect[i].message)
		err := xcHandler.AddL2XConnect(inTestDataXConnect[i].receiveIfaceIndex,
			inTestDataXConnect[i].transmitIfaceIndex)

		if outTestDataXConnect[i].isResultOk {
			Expect(err).To(BeNil())
		} else {
			Expect(err).NotTo(BeNil())
		}
		outTestDataXConnect[i].outData.Enable = 1
		Expect(ctx.MockChannel.Msg).To(Equal(outTestDataXConnect[i].outData))
	}
}

/**
scenarios:
- disabling xconnect
	- ok
	- retvalue != 0
	- returned VPP message != what is expected
*/
// TestVppUnsetL2XConnect tests VppUnsetL2XConnect method
func TestVppUnsetL2XConnect(t *testing.T) {
	ctx, xcHandler, ifaceIdx := xcTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifaceIdx.Put("rxIf1", &ifaceidx.IfaceMetadata{SwIfIndex: 100})
	ifaceIdx.Put("rxIf2", &ifaceidx.IfaceMetadata{SwIfIndex: 101})
	ifaceIdx.Put("txIf1", &ifaceidx.IfaceMetadata{SwIfIndex: 200})
	ifaceIdx.Put("txIf2", &ifaceidx.IfaceMetadata{SwIfIndex: 201})

	for i := 0; i < len(inTestDataXConnect); i++ {
		ctx.MockVpp.MockReply(inTestDataXConnect[i].message)
		err := xcHandler.DeleteL2XConnect(inTestDataXConnect[i].receiveIfaceIndex,
			inTestDataXConnect[i].transmitIfaceIndex)

		if outTestDataXConnect[i].isResultOk {
			Expect(err).To(BeNil())
		} else {
			Expect(err).NotTo(BeNil())
		}
		outTestDataXConnect[i].outData.Enable = 0
		Expect(ctx.MockChannel.Msg).To(Equal(outTestDataXConnect[i].outData))
	}
}

func xcTestSetup(t *testing.T) (*vppcallmock.TestCtx, vppcalls.XConnectVppAPI, ifaceidx.IfaceMetadataIndexRW) {
	ctx := vppcallmock.SetupTestCtx(t)
	log := logrus.NewLogger("test-log")
	ifaceIdx := ifaceidx.NewIfaceIndex(log, "xc-if-idx")
	xcHandler := vppcalls.NewXConnectVppHandler(ctx.MockChannel, ifaceIdx, log)
	return ctx, xcHandler, ifaceIdx
}
