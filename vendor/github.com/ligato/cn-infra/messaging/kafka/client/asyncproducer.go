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
	"crypto/md5"
	"errors"
	"fmt"
	"sync"

	"github.com/Shopify/sarama"
	"github.com/ligato/cn-infra/logging"
)

// AsyncProducer allows to publish message to kafka using asynchronous API.
// The message using SendMsgToPartition and SendMsgByte function returns do not block.
// The status whether message was sent successfully or not is delivered using channels
// specified in config structure.
type AsyncProducer struct {
	logging.Logger
	Config       *Config
	Client       sarama.Client
	Producer     sarama.AsyncProducer
	Partition    int32
	closed       bool
	xwg          *sync.WaitGroup
	closeChannel chan struct{}
	sync.Mutex
}

// NewAsyncProducer returns a new AsyncProducer instance. Producer is created from provided sarama client which can be nil;
// in that case a new client will be created. Also the partitioner is set here. Note: provided sarama client partitioner
// should match the one used in config.
func NewAsyncProducer(config *Config, sClient sarama.Client, partitioner string, wg *sync.WaitGroup) (*AsyncProducer, error) {
	if config.Debug {
		config.Logger.SetLevel(logging.DebugLevel)
	}

	config.Logger.Debug("Entering NewAsyncProducer ...")
	if err := config.ValidateAsyncProducerConfig(); err != nil {
		return nil, err
	}

	// set "RequiredAcks" for producer
	if config.RequiredAcks == AcksUnset {
		config.RequiredAcks = WaitForLocal
	}
	err := setProducerRequiredAcks(config)
	if err != nil {
		return nil, errors.New("invalid RequiredAcks field in config")
	}

	// set partitioner
	config.SetPartitioner(partitioner)

	// initAsyncProducer object
	ap := &AsyncProducer{
		Logger:       config.Logger,
		Config:       config,
		Partition:    config.Partition,
		closed:       false,
		closeChannel: make(chan struct{}),
	}

	// If client is nil, create a new one
	if sClient == nil {
		localClient, err := NewClient(config, partitioner)
		if err != nil {
			return nil, err
		}
		// store local client in syncProducer if it was created here
		ap.Client = localClient
		sClient = localClient
	}

	// init a new asyncproducer using this client
	producer, err := sarama.NewAsyncProducerFromClient(sClient)
	if err != nil {
		return nil, err
	}
	ap.Producer = producer

	// if there is a "waitgroup" arg then use it
	if wg != nil {
		ap.xwg = wg
		ap.xwg.Add(1)
	}

	// if required, start reading from the successes channel
	if config.ProducerConfig().Producer.Return.Successes {
		go ap.successHandler(ap.Producer.Successes())
	}

	// if required, start reading from the errors channel
	if config.ProducerConfig().Producer.Return.Errors {
		go ap.errorHandler(ap.Producer.Errors())
	}

	return ap, nil
}

// SendMsgByte sends an async message to Kafka.
func (ref *AsyncProducer) SendMsgByte(topic string, key []byte, msg []byte, metadata interface{}) {
	// generate key if none supplied - used by Hash partitioner
	if key == nil || len(key) == 0 {
		md5Sum := fmt.Sprintf("%x", md5.Sum(msg))
		ref.SendMsgToPartition(topic, ref.Partition, sarama.ByteEncoder([]byte(md5Sum)), sarama.ByteEncoder(msg), metadata)
		return
	}
	ref.SendMsgToPartition(topic, ref.Partition, sarama.ByteEncoder(key), sarama.ByteEncoder(msg), metadata)
}

// SendMsgToPartition sends an async message to Kafka
func (ref *AsyncProducer) SendMsgToPartition(topic string, partition int32, key Encoder, msg Encoder, metadata interface{}) {
	if msg == nil {
		return
	}

	message := &sarama.ProducerMessage{
		Topic:     topic,
		Partition: partition,
		Key:       key,
		Value:     msg,
		Metadata:  metadata,
	}

	ref.Producer.Input() <- message

	ref.Debugf("message sent: %s", message)

	return
}

// Close closes the client and producer
func (ref *AsyncProducer) Close(async ...bool) error {
	var err error
	defer func() {
		if ref.closed {
			ref.Unlock()
			return
		}
		ref.closed = true
		close(ref.closeChannel)

		// decrement external wait group
		if ref.xwg != nil {
			ref.xwg.Done()
		}
		ref.Unlock()
	}()

	ref.Lock()
	if ref.closed {
		return nil
	}

	if async != nil && len(async) > 0 {
		ref.Debug("async close")
		ref.Producer.AsyncClose()
	} else {
		ref.Debug("sync close")
		ref.Producer.Close()
	}
	if err != nil {
		ref.Errorf("asyncProducer close error: %v", err)
		return err
	}
	if ref.Client != nil && !ref.Client.Closed() {
		err = ref.Client.Close()
		if err != nil {
			ref.Errorf("client close error: %v", err)
			return err
		}
	}

	return nil
}

// successHandler handles success messages
func (ref *AsyncProducer) successHandler(in <-chan *sarama.ProducerMessage) {
	ref.Debug("starting success handler ...")
	for {
		select {
		case <-ref.closeChannel:
			ref.Debug("success handler exited ...")
			return
		case msg := <-in:
			if msg == nil {
				continue
			}
			ref.Debugf("Message is stored in topic(%s)/partition(%d)/offset(%d)\n", msg.Topic, msg.Partition, msg.Offset)
			pmsg := &ProducerMessage{
				Topic:     msg.Topic,
				Key:       msg.Key,
				Value:     msg.Value,
				Metadata:  msg.Metadata,
				Offset:    msg.Offset,
				Partition: msg.Partition,
			}
			ref.Config.SuccessChan <- pmsg
		}
	}
}

// errorHandler handles error messages
func (ref *AsyncProducer) errorHandler(in <-chan *sarama.ProducerError) {
	ref.Debug("starting error handler ...")
	for {
		select {
		case <-ref.closeChannel:
			ref.Debug("error handler exited ...")
			return
		case perr := <-in:
			if perr == nil {
				continue
			}

			msg := perr.Msg
			err := perr.Err
			pmsg := &ProducerMessage{
				Topic:     msg.Topic,
				Key:       msg.Key,
				Value:     msg.Value,
				Metadata:  msg.Metadata,
				Offset:    msg.Offset,
				Partition: msg.Partition,
			}
			perr2 := &ProducerError{
				ProducerMessage: pmsg,
				Err:             err,
			}
			val, _ := msg.Value.Encode()
			ref.Errorf("message %s errored in topic(%s)/partition(%d)/offset(%d)\n", string(val), pmsg.Topic, pmsg.Partition, pmsg.Offset)
			ref.Errorf("message error: %v", perr.Err)
			ref.Config.ErrorChan <- perr2
		}
	}
}

// IsClosed returns the "closed" status
func (ref *AsyncProducer) IsClosed() bool {
	ref.Lock()
	defer ref.Unlock()
	return ref.closed
}

// WaitForClose returns when the producer is closed
func (ref *AsyncProducer) WaitForClose() {
	<-ref.closeChannel
}

// GetCloseChannel returns a channel that is closed on asyncProducer cleanup
func (ref *AsyncProducer) GetCloseChannel() <-chan struct{} {
	return ref.closeChannel
}
