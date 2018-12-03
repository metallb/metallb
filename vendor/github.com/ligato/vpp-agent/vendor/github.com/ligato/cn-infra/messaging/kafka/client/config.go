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
	"context"
	"errors"
	"strings"

	"crypto/tls"
	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	"github.com/ligato/cn-infra/logging"
)

// RequiredAcks is used in Produce Requests to tell the broker how many replica acknowledgements
// it must see before responding. Any of the constants defined here are valid except AcksUnset.
type RequiredAcks int16

const (
	// AcksUnset indicates that no valid value has been set
	AcksUnset RequiredAcks = -32768
	// NoResponse doesn't send any response, the TCP ACK is all you get.
	NoResponse RequiredAcks = 0
	// WaitForLocal waits for only the local commit to succeed before responding.
	WaitForLocal RequiredAcks = 1
	// WaitForAll waits for all replicas to commit before responding.
	WaitForAll RequiredAcks = -1
)

// Partitioner schemes
const (
	// Hash scheme (messages with the same key always end up on the same partition)
	Hash = "hash"
	// Random scheme (random partition is always used)
	Random = "random"
	// Manual scheme (partitions are manually set in the provided message's partition field)
	Manual = "manual"
)

// Config struct provides the configuration for a Producer (Sync or Async) and Consumer.
type Config struct {
	logging.Logger
	// Config extends the sarama-cluster.Config with the kafkaclient namespace
	*cluster.Config
	// Context Package carries deadlines, cancelation signals, and other values.
	// see: http://golang.org/x/net/context
	Context context.Context
	// Cancel is a function that can be call, e.g. config.Cancel(), to cancel and close
	// the producer/consumer
	Cancel context.CancelFunc
	// Brokers contains "{domain:port}" array of Kafka brokers.
	// This list of brokers is used by the kafkaclient to determine the 'lead' broker for each topic
	// and the 'lead' consumer for each topic. If only one broker is supplied then it will be used to
	// communicate with the other brokers.
	// REQUIRED: PRODUCER AND CONSUMER.
	Brokers []string
	// GroupID contains the name of the consumer's group.
	// REQUIRED: CONSUMER.
	GroupID string
	// Debug determines if debug code should be 'turned-on'.
	// DEFAULT: false. OPTIONAL.
	Debug bool
	// Topics contains the topics that a consumer should retrieve messages for.
	// REQUIRED: CONSUMER.
	Topics []string
	// Partition is the partition. Used when configuring partitions manually.
	Partition int32
	// Partitioner is the method used to determine a topic's partition.
	// REQUIRED: PRODUCER. DEFAULT: HASH
	Partitioner sarama.PartitionerConstructor
	// InitialOffset indicates the initial offset that should be used when a consumer is initialized and begins reading
	// the Kafka message log for the topic. If the offset was previously committed then the committed offset is used
	// rather than the initial offset.
	// REQUIRED: CONSUMER
	InitialOffset int64
	// RequiredAcks is the level of acknowledgement reliability needed from the broker
	// REQUIRED: PRODUCER. DEFAULT(Async) WaitForLocal DEFAULT(Sync) WaitForAll
	RequiredAcks RequiredAcks
	// RecvNotification indicates that a Consumer return "Notification" messages after it has rebalanced.
	// REQUIRED: CONSUMER. DEFAULT: false.
	RecvNotification bool
	// NotificationChan function called when a "Notification" message is received by a consumer.
	// REQUIRED: CONSUMER if 'RecvNotification=true'
	RecvNotificationChan chan *cluster.Notification
	// RecvError indicates that "receive" errors should not be ignored and should be returned to the consumer.
	// REQUIRED: CONSUMER. DEFAULT: true.
	RecvError bool
	// RecvErrorChan channel is for delivery of "Error" messages received by the consumer.
	// REQUIRED: CONSUMER if 'RecvError=true'
	RecvErrorChan chan error
	// MessageChan channel is used for delivery of consumer messages.
	// REQUIRED: CONSUMER
	RecvMessageChan chan *ConsumerMessage
	// SendSuccess indicates that the Async Producer should return "Success" messages when a message
	//  has been successfully received by the Kafka.
	// REQUIRED: CONSUMER. DEFAULT: false.
	SendSuccess bool
	// SuccessChan is used for delivery of message when a "Success" is returned by Async Producer.
	// REQUIRED: PRODUCER if 'SendSuccess=true'
	SuccessChan chan *ProducerMessage
	// SendError indicates that an Async Producer should return "Error" messages when a message transmission to Kafka
	// failed.
	// REQUIRED: CONSUMER. DEFAULT: true.
	SendError bool
	// ErrorChan is used for delivery of "Error" message if an error is returned by Async Producer.
	// REQUIRED: PRODUCER if 'SendError=true'
	ErrorChan chan *ProducerError
}

// NewConfig return a new Config object.
func NewConfig(log logging.Logger) *Config {

	cfg := &Config{
		Logger:       log,
		Config:       cluster.NewConfig(),
		Partition:    -1,
		Partitioner:  sarama.NewHashPartitioner,
		RequiredAcks: AcksUnset,
	}

	return cfg
}

// SetBrokers sets the Config.Brokers field
func (ref *Config) SetBrokers(brokers ...string) {
	ref.Brokers = brokers
}

