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

package client

import (
	"testing"

	"github.com/Shopify/sarama"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/onsi/gomega"
)

func ExampleSyncProducer() {
	// init config
	config := NewConfig(logrus.DefaultLogger())
	config.ProducerConfig().Producer.RequiredAcks = sarama.WaitForAll
	config.SetBrokers("localhost:9091", "localhost:9092")

	// init client
	sClient, err := NewClient(config, Hash)
	if err != nil {
		return
	}

	// init producer
	producer, err := NewSyncProducer(config, sClient, Hash, nil)
	if err != nil {
		log.Errorf("NewSyncProducer errored: %v\n", err)
		return
	}

	// send message
	_, err = producer.SendMsgByte("test-topic", nil, []byte("test message"))
	if err != nil {
		log.Errorf("SendMsg errored: %v", err)
	}

	// close producer and release resources
	err = producer.Close()
	if err != nil {
		log.Errorf("SyncProducer close errored: %v", err)
		return
	}
	log.Info("SyncProducer closed")
}

func TestSyncProducer(t *testing.T) {
	gomega.RegisterTestingT(t)

	const topic string = "test"
	sp, mock := GetSyncProducerMock(t)

	mock.ExpectSendMessageAndSucceed()

	msg, err := sp.SendMsgByte(topic, []byte("key"), []byte("value"))
	gomega.Expect(msg).NotTo(gomega.BeNil())
	gomega.Expect(err).To(gomega.BeNil())
}
