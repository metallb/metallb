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
	"testing"

	"github.com/Shopify/sarama/mocks"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/messaging/kafka/client"
)

func getMockConsumerFactory(t *testing.T) ConsumerFactory {
	return func(topics []string, name string) (*client.Consumer, error) {
		return client.GetConsumerMock(t), nil
	}
}

// Mock returns mock of Multiplexer that can be used for testing purposes.
func Mock(t *testing.T) *KafkaMock {
	asyncP, aMock := client.GetAsyncProducerMock(t)
	syncP, sMock := client.GetSyncProducerMock(t)
	producers := multiplexerProducers{
		syncP, syncP, asyncP, asyncP,
	}

	return &KafkaMock{
		NewMultiplexer(getMockConsumerFactory(t), producers, &client.Config{}, "name", logrus.DefaultLogger()),
		aMock, sMock}
}

// KafkaMock for the tests
type KafkaMock struct {
	Mux      *Multiplexer
	AsyncPub *mocks.AsyncProducer
	SyncPub  *mocks.SyncProducer
}
