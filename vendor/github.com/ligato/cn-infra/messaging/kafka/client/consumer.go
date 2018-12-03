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
	"fmt"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	"github.com/ligato/cn-infra/logging"
)

// clusterConsumer defines an interface that allows to mock the implementation of
// bsm/sarama-cluster consumer.
type clusterConsumer interface {
	Notifications() <-chan *cluster.Notification
	Errors() <-chan error
	Messages() <-chan *sarama.ConsumerMessage
	Close() (err error)
	MarkOffset(msg *sarama.ConsumerMessage, metadata string)
	MarkPartitionOffset(topic string, partition int32, offset int64, metadata string)
	Subscriptions() map[string][]int32
	CommitOffsets() error
}

// Consumer allows to consume message belonging to specified set of kafka
// topics.
type Consumer struct {
	logging.Logger
	Config       *Config
	SConsumer    sarama.Consumer
	Consumer     clusterConsumer
	closed       bool
	xwg          *sync.WaitGroup
	closeChannel chan struct{}
	sync.Mutex
}

// NewConsumer returns a Consumer instance. If startHandlers is set to true, reading of messages, errors
// and notifications is started using new consumer. Otherwise, only instance is returned
func NewConsumer(config *Config, wg *sync.WaitGroup) (*Consumer, error) {
	if config.Debug {
		config.Logger.SetLevel(logging.DebugLevel)
	}
	config.Logger.Debug("entering NewConsumer ...")
	if err := config.ValidateConsumerConfig(); err != nil {
		return nil, err
	}
	config.Logger.Debugf("Consumer config: %#v", config)

	// set consumer config params
	config.ConsumerConfig().Group.Return.Notifications = config.RecvNotification
	config.ProducerConfig().Consumer.Return.Errors = config.RecvError
	config.ConsumerConfig().Consumer.Offsets.Initial = config.InitialOffset

	cClient, err := cluster.NewClient(config.Brokers, config.Config)
	if err != nil {
		return nil, err
	}

	config.Logger.Debug("new client created successfully ...")

	consumer, err := cluster.NewConsumerFromClient(cClient, config.GroupID, config.Topics)
	if err != nil {
		return nil, err
	}

	sConsumer, err := sarama.NewConsumerFromClient(cClient)
	if err != nil {
		return nil, err
	}

	csmr := &Consumer{
		Logger:       config.Logger,
		Config:       config,
		SConsumer:    sConsumer,
		Consumer:     consumer,
		closed:       false,
		closeChannel: make(chan struct{}),
	}

	// if there is a "waitgroup" arg then use it
	if wg != nil {
		csmr.xwg = wg
		csmr.xwg.Add(1)
	}

	return csmr, nil
}

// StartConsumerHandlers starts required handlers using bsm/sarama consumer. Used when partitioner set in config is
// non-manual
func (ref *Consumer) StartConsumerHandlers() {
	config := ref.Config
	config.Logger.Info("Starting message handlers for new consumer ...")
	// if required, start reading from the notifications channel
	if config.ConsumerConfig().Group.Return.Notifications {
		go ref.notificationHandler(ref.Consumer.Notifications())
	}

	// if required, start reading from the errors channel
	if config.ProducerConfig().Consumer.Return.Errors {
		go ref.errorHandler(ref.Consumer.Errors())
	}

	// start the message handler
	go ref.messageHandler(ref.Consumer.Messages())
}

// StartConsumerManualHandlers starts required handlers using sarama partition consumer. Used when partitioner set in config is
// manual
func (ref *Consumer) StartConsumerManualHandlers(partitionConsumer sarama.PartitionConsumer) {
	config := ref.Config
	config.Logger.Info("Starting message handlers for new manual consumer ...")

	// if required, start reading from the errors channel
	if config.ProducerConfig().Consumer.Return.Errors {
		go ref.manualErrorHandler(partitionConsumer.Errors())
	}

	// start the message handler
	go ref.messageHandler(partitionConsumer.Messages())
}

// NewClient initializes new sarama client instance from provided config and with defined partitioner
func NewClient(config *Config, partitioner string) (sarama.Client, error) {
	config.Logger.Debug("Creating new consumer")
	if err := config.ValidateAsyncProducerConfig(); err != nil {
		return nil, err
	}

	config.SetSendSuccess(true)
	config.SetSuccessChan(make(chan *ProducerMessage))
	config.SetSendError(true)
	config.SetErrorChan(make(chan *ProducerError))
	// Required acks will be set in sync/async producer
	config.RequiredAcks = AcksUnset

	// set other Producer config params
	config.ProducerConfig().Producer.Return.Successes = config.SendSuccess
	config.ProducerConfig().Producer.Return.Errors = config.SendError

	// set partitioner
	switch partitioner {
	case Hash:
		config.ProducerConfig().Producer.Partitioner = sarama.NewHashPartitioner
	case Random:
		config.ProducerConfig().Producer.Partitioner = sarama.NewRandomPartitioner
	case Manual:
		config.ProducerConfig().Producer.Partitioner = sarama.NewManualPartitioner
	default:
		// Hash partitioner is set as default
		config.ProducerConfig().Producer.Partitioner = sarama.NewHashPartitioner
	}

	config.Logger.Debugf("AsyncProducer config: %#v", config)

	sClient, err := sarama.NewClient(config.Brokers, &config.Config.Config)
	if err != nil {
		fmt.Printf("Error creating consumer client %v", err)
		return nil, err
	}

	return sClient, nil
}

