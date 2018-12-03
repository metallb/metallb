package mux

import (
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/messaging/kafka/client"
)

// BytesConnection is interface for multiplexer with dynamic partitioner.
type BytesConnection interface {
	// Creates new synchronous publisher allowing to publish kafka messages
	NewSyncPublisher(topic string) (BytesPublisher, error)
	// Creates new asynchronous publisher allowing to publish kafka messages
	NewAsyncPublisher(topic string, successClb func(*client.ProducerMessage), errorClb func(err *client.ProducerError)) (BytesPublisher, error)
}

// BytesManualConnection is interface for multiplexer with manual partitioner.
type BytesManualConnection interface {
	// Creates new synchronous publisher allowing to publish kafka messages to chosen partition
	NewSyncPublisherToPartition(topic string, partition int32) (BytesPublisher, error)
	// Creates new asynchronous publisher allowing to publish kafka messages to chosen partition
	NewAsyncPublisherToPartition(topic string, partition int32, successClb func(*client.ProducerMessage), errorClb func(err *client.ProducerError)) (BytesPublisher, error)
}

// BytesConnectionStr represents connection built on hash-mode multiplexer
type BytesConnectionStr struct {
	BytesConnectionFields
}

// BytesManualConnectionStr represents connection built on manual-mode multiplexer
type BytesManualConnectionStr struct {
	BytesConnectionFields
}

// BytesConnectionFields is an entity that provides access to shared producers/consumers of multiplexer
type BytesConnectionFields struct {
	// multiplexer is used for access to kafka brokers
	multiplexer *Multiplexer
	// name identifies the connection
	name string
}

// BytesPublisher allows to publish a message of type []bytes into messaging system.
type BytesPublisher interface {
	Put(key string, data []byte) error
}

type bytesSyncPublisherKafka struct {
	conn  *BytesConnectionStr
	topic string
}

type bytesAsyncPublisherKafka struct {
	conn         *BytesConnectionStr
	topic        string
	succCallback func(*client.ProducerMessage)
	errCallback  func(*client.ProducerError)
}

type bytesManualSyncPublisherKafka struct {
	conn      *BytesManualConnectionStr
	topic     string
	partition int32
}

type bytesManualAsyncPublisherKafka struct {
	conn         *BytesManualConnectionStr
	topic        string
	partition    int32
	succCallback func(*client.ProducerMessage)
	errCallback  func(*client.ProducerError)
}

// NewSyncPublisher creates a new instance of bytesSyncPublisherKafka that allows to publish sync kafka messages using common messaging API
func (conn *BytesConnectionStr) NewSyncPublisher(topic string) (BytesPublisher, error) {
	return &bytesSyncPublisherKafka{conn, topic}, nil
}

// NewAsyncPublisher creates a new instance of bytesAsyncPublisherKafka that allows to publish async kafka messages using common messaging API
func (conn *BytesConnectionStr) NewAsyncPublisher(topic string, successClb func(*client.ProducerMessage), errorClb func(err *client.ProducerError)) (BytesPublisher, error) {
	return &bytesAsyncPublisherKafka{conn, topic, successClb, errorClb}, nil
}

// NewSyncPublisherToPartition creates a new instance of bytesSyncPublisherKafka that allows to publish sync kafka messages using common messaging API
func (conn *BytesManualConnectionStr) NewSyncPublisherToPartition(topic string, partition int32) (BytesPublisher, error) {
	return &bytesManualSyncPublisherKafka{conn, topic, partition}, nil
}

// NewAsyncPublisherToPartition creates a new instance of bytesAsyncPublisherKafka that allows to publish async kafka messages using common messaging API
func (conn *BytesManualConnectionStr) NewAsyncPublisherToPartition(topic string, partition int32, successClb func(*client.ProducerMessage), errorClb func(err *client.ProducerError)) (BytesPublisher, error) {
	return &bytesManualAsyncPublisherKafka{conn, topic, partition, successClb, errorClb}, nil
}

// ConsumeTopic is called to start consuming of a topic.
// Function can be called until the multiplexer is started, it returns an error otherwise.
// The provided channel should be buffered, otherwise messages might be lost.
func (conn *BytesConnectionStr) ConsumeTopic(msgClb func(message *client.ConsumerMessage), topics ...string) error {
	conn.multiplexer.rwlock.Lock()
	defer conn.multiplexer.rwlock.Unlock()

	if conn.multiplexer.started {
		return fmt.Errorf("ConsumeTopic can be called only if the multiplexer has not been started yet")
	}

	for _, topic := range topics {
		// check if we have already consumed the topic
		var found bool
		var subs *consumerSubscription
	LoopSubs:
		for _, subscription := range conn.multiplexer.mapping {
			if subscription.manual == true {
				// do not mix dynamic and manual mode
				continue
			}
			if subscription.topic == topic {
				found = true
				subs = subscription
				break LoopSubs
			}
		}

		if !found {
			subs = &consumerSubscription{
				manual:         false, // non-manual example
				topic:          topic,
				connectionName: conn.name,
				byteConsMsg:    msgClb,
			}
			// subscribe new topic
			conn.multiplexer.mapping = append(conn.multiplexer.mapping, subs)
		}

		// add subscription to consumerList
		subs.byteConsMsg = msgClb
	}

	return nil
}

