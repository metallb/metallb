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
	"time"

	"github.com/Shopify/sarama"
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/db/keyval"
)

// Encoder defines an interface that is used as argument of producer functions.
// It wraps the sarama.Encoder
type Encoder interface {
	sarama.Encoder
}

// ConsumerMessage encapsulates a Kafka message returned by the consumer.
type ConsumerMessage struct {
	Key, Value, PrevValue []byte
	Topic                 string
	Partition             int32
	Offset                int64
	Timestamp             time.Time
}

// GetTopic returns the topic associated with the message
func (cm *ConsumerMessage) GetTopic() string {
	return cm.Topic
}

// GetPartition returns the partition associated with the message
func (cm *ConsumerMessage) GetPartition() int32 {
	return cm.Partition
}

// GetOffset returns the offset associated with the message
func (cm *ConsumerMessage) GetOffset() int64 {
	return cm.Offset
}

// GetKey returns the key associated with the message.
func (cm *ConsumerMessage) GetKey() string {
	return string(cm.Key)
}

// GetValue returns the value associated with the message.
func (cm *ConsumerMessage) GetValue() []byte {
	return cm.Value
}

// GetPrevValue returns the previous value associated with the message.
func (cm *ConsumerMessage) GetPrevValue() []byte {
	return cm.PrevValue
}

// ProtoConsumerMessage encapsulates a Kafka message returned by the consumer and provides means
// to unmarshal the value into proto.Message.
type ProtoConsumerMessage struct {
	*ConsumerMessage
	serializer keyval.Serializer
}

// NewProtoConsumerMessage creates new instance of ProtoConsumerMessage
func NewProtoConsumerMessage(msg *ConsumerMessage, serializer keyval.Serializer) *ProtoConsumerMessage {
	return &ProtoConsumerMessage{msg, serializer}
}

// GetTopic returns the topic associated with the message.
func (cm *ProtoConsumerMessage) GetTopic() string {
	return cm.Topic
}

// GetPartition returns the partition associated with the message.
func (cm *ProtoConsumerMessage) GetPartition() int32 {
	return cm.Partition
}

// GetOffset returns the offset associated with the message.
func (cm *ProtoConsumerMessage) GetOffset() int64 {
	return cm.Offset
}

// GetKey returns the key associated with the message.
func (cm *ProtoConsumerMessage) GetKey() string {
	return string(cm.Key)
}

// GetValue returns the value associated with the message.
func (cm *ProtoConsumerMessage) GetValue(msg proto.Message) error {
	err := cm.serializer.Unmarshal(cm.ConsumerMessage.GetValue(), msg)
	if err != nil {
		return err
	}
	return nil
}

// GetPrevValue returns the previous value associated with the latest message.
func (cm *ProtoConsumerMessage) GetPrevValue(msg proto.Message) (prevValueExist bool, err error) {
	prevVal := cm.ConsumerMessage.GetPrevValue()
	if prevVal == nil {
		return false, nil
	}
	err = cm.serializer.Unmarshal(prevVal, msg)
	if err != nil {
		return true, err
	}
	return true, nil
}

// ProducerMessage is the collection of elements passed to the Producer in order to send a message.
type ProducerMessage struct {
	// The Kafka topic for this message.
	Topic string
	// The partitioning key for this message. Pre-existing Encoders include
	// StringEncoder and ByteEncoder.
	Key Encoder
	// The actual message to store in Kafka. Pre-existing Encoders include
	// StringEncoder and ByteEncoder.
	Value Encoder

	// This field is used to hold arbitrary data you wish to include so it
	// will be available when receiving on the Successes and Errors channels.
	// Sarama completely ignores this field and is only to be used for
	// pass-through data.
	Metadata interface{}

	// Below this point are filled in by the producer as the message is processed

	// Offset is the offset of the message stored on the broker. This is only
	// guaranteed to be defined if the message was successfully delivered and
	// RequiredAcks is not NoResponse.
	Offset int64
	// Partition is the partition that the message was sent to. This is only
	// guaranteed to be defined if the message was successfully delivered.
	Partition int32
}

