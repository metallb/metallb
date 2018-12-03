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

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	bfd_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/bfd"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vpp/model/bfd"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

func TestAddBfdUDPSession(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)
	bfdKeyIndexes.RegisterName(ifplugin.AuthKeyIdentifier(1), 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "10.0.0.1",
		DestinationAddress:    "20.0.0.1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
		Authentication: &bfd.SingleHopBFD_Session_Authentication{
			KeyId:           1,
			AdvertisedKeyId: 1,
		},
	}, 1, bfdKeyIndexes)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPAdd)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.DesiredMinTx).To(BeEquivalentTo(10))
	Expect(vppMsg.RequiredMinRx).To(BeEquivalentTo(15))
	Expect(vppMsg.DetectMult).To(BeEquivalentTo(2))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAuthenticated).To(BeEquivalentTo(1))
}

func TestAddBfdUDPSessionIPv6(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)
	bfdKeyIndexes.RegisterName(ifplugin.AuthKeyIdentifier(1), 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "2001:db8::1",
		DestinationAddress:    "2001:db8:0:1:1:1:1:1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
		Authentication: &bfd.SingleHopBFD_Session_Authentication{
			KeyId:           1,
			AdvertisedKeyId: 1,
		},
	}, 1, bfdKeyIndexes)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPAdd)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.DesiredMinTx).To(BeEquivalentTo(10))
	Expect(vppMsg.RequiredMinRx).To(BeEquivalentTo(15))
	Expect(vppMsg.DetectMult).To(BeEquivalentTo(2))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(1))
	Expect(vppMsg.IsAuthenticated).To(BeEquivalentTo(1))
}

func TestAddBfdUDPSessionAuthKeyNotFound(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "10.0.0.1",
		DestinationAddress:    "20.0.0.1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
		Authentication: &bfd.SingleHopBFD_Session_Authentication{
			KeyId:           1,
			AdvertisedKeyId: 1,
		},
	}, 1, bfdKeyIndexes)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPAdd)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.DesiredMinTx).To(BeEquivalentTo(10))
	Expect(vppMsg.RequiredMinRx).To(BeEquivalentTo(15))
	Expect(vppMsg.DetectMult).To(BeEquivalentTo(2))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAuthenticated).To(BeEquivalentTo(0))
}

func TestAddBfdUDPSessionNoAuthKey(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})
	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "10.0.0.1",
		DestinationAddress:    "20.0.0.1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
	}, 1, nil)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPAdd)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.DesiredMinTx).To(BeEquivalentTo(10))
	Expect(vppMsg.RequiredMinRx).To(BeEquivalentTo(15))
	Expect(vppMsg.DetectMult).To(BeEquivalentTo(2))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAuthenticated).To(BeEquivalentTo(0))
}

func TestAddBfdUDPSessionIncorrectSrcIPError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)
	bfdKeyIndexes.RegisterName(ifplugin.AuthKeyIdentifier(1), 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "incorrect-ip",
		DestinationAddress:    "20.0.0.1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
		Authentication: &bfd.SingleHopBFD_Session_Authentication{
			KeyId:           1,
			AdvertisedKeyId: 1,
		},
	}, 1, bfdKeyIndexes)

	Expect(err).ToNot(BeNil())
}

func TestAddBfdUDPSessionIncorrectDstIPError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)
	bfdKeyIndexes.RegisterName(string(1), 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "10.0.0.1",
		DestinationAddress:    "incorrect-ip",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
		Authentication: &bfd.SingleHopBFD_Session_Authentication{
			KeyId:           1,
			AdvertisedKeyId: 1,
		},
	}, 1, bfdKeyIndexes)

	Expect(err).ToNot(BeNil())
}

func TestAddBfdUDPSessionIPVerError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)
	bfdKeyIndexes.RegisterName(string(1), 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "10.0.0.1",
		DestinationAddress:    "2001:db8:0:1:1:1:1:1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
		Authentication: &bfd.SingleHopBFD_Session_Authentication{
			KeyId:           1,
			AdvertisedKeyId: 1,
		},
	}, 1, bfdKeyIndexes)

	Expect(err).ToNot(BeNil())
}

