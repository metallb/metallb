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
	"time"

	"github.com/bsm/sarama-cluster"
	"github.com/ligato/cn-infra/logging/logrus"
)

var log = logrus.DefaultLogger()

func ExampleConsumer() {

	//init config
	config := NewConfig(logrus.DefaultLogger())
	config.SetBrokers("localhost:9091,localhost:9092")
	config.SetRecvNotification(true)
	config.SetRecvNotificationChan(make(chan *cluster.Notification))
	config.SetRecvError(true)
	config.SetRecvErrorChan(make(chan error))
	config.SetRecvMessageChan(make(chan *ConsumerMessage))
	config.SetTopics("topic1,topic2,topic3")
	config.SetGroup("test-group")

	// init consumer with message handlers
	consumer, err := NewConsumer(config, nil)
	if err != nil {
		log.Errorf("NewConsumer Error: %v", err)
	}

	go watchChannels(consumer, config)

	// wait for consumer to finish receiving messages
	consumer.WaitForClose()
	log.Info("consumer closed")
	// do something
}

func watchChannels(consumer *Consumer, cfg *Config) {

	for {
		select {
		case notification, more := <-cfg.RecvNotificationChan:
			if more {
				handleNotification(notification)
			}
		case err, more := <-cfg.RecvErrorChan:
			if more {
				log.Errorf("Message Recv Errored: %v\n", err)
			}
		case msg, more := <-cfg.RecvMessageChan:
			if more {
				handleMessage(consumer, msg)
			}
		case <-consumer.GetCloseChannel():
			return
		}
	}
}

func handleNotification(note *cluster.Notification) {
	log.Info("Rebalanced Consumer at ", time.Now())
	log.Info("Claimed: ")
	consumerLogNotification(note.Claimed)
	log.Info("Released: ")
	consumerLogNotification(note.Released)
	log.Info("Current: ")
	consumerLogNotification(note.Current)
}

func handleMessage(consumer *Consumer, msg *ConsumerMessage) {
	log.Infof("Consumer Message - Topic: msg.Topic, Key: %s, Value: %s, Partition: %d Offset: %d\n", string(msg.Key), string(msg.Value), msg.Partition, msg.Offset)

	// mark the offset so that it will be committed
	consumer.MarkOffset(msg, "")
}

// logNotifications logs the topics and partitions
func consumerLogNotification(note map[string][]int32) {
	for k, v := range note {
		log.Infof("Topic: %s, Partitions: %v", k, v)
	}
}

func TestConsumer(t *testing.T) {
	c := GetConsumerMock(t)
	c.Close()
}
