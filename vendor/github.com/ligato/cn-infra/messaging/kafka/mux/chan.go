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

package mux

import (
	"github.com/ligato/cn-infra/messaging"
	"github.com/ligato/cn-infra/messaging/kafka/client"
	"time"
)

// ToBytesMsgChan allows to receive ConsumerMessage through channel. This function can be used as an argument for
// ConsumeTopic call.
func ToBytesMsgChan(ch chan *client.ConsumerMessage, opts ...interface{}) func(*client.ConsumerMessage) {

	timeout, logger := messaging.ParseOpts(opts...)

	return func(msg *client.ConsumerMessage) {
		select {
		case ch <- msg:
		case <-time.After(timeout):
			logger.Warn("Unable to deliver message")
		}
	}
}

// ToBytesProducerChan allows to receive ProducerMessage through channel. This function can be used as an argument for
// methods publishing using async API.
func ToBytesProducerChan(ch chan *client.ProducerMessage, opts ...interface{}) func(*client.ProducerMessage) {

	timeout, logger := messaging.ParseOpts(opts...)

	return func(msg *client.ProducerMessage) {
		select {
		case ch <- msg:
		case <-time.After(timeout):
			logger.Warn("Unable to deliver message")
		}
	}
}

// ToBytesProducerErrChan allows to receive ProducerMessage through channel. This function can be used as an argument for
// methods publishing using async API.
func ToBytesProducerErrChan(ch chan *client.ProducerError, opts ...interface{}) func(*client.ProducerError) {

	timeout, logger := messaging.ParseOpts(opts...)

	return func(msg *client.ProducerError) {
		select {
		case ch <- msg:
		case <-time.After(timeout):
			logger.Warn("Unable to deliver message")
		}
	}
}
