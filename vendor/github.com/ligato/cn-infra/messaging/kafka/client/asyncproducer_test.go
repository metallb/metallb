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
	"sync"
	"testing"
	"time"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/onsi/gomega"
)

func ExampleAsyncProducer() {
	log := logrus.DefaultLogger()

	//init config
	config := NewConfig(logrus.DefaultLogger())
	config.SetBrokers("localhost:9091", "localhost:9092")
	config.SetSendSuccess(true)
	config.SetSuccessChan(make(chan *ProducerMessage))
	config.SetSendError(true)
	config.SetErrorChan(make(chan *ProducerError))

	// init client
	sClient, err := NewClient(config, Hash)
	if err != nil {
		return
	}

	// init producer
	producer, err := NewAsyncProducer(config, sClient, Hash, nil)
	if err != nil {
		log.Errorf("NewAsyncProducer errored: %v\n", err)
		return
	}

	// send a message
	producer.SendMsgByte("test-topic", []byte("key"), []byte("test message"), nil)

	select {
	case msg := <-config.SuccessChan:
		log.Info("message sent successfully - ", msg)
	case err := <-config.ErrorChan:
		log.Error("message errored - ", err)
	}

	// close producer and release resources
	err = producer.Close(true)
	if err != nil {
		log.Errorf("AsyncProducer close errored: %v\n", err)
		return
	}
	log.Info("AsyncProducer closed")
}

func TestAsyncProducer(t *testing.T) {
	gomega.RegisterTestingT(t)

	const topic string = "test"
	ap, mock := GetAsyncProducerMock(t)

	mock.ExpectInputAndSucceed()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		select {
		case msg := <-ap.Config.SuccessChan:
			gomega.Expect(msg.Topic).To(gomega.BeEquivalentTo(topic))
		case <-time.After(1 * time.Second):
			t.Fail()
		}
		wg.Done()
	}()
	ap.SendMsgByte(topic, []byte("key"), []byte("value"), nil)
	wg.Wait()
}
