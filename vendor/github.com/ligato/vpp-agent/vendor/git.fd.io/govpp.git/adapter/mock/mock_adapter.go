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

// Package mock is an alternative VPP adapter aimed for unit/integration testing where the
// actual communication with VPP is not demanded.
package mock

import (
	"bytes"
	"log"
	"reflect"
	"sync"

	"git.fd.io/govpp.git/adapter"
	"git.fd.io/govpp.git/adapter/mock/binapi"
	"git.fd.io/govpp.git/api"
	"git.fd.io/govpp.git/codec"
	"github.com/lunixbochs/struc"
)

type replyMode int

const (
	_                replyMode = iota
	useRepliesQueue            // use replies in the queue
	useReplyHandlers           // use reply handler
)

// VppAdapter represents a mock VPP adapter that can be used for unit/integration testing instead of the vppapiclient adapter.
type VppAdapter struct {
	callback adapter.MsgCallback

	msgIDSeq     uint16
	access       sync.RWMutex
	msgNameToIds map[string]uint16
	msgIDsToName map[uint16]string
	binAPITypes  map[string]reflect.Type

	repliesLock   sync.Mutex     // mutex for the queue
	replies       []reply        // FIFO queue of messages
	replyHandlers []ReplyHandler // callbacks that are able to calculate mock responses
	mode          replyMode      // mode in which the mock operates
}

// defaultReply is a default reply message that mock adapter returns for a request.
type defaultReply struct {
	Retval int32
}

// MessageDTO is a structure used for propagating information to ReplyHandlers.
type MessageDTO struct {
	MsgID    uint16
	MsgName  string
	ClientID uint32
	Data     []byte
}

// reply for one request (can be multipart, contain replies to previously timeouted requests, etc.)
type reply struct {
	msgs []MsgWithContext
}

// MsgWithContext encapsulates reply message with possibly sequence number and is-multipart flag.
type MsgWithContext struct {
	Msg       api.Message
	SeqNum    uint16
	Multipart bool

	/* set by mock adapter */
	hasCtx bool
}

// ReplyHandler is a type that allows to extend the behaviour of VPP mock.
// Return value ok is used to signalize that mock reply is calculated and ready to be used.
type ReplyHandler func(request MessageDTO) (reply []byte, msgID uint16, ok bool)

const (
	defaultReplyMsgID = 1 // default message ID for the reply to be sent back via callback
)

// NewVppAdapter returns a new mock adapter.
func NewVppAdapter() *VppAdapter {
	a := &VppAdapter{
		msgIDSeq:     1000,
		msgIDsToName: make(map[uint16]string),
		msgNameToIds: make(map[string]uint16),
		binAPITypes:  make(map[string]reflect.Type),
	}
	a.registerBinAPITypes()
	return a
}

// Connect emulates connecting the process to VPP.
func (a *VppAdapter) Connect() error {
	return nil
}

// Disconnect emulates disconnecting the process from VPP.
func (a *VppAdapter) Disconnect() {
	// no op
}

// GetMsgNameByID returns message name for specified message ID.
func (a *VppAdapter) GetMsgNameByID(msgID uint16) (string, bool) {
	switch msgID {
	case 100:
		return "control_ping", true
	case 101:
		return "control_ping_reply", true
	case 200:
		return "sw_interface_dump", true
	case 201:
		return "sw_interface_details", true
	}

	a.access.Lock()
	defer a.access.Unlock()
	msgName, found := a.msgIDsToName[msgID]

	return msgName, found
}

func (a *VppAdapter) registerBinAPITypes() {
	a.access.Lock()
	defer a.access.Unlock()
	for _, msg := range api.GetAllMessages() {
		a.binAPITypes[msg.GetMessageName()] = reflect.TypeOf(msg).Elem()
	}
}

// ReplyTypeFor returns reply message type for given request message name.
func (a *VppAdapter) ReplyTypeFor(requestMsgName string) (reflect.Type, uint16, bool) {
	replyName, foundName := binapi.ReplyNameFor(requestMsgName)
	if foundName {
		if reply, found := a.binAPITypes[replyName]; found {
			msgID, err := a.GetMsgID(replyName, "")
			if err == nil {
				return reply, msgID, found
			}
		}
	}

	return nil, 0, false
}