// SetTopics sets the Config.Topics field
func (ref *Config) SetTopics(topics string) {
	ref.Topics = strings.Split(topics, ",")
}

// SetDebug sets the Config.Debug field
func (ref *Config) SetDebug(val bool) {
	if val {
		ref.Debug = val
		sarama.Logger = ref.Logger
		ref.SetLevel(logging.DebugLevel)
	} else {
		ref.Debug = val
	}
}

// SetGroup sets the Config.GroupID field
func (ref *Config) SetGroup(id string) {
	ref.GroupID = id
}

// SetAcks sets the Config.RequiredAcks field
func (ref *Config) SetAcks(acks RequiredAcks) {
	ref.RequiredAcks = acks
}

// SetInitialOffset sets the Config.InitialOffset field
func (ref *Config) SetInitialOffset(offset int64) {
	ref.InitialOffset = offset
}

// SetSendSuccess sets the Config.SendSuccess field
func (ref *Config) SetSendSuccess(val bool) {
	ref.SendSuccess = val
}

// SetSendError sets the Config.SendError field
func (ref *Config) SetSendError(val bool) {
	ref.SendError = val
}

// SetRecvNotification sets the Config.RecvNotification field
func (ref *Config) SetRecvNotification(val bool) {
	ref.RecvNotification = val
}

// SetRecvError sets the Config.RecvError field
func (ref *Config) SetRecvError(val bool) {
	ref.RecvError = val
}

// SetSuccessChan sets the Config.SuccessChan field
func (ref *Config) SetSuccessChan(val chan *ProducerMessage) {
	ref.SuccessChan = val
}

// SetErrorChan sets the Config.ErrorChan field
func (ref *Config) SetErrorChan(val chan *ProducerError) {
	ref.ErrorChan = val
}

// SetRecvNotificationChan sets the Config.RecvNotificationChan field
func (ref *Config) SetRecvNotificationChan(val chan *cluster.Notification) {
	ref.RecvNotificationChan = val
}

// SetRecvErrorChan sets the Config.RecvErrorChan field
func (ref *Config) SetRecvErrorChan(val chan error) {
	ref.RecvErrorChan = val
}

// SetRecvMessageChan sets the Config.RecvMessageChan field
func (ref *Config) SetRecvMessageChan(val chan *ConsumerMessage) {
	ref.RecvMessageChan = val
}

// ProducerConfig sets the Config.ProducerConfig field
func (ref *Config) ProducerConfig() *sarama.Config {
	return &ref.Config.Config
}

// ConsumerConfig sets the Config.ConsumerConfig field
func (ref *Config) ConsumerConfig() *cluster.Config {
	return ref.Config
}

// SetPartition sets the Config.SetPartition field
func (ref *Config) SetPartition(val int32) {
	ref.Partition = val
}

// SetPartitioner sets the Config.SetPartitioner field
func (ref *Config) SetPartitioner(val string) {
	switch val {
	default:
		ref.Errorf("Invalid partitioner %s - defaulting to ''", val)
		fallthrough
	case "":
		if ref.Partition >= 0 {
			ref.Partitioner = sarama.NewManualPartitioner
		} else {
			ref.Partitioner = sarama.NewHashPartitioner
		}
	case Hash:
		ref.Partitioner = sarama.NewHashPartitioner
		ref.Partition = -1
	case Random:
		ref.Partitioner = sarama.NewRandomPartitioner
		ref.Partition = -1
	case Manual:
		ref.Partitioner = sarama.NewManualPartitioner
		if ref.Partition < 0 {
			ref.Infof("Invalid partition %d - defaulting to 0", ref.Partition)
			ref.Partition = 0
		}
	}
}

// SetTLS sets the TLS configuration
func (ref *Config) SetTLS(tlsConfig *tls.Config) (err error) {
	ref.Net.TLS.Enable = true
	ref.Net.TLS.Config = tlsConfig

	return nil
}

// ValidateAsyncProducerConfig validates config for an Async Producer
func (ref *Config) ValidateAsyncProducerConfig() error {
	if ref.Brokers == nil {
		return errors.New("invalid Brokers - one or more brokers must be specified")
	}
	if ref.SendSuccess && ref.SuccessChan == nil {
		return errors.New("success channel not specified")
	}
	if ref.SendError && ref.ErrorChan == nil {
		return errors.New("error channel not specified")
	}
	return ref.ProducerConfig().Validate()
}

// ValidateSyncProducerConfig validates config for a Sync Producer
func (ref *Config) ValidateSyncProducerConfig() error {
	if ref.Brokers == nil {
		return errors.New("invalid Brokers - one or more brokers must be specified")
	}
	return ref.ProducerConfig().Validate()
}

// ValidateConsumerConfig validates config for Consumer
func (ref *Config) ValidateConsumerConfig() error {
	if ref.Brokers == nil {
		return errors.New("invalid Brokers - one or more brokers must be specified")
	}
	if ref.GroupID == "" {
		return errors.New("invalid GroupID - no GroupID specified")
	}
	if ref.RecvNotification && ref.RecvNotificationChan == nil {
		return errors.New("notification channel not specified")
	}
	if ref.RecvError && ref.RecvErrorChan == nil {
		return errors.New("error channel not specified")
	}
	if ref.RecvMessageChan == nil {
		return errors.New("recvMessageChan not specified")
	}
	return ref.ConsumerConfig().Validate()
}
