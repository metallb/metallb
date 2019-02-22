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

package utils

import (
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

// RecordedProtoMessage is a proto.Message suitable for recording and access via
// REST API.
type RecordedProtoMessage struct {
	proto.Message
}

// MarshalJSON marshalls proto message using the marshaller from jsonpb.
// The jsonpb package produces a different output than the standard "encoding/json"
// package, which does not operate correctly on protocol buffers.
func (p *RecordedProtoMessage) MarshalJSON() ([]byte, error) {
	marshaller := &jsonpb.Marshaler{}
	str, err := marshaller.MarshalToString(p.Message)
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}

// RecordProtoMessage prepares proto message for recording and potential
// access via REST API.
// Note: no need to clone the message - once un-marshalled, the content is never
// changed (otherwise it would break prev-new value comparisons).
func RecordProtoMessage(msg proto.Message) proto.Message {
	if msg == nil {
		return nil
	}
	return &RecordedProtoMessage{Message: msg}
}