func TestAddBfdUDPSessionError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthSetKeyReply{})
	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "10.0.0.1",
		DestinationAddress:    "20.0.0.1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
	}, 1, nil)

	Expect(err).ToNot(BeNil())
}

func TestAddBfdUDPSessionRetvalError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{
		Retval: 1,
	})
	err := bfdHandler.AddBfdUDPSession(&bfd.SingleHopBFD_Session{
		SourceAddress:         "10.0.0.1",
		DestinationAddress:    "20.0.0.1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
	}, 1, nil)

	Expect(err).ToNot(BeNil())
}

func TestAddBfdUDPSessionFromDetails(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)
	bfdKeyIndexes.RegisterName(string(1), 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSessionFromDetails(&bfd_api.BfdUDPSessionDetails{
		SwIfIndex:       1,
		LocalAddr:       net.ParseIP("10.0.0.1"),
		PeerAddr:        net.ParseIP("20.0.0.1"),
		IsIPv6:          0,
		IsAuthenticated: 1,
		BfdKeyID:        1,
		RequiredMinRx:   15,
		DesiredMinTx:    10,
		DetectMult:      2,
	}, bfdKeyIndexes)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPAdd)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.DesiredMinTx).To(BeEquivalentTo(10))
	Expect(vppMsg.RequiredMinRx).To(BeEquivalentTo(15))
	Expect(vppMsg.DetectMult).To(BeEquivalentTo(2))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAuthenticated).To(BeEquivalentTo(1))
}

func TestAddBfdUDPSessionFromDetailsAuthKeyNotFound(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSessionFromDetails(&bfd_api.BfdUDPSessionDetails{
		SwIfIndex:       1,
		LocalAddr:       net.ParseIP("10.0.0.1"),
		PeerAddr:        net.ParseIP("20.0.0.1"),
		IsIPv6:          0,
		IsAuthenticated: 1,
		BfdKeyID:        1,
		RequiredMinRx:   15,
		DesiredMinTx:    10,
		DetectMult:      2,
	}, bfdKeyIndexes)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPAdd)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.IsAuthenticated).To(BeEquivalentTo(0))
}

func TestAddBfdUDPSessionFromDetailsNoAuth(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.AddBfdUDPSessionFromDetails(&bfd_api.BfdUDPSessionDetails{
		SwIfIndex:       1,
		LocalAddr:       net.ParseIP("10.0.0.1"),
		PeerAddr:        net.ParseIP("20.0.0.1"),
		IsIPv6:          0,
		IsAuthenticated: 0,
		BfdKeyID:        1,
		RequiredMinRx:   15,
		DesiredMinTx:    10,
		DetectMult:      2,
	}, bfdKeyIndexes)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPAdd)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.IsAuthenticated).To(BeEquivalentTo(0))
}

func TestAddBfdUDPSessionFromDetailsError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthSetKeyReply{})

	err := bfdHandler.AddBfdUDPSessionFromDetails(&bfd_api.BfdUDPSessionDetails{
		SwIfIndex: 1,
		LocalAddr: net.ParseIP("10.0.0.1"),
		PeerAddr:  net.ParseIP("20.0.0.1"),
		IsIPv6:    0,
	}, bfdKeyIndexes)

	Expect(err).ToNot(BeNil())
}

func TestAddBfdUDPSessionFromDetailsRetval(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	bfdKeyIndexes := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bfd", nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{
		Retval: 1,
	})

	err := bfdHandler.AddBfdUDPSessionFromDetails(&bfd_api.BfdUDPSessionDetails{
		SwIfIndex: 1,
		LocalAddr: net.ParseIP("10.0.0.1"),
		PeerAddr:  net.ParseIP("20.0.0.1"),
		IsIPv6:    0,
	}, bfdKeyIndexes)

	Expect(err).ToNot(BeNil())
}