// ConsumePartition is called to start consuming given topic on partition with offset
// Function can be called until the multiplexer is started, it returns an error otherwise.
// The provided channel should be buffered, otherwise messages might be lost.
func (conn *BytesManualConnectionStr) ConsumePartition(msgClb func(message *client.ConsumerMessage), topic string, partition int32, offset int64) error {
	conn.multiplexer.rwlock.Lock()
	defer conn.multiplexer.rwlock.Unlock()
	var err error

	// check if we have already consumed the topic on partition and offset
	var found bool
	var subs *consumerSubscription

	for _, subscription := range conn.multiplexer.mapping {
		if subscription.manual == false {
			// do not mix dynamic and manual mode
			continue
		}
		if subscription.topic == topic && subscription.partition == partition && subscription.offset == offset {
			found = true
			subs = subscription
			break
		}
	}

	if !found {
		subs = &consumerSubscription{
			manual:         true, // manual example
			topic:          topic,
			partition:      partition,
			offset:         offset,
			connectionName: conn.name,
			byteConsMsg:    msgClb,
		}
		// subscribe new topic on partition
		conn.multiplexer.mapping = append(conn.multiplexer.mapping, subs)
	}

	// add subscription to consumerList
	subs.byteConsMsg = msgClb

	if conn.multiplexer.started {
		conn.multiplexer.Infof("Starting 'post-init' manual Consumer")
		subs.partitionConsumer, err = conn.StartPostInitConsumer(topic, partition, offset)
		if err != nil {
			return err
		}
		if subs.partitionConsumer == nil {
			return nil
		}
	}

	return nil
}

// StartPostInitConsumer allows to start a new partition consumer after mux is initialized
func (conn *BytesManualConnectionStr) StartPostInitConsumer(topic string, partition int32, offset int64) (*sarama.PartitionConsumer, error) {
	multiplexer := conn.multiplexer
	multiplexer.WithFields(logging.Fields{"topic": topic}).Debugf("Post-init consuming started")

	if multiplexer.Consumer == nil || multiplexer.Consumer.SConsumer == nil {
		multiplexer.Warn("Unable to start post-init Consumer, client not available in the mux")
		return nil, nil
	}

	// Consumer that reads topic/partition/offset. Throws error if offset is 'in the future' (message with offset does not exist yet)
	partitionConsumer, err := multiplexer.Consumer.SConsumer.ConsumePartition(topic, partition, offset)
	if err != nil {
		return nil, err
	}
	multiplexer.Consumer.StartConsumerManualHandlers(partitionConsumer)

	return &partitionConsumer, nil
}

// StopConsuming cancels the previously created subscription for consuming the topic.
func (conn *BytesConnectionStr) StopConsuming(topic string) error {
	return conn.multiplexer.stopConsuming(topic, conn.name)
}

// StopConsumingPartition cancels the previously created subscription for consuming the topic, partition and offset
func (conn *BytesManualConnectionStr) StopConsumingPartition(topic string, partition int32, offset int64) error {
	return conn.multiplexer.stopConsumingPartition(topic, partition, offset, conn.name)
}

//SendSyncMessage sends a message using the sync API and default partitioner
func (conn *BytesConnectionStr) SendSyncMessage(topic string, key client.Encoder, value client.Encoder) (offset int64, err error) {
	msg, err := conn.multiplexer.hashSyncProducer.SendMsgToPartition(topic, DefPartition, key, value)
	if err != nil {
		return 0, err
	}
	return msg.Offset, err
}

// SendAsyncMessage sends a message using the async API and default partitioner
func (conn *BytesConnectionStr) SendAsyncMessage(topic string, key client.Encoder, value client.Encoder, meta interface{}, successClb func(*client.ProducerMessage), errClb func(*client.ProducerError)) {
	auxMeta := &asyncMeta{successClb: successClb, errorClb: errClb, usersMeta: meta}
	conn.multiplexer.hashAsyncProducer.SendMsgToPartition(topic, DefPartition, key, value, auxMeta)
}

//SendSyncMessageToPartition sends a message using the sync API and default partitioner
func (conn *BytesManualConnectionStr) SendSyncMessageToPartition(topic string, partition int32, key client.Encoder, value client.Encoder) (offset int64, err error) {
	msg, err := conn.multiplexer.manSyncProducer.SendMsgToPartition(topic, partition, key, value)
	if err != nil {
		return 0, err
	}
	return msg.Offset, err
}

