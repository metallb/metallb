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
	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/bsm/sarama-cluster"
	"github.com/ligato/cn-infra/logging/logrus"
)

type clusterConsumerMock struct {
	notifCh           chan *cluster.Notification
	errCh             chan error
	consumer          sarama.Consumer
	partitionConsumer sarama.PartitionConsumer
}

type saramaClientMock struct {
}

// GetAsyncProducerMock returns mocked implementation of async producer that doesn't
// need connection to Kafka broker and can be used for testing purposes.
func GetAsyncProducerMock(t mocks.ErrorReporter) (*AsyncProducer, *mocks.AsyncProducer) {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Producer.Return.Successes = true
	mock := mocks.NewAsyncProducer(t, saramaCfg)

	cfg := NewConfig(logrus.DefaultLogger())
	cfg.SetSendSuccess(true)
	cfg.SetSuccessChan(make(chan *ProducerMessage, 1))
	ap := AsyncProducer{Logger: logrus.DefaultLogger(), Config: cfg, Producer: mock, closeChannel: make(chan struct{}), Client: &saramaClientMock{}}
	go ap.successHandler(mock.Successes())

	return &ap, mock
}

// GetSyncProducerMock returns mocked implementation of sync producer that doesn't need
// connection to Kafka broker and can be used for testing purposes.
func GetSyncProducerMock(t mocks.ErrorReporter) (*SyncProducer, *mocks.SyncProducer) {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Producer.Return.Successes = true
	mock := mocks.NewSyncProducer(t, saramaCfg)

	cfg := NewConfig(logrus.DefaultLogger())
	ap := SyncProducer{Logger: logrus.DefaultLogger(), Config: cfg, Producer: mock, closeChannel: make(chan struct{}), Client: &saramaClientMock{}}

	return &ap, mock
}

// GetConsumerMock returns mocked implementation of consumer that doesn't need connection
// to kafka cluster.
func GetConsumerMock(t mocks.ErrorReporter) *Consumer {
	cfg := NewConfig(logrus.DefaultLogger())
	ap := Consumer{
		Logger:       logrus.DefaultLogger(),
		Config:       cfg,
		Consumer:     newClusterConsumerMock(t),
		closeChannel: make(chan struct{}),
	}

	return &ap
}

func newClusterConsumerMock(t mocks.ErrorReporter) *clusterConsumerMock {
	cfg := sarama.NewConfig()
	mockSaramaConsumer := mocks.NewConsumer(t, cfg)
	cl := &clusterConsumerMock{
		notifCh:  make(chan *cluster.Notification),
		errCh:    make(chan error),
		consumer: mockSaramaConsumer,
	}
	mockSaramaConsumer.ExpectConsumePartition("topic", 0, sarama.OffsetOldest)

	cl.partitionConsumer, _ = cl.consumer.ConsumePartition("topic", 0, sarama.OffsetOldest)

	return cl
}

func (c *clusterConsumerMock) Notifications() <-chan *cluster.Notification {
	return c.notifCh
}

func (c *clusterConsumerMock) Errors() <-chan error {
	return c.errCh
}

func (c *clusterConsumerMock) Messages() <-chan *sarama.ConsumerMessage {
	return c.partitionConsumer.Messages()
}

func (c *clusterConsumerMock) Close() (err error) {
	close(c.notifCh)
	c.partitionConsumer.Close()
	c.consumer.Close()
	return nil
}

func (c *clusterConsumerMock) MarkOffset(msg *sarama.ConsumerMessage, metadata string) {

}

func (c *clusterConsumerMock) MarkPartitionOffset(topic string, partition int32, offset int64, metadata string) {

}

func (c *clusterConsumerMock) Subscriptions() map[string][]int32 {
	return map[string][]int32{}
}

func (c *clusterConsumerMock) CommitOffsets() error {
	return nil
}

func (cl *saramaClientMock) Config() *sarama.Config {
	return nil
}

func (cl *saramaClientMock) Brokers() []*sarama.Broker {
	return nil
}

func (cl *saramaClientMock) Topics() ([]string, error) {
	return nil, nil
}

func (cl *saramaClientMock) Partitions(topic string) ([]int32, error) {
	return nil, nil
}

func (cl *saramaClientMock) WritablePartitions(topic string) ([]int32, error) {
	return nil, nil
}

func (cl *saramaClientMock) Leader(topic string, partitionID int32) (*sarama.Broker, error) {
	return nil, nil
}

func (cl *saramaClientMock) Replicas(topic string, partitionID int32) ([]int32, error) {
	return nil, nil
}

func (cl *saramaClientMock) RefreshMetadata(topics ...string) error {
	return nil
}

func (cl *saramaClientMock) GetOffset(topic string, partitionID int32, time int64) (int64, error) {
	return 0, nil
}

func (cl *saramaClientMock) Coordinator(consumerGroup string) (*sarama.Broker, error) {
	return nil, nil
}

func (cl *saramaClientMock) RefreshCoordinator(consumerGroup string) error {
	return nil
}

func (cl *saramaClientMock) InSyncReplicas(topic string, partitionID int32) ([]int32, error) {
	return nil, nil
}

func (cl *saramaClientMock) Close() error {
	return nil
}

func (cl *saramaClientMock) Closed() bool {
	return false
}
