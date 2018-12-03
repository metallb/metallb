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

package vppcallmock

import (
	"testing"

	"git.fd.io/govpp.git/adapter/mock"
	govppapi "git.fd.io/govpp.git/api"
	govpp "git.fd.io/govpp.git/core"
	log "github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

func init() {
	govpp.SetLogLevel(logrus.DebugLevel)
}

// TestCtx is helping structure for unit testing.
// It wraps VppAdapter which is used instead of real VPP.
type TestCtx struct {
	MockVpp     *mock.VppAdapter
	conn        *govpp.Connection
	channel     govppapi.Channel
	MockChannel *mockedChannel
}

// SetupTestCtx sets up all fields of TestCtx structure at the begining of test
func SetupTestCtx(t *testing.T) *TestCtx {
	RegisterTestingT(t)

	ctx := &TestCtx{
		MockVpp: mock.NewVppAdapter(),
	}

	var err error
	ctx.conn, err = govpp.Connect(ctx.MockVpp)
	Expect(err).ShouldNot(HaveOccurred())

	ctx.channel, err = ctx.conn.NewAPIChannel()
	Expect(err).ShouldNot(HaveOccurred())

	ctx.MockChannel = &mockedChannel{Channel: ctx.channel}

	return ctx
}

// TeardownTestCtx politely close all used resources
func (ctx *TestCtx) TeardownTestCtx() {
	ctx.channel.Close()
	ctx.conn.Disconnect()
}

// MockedChannel implements ChannelIntf for testing purposes
type mockedChannel struct {
	govppapi.Channel

	// Last message which passed through method SendRequest
	Msg govppapi.Message

	// List of all messages which passed through method SendRequest
	Msgs []govppapi.Message
}

// SendRequest just save input argument to structure field for future check
func (m *mockedChannel) SendRequest(msg govppapi.Message) govppapi.RequestCtx {
	m.Msg = msg
	m.Msgs = append(m.Msgs, msg)
	return m.Channel.SendRequest(msg)
}

// SendMultiRequest just save input argument to structure field for future check
func (m *mockedChannel) SendMultiRequest(msg govppapi.Message) govppapi.MultiRequestCtx {
	m.Msg = msg
	m.Msgs = append(m.Msgs, msg)
	return m.Channel.SendMultiRequest(msg)
}

// HandleReplies represents spec for MockReplyHandler.
type HandleReplies struct {
	Name     string
	Ping     bool
	Message  govppapi.Message
	Messages []govppapi.Message
}

// MockReplies sets up reply handler for give HandleReplies.
func (ctx *TestCtx) MockReplies(dataList []*HandleReplies) {
	var sendControlPing bool

	ctx.MockVpp.MockReplyHandler(func(request mock.MessageDTO) (reply []byte, msgID uint16, prepared bool) {
		// Following types are not automatically stored in mock adapter's map and will be sent with empty MsgName
		// TODO: initialize mock adapter's map with these
		switch request.MsgID {
		case 100:
			request.MsgName = "control_ping"
		case 101:
			request.MsgName = "control_ping_reply"
		case 200:
			request.MsgName = "sw_interface_dump"
		case 201:
			request.MsgName = "sw_interface_details"
		}

		if request.MsgName == "" {
			log.DefaultLogger().Fatalf("mockHandler received request (ID: %v) with empty MsgName, check if compatbility check is done before using this request", request.MsgID)
		}

		if sendControlPing {
			sendControlPing = false
			data := &vpe.ControlPingReply{}
			reply, err := ctx.MockVpp.ReplyBytes(request, data)
			Expect(err).To(BeNil())
			msgID, err := ctx.MockVpp.GetMsgID(data.GetMessageName(), data.GetCrcString())
			Expect(err).To(BeNil())
			return reply, msgID, true
		}

		for _, dataMock := range dataList {
			if request.MsgName == dataMock.Name {
				// Send control ping next iteration if set
				sendControlPing = dataMock.Ping
				if len(dataMock.Messages) > 0 {
					log.DefaultLogger().Infof(" MOCK HANDLER: mocking %d messages", len(dataMock.Messages))
					for _, msg := range dataMock.Messages {
						ctx.MockVpp.MockReply(msg)
					}
					return nil, 0, false
				}
				msgID, err := ctx.MockVpp.GetMsgID(dataMock.Message.GetMessageName(), dataMock.Message.GetCrcString())
				Expect(err).To(BeNil())
				reply, err := ctx.MockVpp.ReplyBytes(request, dataMock.Message)
				Expect(err).To(BeNil())
				return reply, msgID, true
			}
		}

		var err error
		replyMsg, id, ok := ctx.MockVpp.ReplyFor(request.MsgName)
		if ok {
			reply, err = ctx.MockVpp.ReplyBytes(request, replyMsg)
			Expect(err).To(BeNil())
			msgID = id
			prepared = true
		} else {
			log.DefaultLogger().Warnf("NO REPLY FOR %v FOUND", request.MsgName)
		}

		return reply, msgID, prepared
	})
}