func TestModifyBfdUDPSession(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPModReply{})

	err := bfdHandler.ModifyBfdUDPSession(&bfd.SingleHopBFD_Session{
		Interface:             "if1",
		SourceAddress:         "10.0.0.1",
		DestinationAddress:    "20.0.0.1",
		DesiredMinTxInterval:  10,
		RequiredMinRxInterval: 15,
		DetectMultiplier:      2,
	}, ifIndexes)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPMod)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.DesiredMinTx).To(BeEquivalentTo(10))
	Expect(vppMsg.RequiredMinRx).To(BeEquivalentTo(15))
	Expect(vppMsg.DetectMult).To(BeEquivalentTo(2))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(0))
}

func TestModifyBfdUDPSessionIPv6(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPModReply{})

	err := bfdHandler.ModifyBfdUDPSession(&bfd.SingleHopBFD_Session{
		Interface:          "if1",
		SourceAddress:      "2001:db8::1",
		DestinationAddress: "2001:db8:0:1:1:1:1:1",
	}, ifIndexes)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPMod)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(1))
}

func TestModifyBfdUDPSessionDifferentIPVer(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPModReply{})

	err := bfdHandler.ModifyBfdUDPSession(&bfd.SingleHopBFD_Session{
		Interface:          "if1",
		SourceAddress:      "10.0.0.1",
		DestinationAddress: "2001:db8:0:1:1:1:1:1",
	}, ifIndexes)

	Expect(err).ToNot(BeNil())
}

func TestModifyBfdUDPSessionNoInterface(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPModReply{})

	err := bfdHandler.ModifyBfdUDPSession(&bfd.SingleHopBFD_Session{
		Interface:          "if1",
		SourceAddress:      "10.0.0.1",
		DestinationAddress: "20.0.0.1",
	}, ifIndexes)

	Expect(err).ToNot(BeNil())
}

func TestModifyBfdUDPSessionInvalidSrcIP(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPModReply{})

	err := bfdHandler.ModifyBfdUDPSession(&bfd.SingleHopBFD_Session{
		Interface:          "if1",
		SourceAddress:      "invalid-ip",
		DestinationAddress: "20.0.0.1",
	}, ifIndexes)

	Expect(err).ToNot(BeNil())
}

func TestModifyBfdUDPSessionInvalidDstIP(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPModReply{})

	err := bfdHandler.ModifyBfdUDPSession(&bfd.SingleHopBFD_Session{
		Interface:          "if1",
		SourceAddress:      "10.0.0.1",
		DestinationAddress: "invalid-ip",
	}, ifIndexes)

	Expect(err).ToNot(BeNil())
}

func TestModifyBfdUDPSessionError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPAddReply{})

	err := bfdHandler.ModifyBfdUDPSession(&bfd.SingleHopBFD_Session{
		Interface:          "if1",
		SourceAddress:      "10.0.0.1",
		DestinationAddress: "20.0.0.1",
	}, ifIndexes)

	Expect(err).ToNot(BeNil())
}

func TestModifyBfdUDPSessionRetval(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPModReply{
		Retval: 1,
	})

	err := bfdHandler.ModifyBfdUDPSession(&bfd.SingleHopBFD_Session{
		Interface:          "if1",
		SourceAddress:      "10.0.0.1",
		DestinationAddress: "20.0.0.1",
	}, ifIndexes)

	Expect(err).ToNot(BeNil())
}

func TestDeleteBfdUDPSession(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPDelReply{})

	err := bfdHandler.DeleteBfdUDPSession(1, "10.0.0.1", "20.0.0.1")

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPDel)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.LocalAddr).To(BeEquivalentTo(net.ParseIP("10.0.0.1").To4()))
	Expect(vppMsg.PeerAddr).To(BeEquivalentTo(net.ParseIP("20.0.0.1").To4()))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(0))
}

func TestDeleteBfdUDPSessionError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPModReply{})

	err := bfdHandler.DeleteBfdUDPSession(1, "10.0.0.1", "20.0.0.1")

	Expect(err).ToNot(BeNil())
}