// Close closes the client and consumer
func (ref *Consumer) Close() error {
	ref.Debug("entering consumer close ...")
	defer func() {
		ref.Debug("running defer ...")
		if ref.closed {
			ref.Debug("consumer already closed ...")
			ref.Unlock()
			return
		}
		ref.Debug("setting closed ...")
		ref.closed = true
		ref.Debug("closing closeChannel channel ...")
		close(ref.closeChannel)

		if ref.xwg != nil {
			ref.xwg.Done()
		}
		ref.Unlock()
	}()

	ref.Debug("about to lock ...")
	ref.Lock()
	ref.Debug("locked ...")
	if ref.closed {
		return nil
	}

	// close consumer
	ref.Debug("calling consumer close ....")
	err := ref.Consumer.Close()
	if err != nil {
		ref.Errorf("consumer close error: %v", err)
		return err
	}
	ref.Debug("consumer closed")

	return nil
}

// IsClosed returns the "closed" status
func (ref *Consumer) IsClosed() bool {
	return ref.closed
}

// WaitForClose waits for the consumer to close
func (ref *Consumer) WaitForClose() {
	<-ref.closeChannel
	ref.Debug("exiting WaitForClose ...")
}

// MarkOffset marks the provided message as processed, alongside a metadata string
// that represents the state of the partition consumer at that point in time. The
// metadata string can be used by another consumer to restore that state, so it
// can resume consumption.
//
// Note: calling MarkOffset does not necessarily commit the offset to the backend
// store immediately for efficiency reasons, and it may never be committed if
// your application crashes. This means that you may end up processing the same
// message twice, and your processing should ideally be idempotent.
func (ref *Consumer) MarkOffset(msg *ConsumerMessage, metadata string) {

	ref.Consumer.MarkOffset(&sarama.ConsumerMessage{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
	}, metadata)
}

// MarkPartitionOffset marks an offset of the provided topic/partition as processed.
// See MarkOffset for additional explanation.
func (ref *Consumer) MarkPartitionOffset(topic string, partition int32, offset int64, metadata string) {
	ref.Consumer.MarkPartitionOffset(topic, partition, offset, metadata)
}

// Subscriptions returns the consumed topics and partitions
func (ref *Consumer) Subscriptions() map[string][]int32 {
	return ref.Consumer.Subscriptions()
}

// CommitOffsets manually commits marked offsets
func (ref *Consumer) CommitOffsets() error {
	return ref.Consumer.CommitOffsets()
}

// PrintNotification print the topics and partitions
func (ref *Consumer) PrintNotification(note map[string][]int32) {
	for k, v := range note {
		fmt.Printf("  Topic: %s\n", k)
		fmt.Printf("    Partitions: %v\n", v)
	}
}

// messageHandler processes each incoming message
func (ref *Consumer) messageHandler(in <-chan *sarama.ConsumerMessage) {
	ref.Debug("messageHandler started ...")
	var prevValue []byte

	for {
		select {
		case msg := <-in:
			if msg == nil {
				continue
			}
			consumerMsg := &ConsumerMessage{
				Key:       msg.Key,
				Value:     msg.Value,
				PrevValue: prevValue,
				Topic:     msg.Topic,
				Partition: msg.Partition,
				Offset:    msg.Offset,
				Timestamp: msg.Timestamp,
			}
			// Store value as previous for the next iteration
			prevValue = consumerMsg.Value
			select {
			case ref.Config.RecvMessageChan <- consumerMsg:
			case <-time.After(1 * time.Second):
				ref.Warn("Failed to deliver a message")
			}
		case <-ref.closeChannel:
			ref.Debug("Canceling message handler")
			return
		}
	}
}

// manualErrorHandler processes each error message for partition consumer
func (ref *Consumer) manualErrorHandler(in <-chan *sarama.ConsumerError) {
	ref.Debug("errorHandler started ...")
	for {
		select {
		case err, more := <-in:
			if more {
				ref.Errorf("message error: %T, %v", err, err)
				ref.Config.RecvErrorChan <- err
			}
		case <-ref.closeChannel:
			ref.Debug("Canceling error handler")
			return
		}
	}
}

// errorHandler processes each error message
func (ref *Consumer) errorHandler(in <-chan error) {
	ref.Debug("errorHandler started ...")
	for {
		select {
		case err, more := <-in:
			if more {
				ref.Errorf("message error: %T, %v", err, err)
				ref.Config.RecvErrorChan <- err
			}
		case <-ref.closeChannel:
			ref.Debug("Canceling error handler")
			return
		}
	}
}

// NotificationHandler processes each message received when the consumer is rebalanced
func (ref *Consumer) notificationHandler(in <-chan *cluster.Notification) {
	ref.Debug("NotificationHandler started ...")

	for {
		select {
		case note := <-in:
			ref.Config.RecvNotificationChan <- note
		case <-ref.closeChannel:
			ref.Debug("Canceling notification handler")
			return
		}
	}
}

// GetCloseChannel returns a channel that is closed on asyncProducer cleanup
func (ref *Consumer) GetCloseChannel() <-chan struct{} {
	return ref.closeChannel
}
