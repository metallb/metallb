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

package codec

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"

	"git.fd.io/govpp.git/api"
	"github.com/lunixbochs/struc"
)

// MsgCodec provides encoding and decoding functionality of `api.Message` structs into/from
// binary format as accepted by VPP.
type MsgCodec struct{}

// VppRequestHeader struct contains header fields implemented by all VPP requests.
type VppRequestHeader struct {
	VlMsgID     uint16
	ClientIndex uint32
	Context     uint32
}

// VppReplyHeader struct contains header fields implemented by all VPP replies.
type VppReplyHeader struct {
	VlMsgID uint16
	Context uint32
}

// VppEventHeader struct contains header fields implemented by all VPP events.
type VppEventHeader struct {
	VlMsgID     uint16
	ClientIndex uint32
}

// VppOtherHeader struct contains header fields implemented by other VPP messages (not requests nor replies).
type VppOtherHeader struct {
	VlMsgID uint16
}

// EncodeMsg encodes provided `Message` structure into its binary-encoded data representation.
func (*MsgCodec) EncodeMsg(msg api.Message, msgID uint16) (data []byte, err error) {
	if msg == nil {
		return nil, errors.New("nil message passed in")
	}

	// try to recover panic which might possibly occur in struc.Pack call
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if err, ok = r.(error); !ok {
				err = fmt.Errorf("%v", r)
			}
			err = fmt.Errorf("panic occurred: %v", err)
		}
	}()

	var header interface{}

	// encode message header
	switch msg.GetMessageType() {
	case api.RequestMessage:
		header = &VppRequestHeader{VlMsgID: msgID}
	case api.ReplyMessage:
		header = &VppReplyHeader{VlMsgID: msgID}
	case api.EventMessage:
		header = &VppEventHeader{VlMsgID: msgID}
	default:
		header = &VppOtherHeader{VlMsgID: msgID}
	}

	buf := new(bytes.Buffer)

	// encode message header
	if err := struc.Pack(buf, header); err != nil {
		return nil, fmt.Errorf("failed to encode message header: %+v, error: %v", header, err)
	}

	// encode message content
	if reflect.TypeOf(msg).Elem().NumField() > 0 {
		if err := struc.Pack(buf, msg); err != nil {
			return nil, fmt.Errorf("failed to encode message data: %+v, error: %v", data, err)
		}
	}

	return buf.Bytes(), nil
}

// DecodeMsg decodes binary-encoded data of a message into provided `Message` structure.
func (*MsgCodec) DecodeMsg(data []byte, msg api.Message) error {
	if msg == nil {
		return errors.New("nil message passed in")
	}

	var header interface{}

	// check which header is expected
	switch msg.GetMessageType() {
	case api.RequestMessage:
		header = new(VppRequestHeader)
	case api.ReplyMessage:
		header = new(VppReplyHeader)
	case api.EventMessage:
		header = new(VppEventHeader)
	default:
		header = new(VppOtherHeader)
	}

	buf := bytes.NewReader(data)

	// decode message header
	if err := struc.Unpack(buf, header); err != nil {
		return fmt.Errorf("failed to decode message header: %+v, error: %v", header, err)
	}

	// decode message content
	if err := struc.Unpack(buf, msg); err != nil {
		return fmt.Errorf("failed to decode message data: %+v, error: %v", data, err)
	}

	return nil
}

func (*MsgCodec) DecodeMsgContext(data []byte, msg api.Message) (uint32, error) {
	if msg == nil {
		return 0, errors.New("nil message passed in")
	}

	var header interface{}
	var getContext func() uint32

	// check which header is expected
	switch msg.GetMessageType() {
	case api.RequestMessage:
		header = new(VppRequestHeader)
		getContext = func() uint32 { return header.(*VppRequestHeader).Context }

	case api.ReplyMessage:
		header = new(VppReplyHeader)
		getContext = func() uint32 { return header.(*VppReplyHeader).Context }

	default:
		return 0, nil
	}

	buf := bytes.NewReader(data)

	// decode message header
	if err := struc.Unpack(buf, header); err != nil {
		return 0, fmt.Errorf("decoding message header failed: %v", err)
	}

	return getContext(), nil
}