func TestDeleteBfdUDPSessionRetval(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPDelReply{
		Retval: 1,
	})

	err := bfdHandler.DeleteBfdUDPSession(1, "10.0.0.1", "20.0.0.1")

	Expect(err).ToNot(BeNil())
}

func TestSetBfdUDPAuthenticationKeySha1(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthSetKeyReply{})

	err := bfdHandler.SetBfdUDPAuthenticationKey(&bfd.SingleHopBFD_Key{
		Name:               "bfd-key",
		AuthKeyIndex:       1,
		Id:                 1,
		AuthenticationType: bfd.SingleHopBFD_Key_KEYED_SHA1,
		Secret:             "secret",
	})

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdAuthSetKey)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.ConfKeyID).To(BeEquivalentTo(1))
	Expect(vppMsg.KeyLen).To(BeEquivalentTo(len("secret")))
	Expect(vppMsg.AuthType).To(BeEquivalentTo(4)) // Keyed SHA1
	Expect(vppMsg.Key).To(BeEquivalentTo([]byte("secret")))
}

func TestSetBfdUDPAuthenticationKeyMeticulous(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthSetKeyReply{})

	err := bfdHandler.SetBfdUDPAuthenticationKey(&bfd.SingleHopBFD_Key{
		Name:               "bfd-key",
		AuthKeyIndex:       1,
		Id:                 1,
		AuthenticationType: bfd.SingleHopBFD_Key_METICULOUS_KEYED_SHA1,
		Secret:             "secret",
	})

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdAuthSetKey)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.ConfKeyID).To(BeEquivalentTo(1))
	Expect(vppMsg.KeyLen).To(BeEquivalentTo(len("secret")))
	Expect(vppMsg.AuthType).To(BeEquivalentTo(5)) // METICULOUS
	Expect(vppMsg.Key).To(BeEquivalentTo([]byte("secret")))
}

func TestSetBfdUDPAuthenticationKeyUnknown(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthSetKeyReply{})

	err := bfdHandler.SetBfdUDPAuthenticationKey(&bfd.SingleHopBFD_Key{
		Name:               "bfd-key",
		AuthKeyIndex:       1,
		Id:                 1,
		AuthenticationType: 2, // Unknown type
		Secret:             "secret",
	})

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdAuthSetKey)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.ConfKeyID).To(BeEquivalentTo(1))
	Expect(vppMsg.KeyLen).To(BeEquivalentTo(len("secret")))
	Expect(vppMsg.AuthType).To(BeEquivalentTo(4)) // Keyed SHA1 as default
	Expect(vppMsg.Key).To(BeEquivalentTo([]byte("secret")))
}

func TestSetBfdUDPAuthenticationError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthDelKeyReply{})

	err := bfdHandler.SetBfdUDPAuthenticationKey(&bfd.SingleHopBFD_Key{
		Name:               "bfd-key",
		AuthKeyIndex:       1,
		Id:                 1,
		AuthenticationType: bfd.SingleHopBFD_Key_KEYED_SHA1,
		Secret:             "secret",
	})

	Expect(err).ToNot(BeNil())
}

func TestSetBfdUDPAuthenticationRetval(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthSetKeyReply{
		Retval: 1,
	})

	err := bfdHandler.SetBfdUDPAuthenticationKey(&bfd.SingleHopBFD_Key{
		Name:               "bfd-key",
		AuthKeyIndex:       1,
		Id:                 1,
		AuthenticationType: bfd.SingleHopBFD_Key_KEYED_SHA1,
		Secret:             "secret",
	})

	Expect(err).ToNot(BeNil())
}

func TestDeleteBfdUDPAuthenticationKey(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthDelKeyReply{})

	err := bfdHandler.DeleteBfdUDPAuthenticationKey(&bfd.SingleHopBFD_Key{
		Name:               "bfd-key",
		AuthKeyIndex:       1,
		Id:                 1,
		AuthenticationType: bfd.SingleHopBFD_Key_KEYED_SHA1,
		Secret:             "secret",
	})

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdAuthDelKey)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.ConfKeyID).To(BeEquivalentTo(1))
}

