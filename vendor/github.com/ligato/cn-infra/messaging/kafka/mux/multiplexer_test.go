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

	"github.com/Shopify/sarama"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/examples/model"
	"github.com/ligato/cn-infra/messaging"
	"github.com/ligato/cn-infra/messaging/kafka/client"
	"github.com/onsi/gomega"
)

func TestMultiplexer(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewBytesConnection("c1")
	gomega.Expect(c1).NotTo(gomega.BeNil())
	c2 := mock.Mux.NewBytesConnection("c2")
	gomega.Expect(c2).NotTo(gomega.BeNil())

	ch1 := make(chan *client.ConsumerMessage)
	ch2 := make(chan *client.ConsumerMessage)

	err := c1.ConsumeTopic(ToBytesMsgChan(ch1), "topic1")
	gomega.Expect(err).To(gomega.BeNil())
	err = c2.ConsumeTopic(ToBytesMsgChan(ch2), "topic2", "topic3")
	gomega.Expect(err).To(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	// once the multiplexer is start an attempt to subscribe returns an error
	err = c1.ConsumeTopic(ToBytesMsgChan(ch1), "anotherTopic1")
	gomega.Expect(err).NotTo(gomega.BeNil())

	mock.Mux.Close()
	close(ch1)
	close(ch2)

}

func TestMultiplexerProto(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewProtoConnection("c1", &keyval.SerializerJSON{})
	gomega.Expect(c1).NotTo(gomega.BeNil())
	c2 := mock.Mux.NewProtoConnection("c2", &keyval.SerializerJSON{})
	gomega.Expect(c2).NotTo(gomega.BeNil())

	ch1 := make(chan messaging.ProtoMessage)
	ch2 := make(chan messaging.ProtoMessage)

	err := c1.ConsumeTopic(messaging.ToProtoMsgChan(ch1), "topic1")
	gomega.Expect(err).To(gomega.BeNil())
	err = c2.ConsumeTopic(messaging.ToProtoMsgChan(ch2), "topic2", "topic3")
	gomega.Expect(err).To(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	// once the multiplexer is start an attempt to subscribe returns an error
	err = c1.ConsumeTopic(messaging.ToProtoMsgChan(ch1), "anotherTopic1")
	gomega.Expect(err).NotTo(gomega.BeNil())

	mock.Mux.Close()
	close(ch1)
	close(ch2)

}

func TestStopConsuming(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewBytesConnection("c1")
	gomega.Expect(c1).NotTo(gomega.BeNil())
	c2 := mock.Mux.NewBytesConnection("c2")
	gomega.Expect(c2).NotTo(gomega.BeNil())

	ch1 := make(chan *client.ConsumerMessage)
	ch2 := make(chan *client.ConsumerMessage)

	err := c1.ConsumeTopic(ToBytesMsgChan(ch1), "topic1")
	gomega.Expect(err).To(gomega.BeNil())
	err = c2.ConsumeTopic(ToBytesMsgChan(ch2), "topic2", "topic3")
	gomega.Expect(err).To(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	err = c1.StopConsuming("topic1")
	gomega.Expect(err).To(gomega.BeNil())

	// topic is not consumed
	err = c1.StopConsuming("Unknown topic")
	gomega.Expect(err).NotTo(gomega.BeNil())

	// topic consumed by a different connection
	err = c1.StopConsuming("topic2")
	gomega.Expect(err).NotTo(gomega.BeNil())

	mock.Mux.Close()
	close(ch1)
	close(ch2)

}

func TestStopConsumingProto(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewProtoConnection("c1", &keyval.SerializerJSON{})
	gomega.Expect(c1).NotTo(gomega.BeNil())
	c2 := mock.Mux.NewProtoConnection("c2", &keyval.SerializerJSON{})
	gomega.Expect(c2).NotTo(gomega.BeNil())

	ch1 := make(chan messaging.ProtoMessage)
	ch2 := make(chan messaging.ProtoMessage)

	err := c1.ConsumeTopic(messaging.ToProtoMsgChan(ch1), "topic1")
	gomega.Expect(err).To(gomega.BeNil())
	err = c2.ConsumeTopic(messaging.ToProtoMsgChan(ch2), "topic2", "topic3")
	gomega.Expect(err).To(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	err = c1.StopConsuming("topic1")
	gomega.Expect(err).To(gomega.BeNil())

	// topic is not consumed
	err = c1.StopConsuming("Unknown topic")
	gomega.Expect(err).NotTo(gomega.BeNil())

	// topic consumed by a different connection
	err = c1.StopConsuming("topic2")
	gomega.Expect(err).NotTo(gomega.BeNil())

	mock.Mux.Close()
	close(ch1)
	close(ch2)

}

func TestStopConsumingPartition(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewBytesManualConnection("c1")
	gomega.Expect(c1).NotTo(gomega.BeNil())
	c2 := mock.Mux.NewBytesManualConnection("c2")
	gomega.Expect(c2).NotTo(gomega.BeNil())

	ch1 := make(chan *client.ConsumerMessage)
	ch2 := make(chan *client.ConsumerMessage)

	err := c1.ConsumePartition(ToBytesMsgChan(ch1), "topic1", 1, 0)
	gomega.Expect(err).To(gomega.BeNil())
	err = c2.ConsumePartition(ToBytesMsgChan(ch2), "topic2", 2, 1)
	gomega.Expect(err).To(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	err = c1.StopConsumingPartition("topic1", 1, 0)
	gomega.Expect(err).To(gomega.BeNil())

	// topic is not consumed
	err = c1.StopConsumingPartition("Unknown topic", 1, 0)
	gomega.Expect(err).NotTo(gomega.BeNil())

	// partition is not consumed by topic
	err = c1.StopConsumingPartition("topic1", 2, 0)
	gomega.Expect(err).NotTo(gomega.BeNil())

	// offset is not consumed by topic/partition pair
	err = c1.StopConsumingPartition("topic1", 1, 1)
	gomega.Expect(err).NotTo(gomega.BeNil())

	// topic consumed by a different connection
	err = c1.StopConsumingPartition("topic2", 2, 1)
	gomega.Expect(err).NotTo(gomega.BeNil())

	mock.Mux.Close()
	close(ch1)
	close(ch2)
}

func TestStopConsumingPartitionProto(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewProtoManualConnection("c1", &keyval.SerializerJSON{})
	gomega.Expect(c1).NotTo(gomega.BeNil())
	c2 := mock.Mux.NewProtoManualConnection("c2", &keyval.SerializerJSON{})
	gomega.Expect(c2).NotTo(gomega.BeNil())

	ch1 := make(chan messaging.ProtoMessage)
	ch2 := make(chan messaging.ProtoMessage)

	err := c1.ConsumePartition(messaging.ToProtoMsgChan(ch1), "topic1", 1, 0)
	gomega.Expect(err).To(gomega.BeNil())
	err = c2.ConsumePartition(messaging.ToProtoMsgChan(ch2), "topic2", 2, 1)
	gomega.Expect(err).To(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	err = c1.StopConsumingPartition("topic1", 1, 0)
	gomega.Expect(err).To(gomega.BeNil())

	// topic is not consumed
	err = c1.StopConsumingPartition("Unknown topic", 1, 0)
	gomega.Expect(err).NotTo(gomega.BeNil())

	// partition is not consumed by topic
	err = c1.StopConsumingPartition("topic1", 2, 0)
	gomega.Expect(err).NotTo(gomega.BeNil())

	// offset is not consumed by topic/partition pair
	err = c1.StopConsumingPartition("topic1", 1, 1)
	gomega.Expect(err).NotTo(gomega.BeNil())

	// topic consumed by a different connection
	err = c1.StopConsumingPartition("topic2", 2, 1)
	gomega.Expect(err).NotTo(gomega.BeNil())

	mock.Mux.Close()
	close(ch1)
	close(ch2)
}

func TestSendSync(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewBytesConnection("c1")
	gomega.Expect(c1).NotTo(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	mock.SyncPub.ExpectSendMessageAndSucceed()
	_, err := c1.SendSyncByte("topic", []byte("key"), []byte("value"))
	gomega.Expect(err).To(gomega.BeNil())

	mock.SyncPub.ExpectSendMessageAndSucceed()
	_, err = c1.SendSyncString("topic", "key", "value")
	gomega.Expect(err).To(gomega.BeNil())

	mock.SyncPub.ExpectSendMessageAndSucceed()
	_, err = c1.SendSyncMessage("topic", sarama.ByteEncoder([]byte("key")), sarama.ByteEncoder([]byte("value")))
	gomega.Expect(err).To(gomega.BeNil())

	publisher, err := c1.NewSyncPublisher("test")
	gomega.Expect(err).To(gomega.BeNil())
	mock.SyncPub.ExpectSendMessageAndSucceed()
	publisher.Put("key", []byte("val"))

	mock.Mux.Close()
}

func TestSendProtoSync(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewProtoConnection("c1", &keyval.SerializerJSON{})
	gomega.Expect(c1).NotTo(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	enc := &etcdexample.EtcdExample{
		StringVal: "sync-message",
	}

	mock.SyncPub.ExpectSendMessageAndSucceed()
	_, err := c1.sendSyncMessage("topic", DefPartition, "key", enc, false)
	gomega.Expect(err).To(gomega.BeNil())

	publisher, err := c1.NewSyncPublisher("test")
	gomega.Expect(err).To(gomega.BeNil())
	mock.SyncPub.ExpectSendMessageAndSucceed()
	publisher.Put("key", enc, nil)

	mock.Mux.Close()
}

func TestSendSyncToCustomPartition(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewBytesManualConnection("c1")
	gomega.Expect(c1).NotTo(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	mock.SyncPub.ExpectSendMessageAndSucceed()
	_, err := c1.SendSyncStringToPartition("topic", 1, "key", "value")
	gomega.Expect(err).To(gomega.BeNil())

	publisher, err := c1.NewSyncPublisherToPartition("test", 1)
	gomega.Expect(err).To(gomega.BeNil())
	mock.SyncPub.ExpectSendMessageAndSucceed()
	publisher.Put("key", []byte("val"))

	mock.Mux.Close()
}

func TestSendProtoSyncToCustomPartition(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewProtoManualConnection("c1", &keyval.SerializerJSON{})
	gomega.Expect(c1).NotTo(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	enc := &etcdexample.EtcdExample{
		StringVal: "sync-message",
	}

	mock.SyncPub.ExpectSendMessageAndSucceed()
	_, err := c1.sendSyncMessage("topic", 1, "key", enc, true)
	gomega.Expect(err).To(gomega.BeNil())

	publisher, err := c1.NewSyncPublisherToPartition("test", 1)
	gomega.Expect(err).To(gomega.BeNil())
	mock.SyncPub.ExpectSendMessageAndSucceed()
	publisher.Put("key", enc)

	mock.Mux.Close()
}

func TestSendAsync(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewBytesConnection("c1")
	gomega.Expect(c1).NotTo(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	mock.AsyncPub.ExpectInputAndSucceed()
	c1.SendAsyncByte("topic", []byte("key"), []byte("value"), nil, nil, nil)

	mock.AsyncPub.ExpectInputAndSucceed()
	c1.SendAsyncString("topic", "key", "value", nil, nil, nil)

	mock.AsyncPub.ExpectInputAndSucceed()
	c1.SendAsyncMessage("topic", sarama.ByteEncoder([]byte("key")), sarama.ByteEncoder([]byte("value")), nil, nil, nil)

	publisher, err := c1.NewAsyncPublisher("test", nil, nil)
	gomega.Expect(err).To(gomega.BeNil())
	mock.AsyncPub.ExpectInputAndSucceed()
	publisher.Put("key", []byte("val"))

	mock.Mux.Close()
}

func TestSendProtoAsync(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewProtoConnection("c1", &keyval.SerializerJSON{})
	gomega.Expect(c1).NotTo(gomega.BeNil())
	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	enc := &etcdexample.EtcdExample{
		StringVal: "sync-message",
	}

	asyncSuccessChannel := make(chan messaging.ProtoMessage, 0)
	asyncErrorChannel := make(chan messaging.ProtoMessageErr, 0)

	mock.AsyncPub.ExpectInputAndSucceed()
	err := c1.sendAsyncMessage("topic", DefPartition, "key", enc, false, nil, messaging.ToProtoMsgChan(asyncSuccessChannel),
		messaging.ToProtoMsgErrChan(asyncErrorChannel))
	gomega.Expect(err).To(gomega.BeNil())

	publisher, err := c1.NewAsyncPublisher("test", messaging.ToProtoMsgChan(asyncSuccessChannel),
		messaging.ToProtoMsgErrChan(asyncErrorChannel))
	gomega.Expect(err).To(gomega.BeNil())
	mock.AsyncPub.ExpectInputAndSucceed()
	publisher.Put("key", enc)

	mock.Mux.Close()
}

func TestSendAsyncToCustomPartition(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewBytesManualConnection("c1")
	gomega.Expect(c1).NotTo(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	mock.AsyncPub.ExpectInputAndSucceed()
	c1.SendAsyncStringToPartition("topic", 1, "key", "value", nil, nil, nil)

	mock.AsyncPub.ExpectInputAndSucceed()
	c1.SendAsyncMessageToPartition("topic", 2, sarama.ByteEncoder([]byte("key")), sarama.ByteEncoder([]byte("value")), nil, nil, nil)

	publisher, err := c1.NewAsyncPublisherToPartition("test", 1, nil, nil)
	gomega.Expect(err).To(gomega.BeNil())
	mock.AsyncPub.ExpectInputAndSucceed()
	publisher.Put("key", []byte("val"))

	mock.Mux.Close()
}

func TestSendProtoAsyncToCustomPartition(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewProtoManualConnection("c1", &keyval.SerializerJSON{})
	gomega.Expect(c1).NotTo(gomega.BeNil())

	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	enc := &etcdexample.EtcdExample{
		StringVal: "sync-message",
	}

	asyncSuccessChannel := make(chan messaging.ProtoMessage, 0)
	asyncErrorChannel := make(chan messaging.ProtoMessageErr, 0)

	mock.AsyncPub.ExpectInputAndSucceed()
	c1.sendAsyncMessage("topic", 1, "key", enc, true, nil, messaging.ToProtoMsgChan(asyncSuccessChannel),
		messaging.ToProtoMsgErrChan(asyncErrorChannel))

	mock.AsyncPub.ExpectInputAndSucceed()
	c1.sendAsyncMessage("topic", 2, "key", enc, true, nil, messaging.ToProtoMsgChan(asyncSuccessChannel),
		messaging.ToProtoMsgErrChan(asyncErrorChannel))

	publisher, err := c1.NewAsyncPublisherToPartition("test", 1, nil, nil)
	gomega.Expect(err).To(gomega.BeNil())
	mock.AsyncPub.ExpectInputAndSucceed()
	publisher.Put("key", enc, nil)

	mock.Mux.Close()
}

func TestCustomPartition(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewBytesManualConnection("c1")
	gomega.Expect(c1).NotTo(gomega.BeNil())
	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	asyncSubscription := make(chan *client.ConsumerMessage)

	err := c1.ConsumePartition(ToBytesMsgChan(asyncSubscription), "test", 1, OffsetOldest)
	gomega.Expect(err).To(gomega.BeNil())
}

func TestProtoCustomPartition(t *testing.T) {
	gomega.RegisterTestingT(t)
	mock := Mock(t)
	gomega.Expect(mock.Mux).NotTo(gomega.BeNil())

	c1 := mock.Mux.NewProtoManualConnection("c1", &keyval.SerializerJSON{})
	gomega.Expect(c1).NotTo(gomega.BeNil())
	mock.Mux.Start()
	gomega.Expect(mock.Mux.started).To(gomega.BeTrue())

	asyncSubscription := make(chan messaging.ProtoMessage)

	err := c1.WatchPartition(messaging.ToProtoMsgChan(asyncSubscription), "test", 1, OffsetOldest)
	gomega.Expect(err).To(gomega.BeNil())
}