// SendAsyncMessageToPartition sends a message using the async API and default partitioner
func (conn *BytesManualConnectionStr) SendAsyncMessageToPartition(topic string, partition int32, key client.Encoder, value client.Encoder, meta interface{}, successClb func(*client.ProducerMessage), errClb func(*client.ProducerError)) {
	auxMeta := &asyncMeta{successClb: successClb, errorClb: errClb, usersMeta: meta}
	conn.multiplexer.manAsyncProducer.SendMsgToPartition(topic, partition, key, value, auxMeta)
}

// SendSyncByte sends a message that uses byte encoder using the sync API
func (conn *BytesConnectionStr) SendSyncByte(topic string, key []byte, value []byte) (offset int64, err error) {
	return conn.SendSyncMessage(topic, sarama.ByteEncoder(key), sarama.ByteEncoder(value))
}

// SendSyncString sends a message that uses string encoder using the sync API
func (conn *BytesConnectionStr) SendSyncString(topic string, key string, value string) (offset int64, err error) {
	return conn.SendSyncMessage(topic, sarama.StringEncoder(key), sarama.StringEncoder(value))
}

// SendSyncStringToPartition sends a message that uses string encoder using the sync API to custom partition
func (conn *BytesManualConnectionStr) SendSyncStringToPartition(topic string, partition int32, key string, value string) (offset int64, err error) {
	return conn.SendSyncMessageToPartition(topic, partition, sarama.StringEncoder(key), sarama.StringEncoder(value))
}

// SendAsyncByte sends a message that uses byte encoder using the async API
func (conn *BytesConnectionStr) SendAsyncByte(topic string, key []byte, value []byte, meta interface{}, successClb func(*client.ProducerMessage), errClb func(*client.ProducerError)) {
	conn.SendAsyncMessage(topic, sarama.ByteEncoder(key), sarama.ByteEncoder(value), meta, successClb, errClb)
}

// SendAsyncString sends a message that uses string encoder using the async API
func (conn *BytesConnectionStr) SendAsyncString(topic string, key string, value string, meta interface{}, successClb func(*client.ProducerMessage), errClb func(*client.ProducerError)) {
	conn.SendAsyncMessage(topic, sarama.StringEncoder(key), sarama.StringEncoder(value), meta, successClb, errClb)
}

// SendAsyncStringToPartition sends a message that uses string encoder using the async API to custom partition
func (conn *BytesManualConnectionStr) SendAsyncStringToPartition(topic string, partition int32, key string, value string, meta interface{}, successClb func(*client.ProducerMessage), errClb func(*client.ProducerError)) {
	conn.SendAsyncMessageToPartition(topic, partition, sarama.StringEncoder(key), sarama.StringEncoder(value), meta, successClb, errClb)
}

// Put publishes a message into kafka
func (p *bytesSyncPublisherKafka) Put(key string, data []byte) error {
	_, err := p.conn.SendSyncMessage(p.topic, sarama.StringEncoder(key), sarama.ByteEncoder(data))
	return err
}

// Put publishes a message into kafka
func (p *bytesAsyncPublisherKafka) Put(key string, data []byte) error {
	p.conn.SendAsyncMessage(p.topic, sarama.StringEncoder(key), sarama.ByteEncoder(data), nil, p.succCallback, p.errCallback)
	return nil
}

// Put publishes a message into kafka
func (p *bytesManualSyncPublisherKafka) Put(key string, data []byte) error {
	_, err := p.conn.SendSyncMessageToPartition(p.topic, p.partition, sarama.StringEncoder(key), sarama.ByteEncoder(data))
	return err
}

// Put publishes a message into kafka
func (p *bytesManualAsyncPublisherKafka) Put(key string, data []byte) error {
	p.conn.SendAsyncMessageToPartition(p.topic, p.partition, sarama.StringEncoder(key), sarama.ByteEncoder(data), nil, p.succCallback, p.errCallback)
	return nil
}

// MarkOffset marks the specified message as read
func (conn *BytesConnectionFields) MarkOffset(msg client.ConsumerMessage, metadata string) {
	if conn.multiplexer != nil && conn.multiplexer.Consumer != nil {
		conn.multiplexer.Consumer.MarkOffset(&msg, metadata)
	}
}

// CommitOffsets manually commits message offsets
func (conn *BytesConnectionFields) CommitOffsets() error {
	if conn.multiplexer != nil && conn.multiplexer.Consumer != nil {
		return conn.multiplexer.Consumer.CommitOffsets()
	}
	return fmt.Errorf("cannot commit offsets, consumer not available")
}