// GetTopic returns the topic associated with the message.
func (pm *ProducerMessage) GetTopic() string {
	return pm.Topic
}

// GetPartition returns the partition associated with the message.
func (pm *ProducerMessage) GetPartition() int32 {
	return pm.Partition
}

// GetOffset returns the offset associated with the message.
func (pm *ProducerMessage) GetOffset() int64 {
	return pm.Offset
}

// GetKey returns the key associated with the message.
func (pm *ProducerMessage) GetKey() string {
	key, _ := pm.Key.Encode()
	return string(key)
}

// GetValue returns the content of the message.
func (pm *ProducerMessage) GetValue() []byte {
	val, _ := pm.Value.Encode()
	return val
}

// GetPrevValue returns nil for the producer
func (pm *ProducerMessage) GetPrevValue() []byte {
	return nil
}

func (pm *ProducerMessage) String() string {
	var meta string
	switch t := pm.Metadata.(type) {
	default:
		meta = fmt.Sprintf("unexpected type %T", t) // %T prints whatever type t has
	case string:
		meta = t
	case *string:
		meta = *t
	case []byte:
		meta = string(t)
	case bool:
		meta = fmt.Sprintf("%t", t) // t has type bool
	case int:
		meta = fmt.Sprintf("%d", t) // t has type int
	case *bool:
		meta = fmt.Sprintf("%t", *t) // t has type *bool
	case *int:
		meta = fmt.Sprintf("%d", *t) // t has type *int
	}

	key, _ := pm.Key.Encode()
	val, _ := pm.Value.Encode()

	return fmt.Sprintf("ProducerMessage - Topic: %s, Key: %s, Value: %s, Meta: %v, Offset: %d, Partition: %d\n", pm.Topic, string(key), string(val), meta, pm.Offset, pm.Partition)
}

// ProducerError is the type of error generated when the producer fails to deliver a message.
// It contains the original ProducerMessage as well as the actual error value.
type ProducerError struct {
	*ProducerMessage
	Err error
}

func (ref *ProducerError) Error() error {
	return ref.Err
}

func (ref *ProducerError) String() string {
	return fmt.Sprintf("ProducerError: %s, error: %v\n", ref.ProducerMessage, ref.Err.Error())
}

// ProtoProducerMessage is wrapper of a producer message that simplify work with proto-modelled data.
type ProtoProducerMessage struct {
	*ProducerMessage
	Serializer keyval.Serializer
}

// GetTopic returns the topic associated with the message.
func (ppm *ProtoProducerMessage) GetTopic() string {
	return ppm.Topic
}

// GetPartition returns the partition associated with the message.
func (ppm *ProtoProducerMessage) GetPartition() int32 {
	return ppm.Partition
}

// GetOffset returns the offset associated with the message.
func (ppm *ProtoProducerMessage) GetOffset() int64 {
	return ppm.Offset
}

// GetKey returns the key associated with the message.
func (ppm *ProtoProducerMessage) GetKey() string {
	key, _ := ppm.Key.Encode()
	return string(key)
}

// GetValue unmarshalls the content of the msg into provided structure.
func (ppm *ProtoProducerMessage) GetValue(msg proto.Message) error {
	err := ppm.Serializer.Unmarshal(ppm.ProducerMessage.GetValue(), msg)
	if err != nil {
		return err
	}
	return nil
}

// GetPrevValue for producer returns false (value does not exist)
func (ppm *ProtoProducerMessage) GetPrevValue(msg proto.Message) (prevValueExist bool, err error) {
	return false, nil
}

// ProtoProducerMessageErr represents a proto-modelled message that was not published successfully.
type ProtoProducerMessageErr struct {
	*ProtoProducerMessage
	Err error
}

func (pme *ProtoProducerMessageErr) Error() error {
	return pme.Err
}