func TestDeleteBfdUDPAuthenticationKeyError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthSetKeyReply{})

	err := bfdHandler.DeleteBfdUDPAuthenticationKey(&bfd.SingleHopBFD_Key{
		Name:               "bfd-key",
		AuthKeyIndex:       1,
		Id:                 1,
		AuthenticationType: bfd.SingleHopBFD_Key_KEYED_SHA1,
		Secret:             "secret",
	})

	Expect(err).ToNot(BeNil())
}

func TestDeleteBfdUDPAuthenticationKeyRetval(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdAuthDelKeyReply{
		Retval: 1,
	})

	err := bfdHandler.DeleteBfdUDPAuthenticationKey(&bfd.SingleHopBFD_Key{
		Name:               "bfd-key",
		AuthKeyIndex:       1,
		Id:                 1,
		AuthenticationType: bfd.SingleHopBFD_Key_KEYED_SHA1,
		Secret:             "secret",
	})

	Expect(err).ToNot(BeNil())
}

func TestAddBfdEchoFunction(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPSetEchoSourceReply{})

	err := bfdHandler.AddBfdEchoFunction(&bfd.SingleHopBFD_EchoFunction{
		Name:                "echo",
		EchoSourceInterface: "if1",
	}, ifIndexes)
	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPSetEchoSource)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
}

func TestAddBfdEchoFunctionInterfaceNotFound(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPSetEchoSourceReply{})

	err := bfdHandler.AddBfdEchoFunction(&bfd.SingleHopBFD_EchoFunction{
		Name:                "echo",
		EchoSourceInterface: "if1",
	}, ifIndexes)
	Expect(err).ToNot(BeNil())
}

func TestAddBfdEchoFunctionError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPDelEchoSourceReply{})

	err := bfdHandler.AddBfdEchoFunction(&bfd.SingleHopBFD_EchoFunction{
		Name:                "echo",
		EchoSourceInterface: "if1",
	}, ifIndexes)
	Expect(err).ToNot(BeNil())
}

func TestAddBfdEchoFunctionRetval(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "if", nil))
	ifIndexes.RegisterName("if1", 1, nil)

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPSetEchoSourceReply{
		Retval: 1,
	})

	err := bfdHandler.AddBfdEchoFunction(&bfd.SingleHopBFD_EchoFunction{
		Name:                "echo",
		EchoSourceInterface: "if1",
	}, ifIndexes)
	Expect(err).ToNot(BeNil())
}

func TestDeleteBfdEchoFunction(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPDelEchoSourceReply{})

	err := bfdHandler.DeleteBfdEchoFunction()
	Expect(err).To(BeNil())
	_, ok := ctx.MockChannel.Msg.(*bfd_api.BfdUDPDelEchoSource)
	Expect(ok).To(BeTrue())
}

func TestDeleteBfdEchoFunctionError(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPSetEchoSourceReply{})

	err := bfdHandler.DeleteBfdEchoFunction()
	Expect(err).ToNot(BeNil())
}

func TestDeleteBfdEchoFunctionRetval(t *testing.T) {
	ctx, bfdHandler, _ := bfdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bfd_api.BfdUDPDelEchoSourceReply{
		Retval: 1,
	})

	err := bfdHandler.DeleteBfdEchoFunction()
	Expect(err).ToNot(BeNil())
}

func bfdTestSetup(t *testing.T) (*vppcallmock.TestCtx, vppcalls.BfdVppAPI, ifaceidx.SwIfIndexRW) {
	ctx := vppcallmock.SetupTestCtx(t)
	log := logrus.NewLogger("test-log")
	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "bfd-if-idx", nil))
	bfdHandler := vppcalls.NewBfdVppHandler(ctx.MockChannel, ifIndexes, log,)
	return ctx, bfdHandler, ifIndexes
}