// ReplyFor returns reply message for given request message name.
func (a *VppAdapter) ReplyFor(requestMsgName string) (api.Message, uint16, bool) {
	replType, msgID, foundReplType := a.ReplyTypeFor(requestMsgName)
	if foundReplType {
		msgVal := reflect.New(replType)
		if msg, ok := msgVal.Interface().(api.Message); ok {
			log.Println("FFF ", replType, msgID, foundReplType)
			return msg, msgID, true
		}
	}

	return nil, 0, false
}

// ReplyBytes encodes the mocked reply into binary format.
func (a *VppAdapter) ReplyBytes(request MessageDTO, reply api.Message) ([]byte, error) {
	replyMsgID, err := a.GetMsgID(reply.GetMessageName(), reply.GetCrcString())
	if err != nil {
		log.Println("ReplyBytesE ", replyMsgID, " ", reply.GetMessageName(), " clientId: ", request.ClientID,
			" ", err)
		return nil, err
	}
	log.Println("ReplyBytes ", replyMsgID, " ", reply.GetMessageName(), " clientId: ", request.ClientID)

	buf := new(bytes.Buffer)
	err = struc.Pack(buf, &codec.VppReplyHeader{
		VlMsgID: replyMsgID,
		Context: request.ClientID,
	})
	if err != nil {
		return nil, err
	}
	if err = struc.Pack(buf, reply); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GetMsgID returns mocked message ID for the given message name and CRC.
func (a *VppAdapter) GetMsgID(msgName string, msgCrc string) (uint16, error) {
	switch msgName {
	case "control_ping":
		return 100, nil
	case "control_ping_reply":
		return 101, nil
	case "sw_interface_dump":
		return 200, nil
	case "sw_interface_details":
		return 201, nil
	}

	a.access.Lock()
	defer a.access.Unlock()

	msgID, found := a.msgNameToIds[msgName]
	if found {
		return msgID, nil
	}

	a.msgIDSeq++
	msgID = a.msgIDSeq
	a.msgNameToIds[msgName] = msgID
	a.msgIDsToName[msgID] = msgName

	return msgID, nil
}

// SendMsg emulates sending a binary-encoded message to VPP.
func (a *VppAdapter) SendMsg(clientID uint32, data []byte) error {
	switch a.mode {
	case useReplyHandlers:
		for i := len(a.replyHandlers) - 1; i >= 0; i-- {
			replyHandler := a.replyHandlers[i]

			buf := bytes.NewReader(data)
			reqHeader := codec.VppRequestHeader{}
			struc.Unpack(buf, &reqHeader)

			a.access.Lock()
			reqMsgName := a.msgIDsToName[reqHeader.VlMsgID]
			a.access.Unlock()

			reply, msgID, finished := replyHandler(MessageDTO{
				MsgID:    reqHeader.VlMsgID,
				MsgName:  reqMsgName,
				ClientID: clientID,
				Data:     data,
			})
			if finished {
				a.callback(msgID, reply)
				return nil
			}
		}
		fallthrough

	case useRepliesQueue:
		a.repliesLock.Lock()
		defer a.repliesLock.Unlock()

		// pop the first reply
		if len(a.replies) > 0 {
			reply := a.replies[0]
			for _, msg := range reply.msgs {
				msgID, _ := a.GetMsgID(msg.Msg.GetMessageName(), msg.Msg.GetCrcString())
				buf := new(bytes.Buffer)
				context := clientID
				if msg.hasCtx {
					context = setMultipart(context, msg.Multipart)
					context = setSeqNum(context, msg.SeqNum)
				}
				if msg.Msg.GetMessageType() == api.ReplyMessage {
					struc.Pack(buf, &codec.VppReplyHeader{VlMsgID: msgID, Context: context})
				} else if msg.Msg.GetMessageType() == api.RequestMessage {
					struc.Pack(buf, &codec.VppRequestHeader{VlMsgID: msgID, Context: context})
				} else if msg.Msg.GetMessageType() == api.EventMessage {
					struc.Pack(buf, &codec.VppEventHeader{VlMsgID: msgID})
				} else {
					struc.Pack(buf, &codec.VppOtherHeader{VlMsgID: msgID})
				}
				struc.Pack(buf, msg.Msg)
				a.callback(msgID, buf.Bytes())
			}

			a.replies = a.replies[1:]
			if len(a.replies) == 0 && len(a.replyHandlers) > 0 {
				// Switch back to handlers once the queue is empty to revert back
				// the fallthrough effect.
				a.mode = useReplyHandlers
			}
			return nil
		}

		//fallthrough
	default:
		// return default reply
		buf := new(bytes.Buffer)
		msgID := uint16(defaultReplyMsgID)
		struc.Pack(buf, &codec.VppReplyHeader{VlMsgID: msgID, Context: clientID})
		struc.Pack(buf, &defaultReply{})
		a.callback(msgID, buf.Bytes())
	}
	return nil
}

// SetMsgCallback sets a callback function that will be called by the adapter whenever a message comes from the mock.
func (a *VppAdapter) SetMsgCallback(cb adapter.MsgCallback) {
	a.callback = cb
}

// WaitReady mocks waiting for VPP
func (a *VppAdapter) WaitReady() error {
	return nil
}

// MockReply stores a message or a list of multipart messages to be returned when
// the next request comes. It is a FIFO queue - multiple replies can be pushed into it,
// the first message or the first set of multi-part messages will be popped when
// some request comes.
// Using of this method automatically switches the mock into the useRepliesQueue mode.
//
// Note: multipart requests are implemented using two requests actually - the multipart
// request itself followed by control ping used to tell which multipart message
// is the last one. A mock reply to a multipart request has to thus consist of
// exactly two calls of this method.
// For example:
//
//    mockVpp.MockReply(  // push multipart messages all at once
// 			&interfaces.SwInterfaceDetails{SwIfIndex:1},
// 			&interfaces.SwInterfaceDetails{SwIfIndex:2},
// 			&interfaces.SwInterfaceDetails{SwIfIndex:3},
//    )
//    mockVpp.MockReply(&vpe.ControlPingReply{})
//
// Even if the multipart request has no replies, MockReply has to be called twice:
//
//    mockVpp.MockReply()  // zero multipart messages
//    mockVpp.MockReply(&vpe.ControlPingReply{})
func (a *VppAdapter) MockReply(msgs ...api.Message) {
	a.repliesLock.Lock()
	defer a.repliesLock.Unlock()

	r := reply{}
	for _, msg := range msgs {
		r.msgs = append(r.msgs, MsgWithContext{Msg: msg, hasCtx: false})
	}
	a.replies = append(a.replies, r)
	a.mode = useRepliesQueue
}

// MockReplyWithContext queues next reply like MockReply() does, except that the
// sequence number and multipart flag (= context minus channel ID) can be customized
// and not necessarily match with the request.
// The purpose of this function is to test handling of sequence numbers and as such
// it is not really meant to be used outside the govpp UTs.
func (a *VppAdapter) MockReplyWithContext(msgs ...MsgWithContext) {
	a.repliesLock.Lock()
	defer a.repliesLock.Unlock()

	r := reply{}
	for _, msg := range msgs {
		r.msgs = append(r.msgs,
			MsgWithContext{Msg: msg.Msg, SeqNum: msg.SeqNum, Multipart: msg.Multipart, hasCtx: true})
	}
	a.replies = append(a.replies, r)
	a.mode = useRepliesQueue
}

// MockReplyHandler registers a handler function that is supposed to generate mock responses to incoming requests.
// Using of this method automatically switches the mock into th useReplyHandlers mode.
func (a *VppAdapter) MockReplyHandler(replyHandler ReplyHandler) {
	a.repliesLock.Lock()
	defer a.repliesLock.Unlock()

	a.replyHandlers = append(a.replyHandlers, replyHandler)
	a.mode = useReplyHandlers
}

func setSeqNum(context uint32, seqNum uint16) (newContext uint32) {
	context &= 0xffff0000
	context |= uint32(seqNum)
	return context
}

func setMultipart(context uint32, isMultipart bool) (newContext uint32) {
	context &= 0xfffeffff
	if isMultipart {
		context |= 1 << 16
	}
	return context
}
