package mux

import (
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/messaging"
	"github.com/ligato/cn-infra/messaging/kafka/client"
)

// Connection is interface for multiplexer with dynamic partitioner.
type Connection interface {
	messaging.ProtoWatcher
	// Creates new synchronous publisher allowing to publish kafka messages
	NewSyncPublisher(topic string) (messaging.ProtoPublisher, error)
	// Creates new asynchronous publisher allowing to publish kafka messages
	NewAsyncPublisher(topic string, successClb func(messaging.ProtoMessage), errorClb func(messaging.ProtoMessageErr)) (messaging.ProtoPublisher, error)
}

// ManualConnection is interface for multiplexer with manual partitioner.
type ManualConnection interface {
	messaging.ProtoPartitionWatcher
	// Creates new synchronous publisher allowing to publish kafka messages to chosen partition
	NewSyncPublisherToPartition(topic string, partition int32) (messaging.ProtoPublisher, error)
	// Creates new asynchronous publisher allowing to publish kafka messages to chosen partition
	NewAsyncPublisherToPartition(topic string, partition int32, successClb func(messaging.ProtoMessage), errorClb func(messaging.ProtoMessageErr)) (messaging.ProtoPublisher, error)
}

// ProtoConnection represents connection built on hash-mode multiplexer
type ProtoConnection struct {
	ProtoConnectionFields
}

// ProtoManualConnection represents connection built on manual-mode multiplexer
type ProtoManualConnection struct {
	ProtoConnectionFields
}

// ProtoConnectionFields is an entity that provides access to shared producers/consumers of multiplexer. The value of
// message are marshaled and unmarshaled to/from proto.message behind the scene.
type ProtoConnectionFields struct {
	// multiplexer is used for access to kafka brokers
	multiplexer *Multiplexer
	// name identifies the connection
	name string
	// serializer marshals and unmarshals data to/from proto.Message
	serializer keyval.Serializer
}

type protoSyncPublisherKafka struct {
	conn  *ProtoConnection
	topic string
}

type protoAsyncPublisherKafka struct {
	conn         *ProtoConnection
	topic        string
	succCallback func(messaging.ProtoMessage)
	errCallback  func(messaging.ProtoMessageErr)
}

type protoManualSyncPublisherKafka struct {
	conn      *ProtoManualConnection
	topic     string
	partition int32
}

type protoManualAsyncPublisherKafka struct {
	conn         *ProtoManualConnection
	topic        string
	partition    int32
	succCallback func(messaging.ProtoMessage)
	errCallback  func(messaging.ProtoMessageErr)
}

// NewSyncPublisher creates a new instance of protoSyncPublisherKafka that allows to publish sync kafka messages using common messaging API
func (conn *ProtoConnection) NewSyncPublisher(topic string) (messaging.ProtoPublisher, error) {
	return &protoSyncPublisherKafka{conn, topic}, nil
}

// NewAsyncPublisher creates a new instance of protoAsyncPublisherKafka that allows to publish sync kafka messages using common messaging API
func (conn *ProtoConnection) NewAsyncPublisher(topic string, successClb func(messaging.ProtoMessage), errorClb func(messaging.ProtoMessageErr)) (messaging.ProtoPublisher, error) {
	return &protoAsyncPublisherKafka{conn, topic, successClb, errorClb}, nil
}

// NewSyncPublisherToPartition creates a new instance of protoManualSyncPublisherKafka that allows to publish sync kafka messages using common messaging API
func (conn *ProtoManualConnection) NewSyncPublisherToPartition(topic string, partition int32) (messaging.ProtoPublisher, error) {
	return &protoManualSyncPublisherKafka{conn, topic, partition}, nil
}

// NewAsyncPublisherToPartition creates a new instance of protoManualAsyncPublisherKafka that allows to publish sync kafka
// messages using common messaging API.
func (conn *ProtoManualConnection) NewAsyncPublisherToPartition(topic string, partition int32, successClb func(messaging.ProtoMessage), errorClb func(messaging.ProtoMessageErr)) (messaging.ProtoPublisher, error) {
	return &protoManualAsyncPublisherKafka{conn, topic, partition, successClb, errorClb}, nil
}

// Watch is an alias for ConsumeTopic method. The alias was added in order to conform to messaging.Mux interface.
func (conn *ProtoConnection) Watch(msgClb func(messaging.ProtoMessage), topics ...string) error {
	return conn.ConsumeTopic(msgClb, topics...)
}

// ConsumeTopic is called to start consuming given topics.
// Function can be called until the multiplexer is started, it returns an error otherwise.
// The provided channel should be buffered, otherwise messages might be lost.
func (conn *ProtoConnection) ConsumeTopic(msgClb func(messaging.ProtoMessage), topics ...string) error {
	conn.multiplexer.rwlock.Lock()
	defer conn.multiplexer.rwlock.Unlock()

	if conn.multiplexer.started {
		return fmt.Errorf("ConsumeTopic can be called only if the multiplexer has not been started yet")
	}

	byteClb := func(bm *client.ConsumerMessage) {
		pm := client.NewProtoConsumerMessage(bm, conn.serializer)
		msgClb(pm)
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
				byteConsMsg:    byteClb,
			}
			// subscribe new topic
			conn.multiplexer.mapping = append(conn.multiplexer.mapping, subs)
		}

		// add subscription to consumerList
		subs.byteConsMsg = byteClb
	}

	return nil
}

// WatchPartition is an alias for ConsumePartition method. The alias was added in order to conform to
// messaging.Mux interface.
func (conn *ProtoManualConnection) WatchPartition(msgClb func(messaging.ProtoMessage), topic string, partition int32, offset int64) error {
	return conn.ConsumePartition(msgClb, topic, partition, offset)
}

// ConsumePartition is called to start consuming given topic on partition with offset
// Function can be called until the multiplexer is started, it returns an error otherwise.
// The provided channel should be buffered, otherwise messages might be lost.
func (conn *ProtoManualConnection) ConsumePartition(msgClb func(messaging.ProtoMessage), topic string, partition int32, offset int64) error {
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

	byteClb := func(bm *client.ConsumerMessage) {
		pm := client.NewProtoConsumerMessage(bm, conn.serializer)
		msgClb(pm)
	}

	if !found {
		subs = &consumerSubscription{
			manual:         true, // manual example
			topic:          topic,
			partition:      partition,
			offset:         offset,
			connectionName: conn.name,
			byteConsMsg:    byteClb,
		}
		// subscribe new topic on partition
		conn.multiplexer.mapping = append(conn.multiplexer.mapping, subs)
	}

	// add subscription to consumerList
	subs.byteConsMsg = byteClb

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

// StartPostInitConsumer allows to start a new partition consumer after mux is initialized. Created partition consumer
// is returned so it can be stored in subscription and closed if needed
func (conn *ProtoManualConnection) StartPostInitConsumer(topic string, partition int32, offset int64) (*sarama.PartitionConsumer, error) {
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

// StopWatch is an alias for StopConsuming method. The alias was added in order to conform to messaging.Mux interface.
func (conn *ProtoConnectionFields) StopWatch(topic string) error {
	return conn.StopConsuming(topic)
}

// StopConsuming cancels the previously created subscription for consuming the topic.
func (conn *ProtoConnectionFields) StopConsuming(topic string) error {
	return conn.multiplexer.stopConsuming(topic, conn.name)
}

// StopWatchPartition is an alias for StopConsumingPartition method. The alias was added in order to conform to messaging.Mux interface.
func (conn *ProtoConnectionFields) StopWatchPartition(topic string, partition int32, offset int64) error {
	return conn.StopConsumingPartition(topic, partition, offset)
}

// StopConsumingPartition cancels the previously created subscription for consuming the topic, partition and offset
func (conn *ProtoConnectionFields) StopConsumingPartition(topic string, partition int32, offset int64) error {
	return conn.multiplexer.stopConsumingPartition(topic, partition, offset, conn.name)
}

// Put publishes a message into kafka
func (p *protoSyncPublisherKafka) Put(key string, message proto.Message, opts ...datasync.PutOption) error {
	_, err := p.conn.sendSyncMessage(p.topic, DefPartition, key, message, false)
	return err
}

// Put publishes a message into kafka
func (p *protoAsyncPublisherKafka) Put(key string, message proto.Message, opts ...datasync.PutOption) error {
	return p.conn.sendAsyncMessage(p.topic, DefPartition, key, message, false, nil, p.succCallback, p.errCallback)
}

// Put publishes a message into kafka
func (p *protoManualSyncPublisherKafka) Put(key string, message proto.Message, opts ...datasync.PutOption) error {
	_, err := p.conn.sendSyncMessage(p.topic, p.partition, key, message, true)
	return err
}

// Put publishes a message into kafka
func (p *protoManualAsyncPublisherKafka) Put(key string, message proto.Message, opts ...datasync.PutOption) error {
	return p.conn.sendAsyncMessage(p.topic, p.partition, key, message, true, nil, p.succCallback, p.errCallback)
}

// MarkOffset marks the specified message as read
func (conn *ProtoConnectionFields) MarkOffset(msg messaging.ProtoMessage, metadata string) {
	if conn.multiplexer != nil && conn.multiplexer.Consumer != nil {
		if msg == nil {
			return
		}
		consumerMsg := &client.ConsumerMessage{
			Topic:     msg.GetTopic(),
			Partition: msg.GetPartition(),
			Offset:    msg.GetOffset(),
		}

		conn.multiplexer.Consumer.MarkOffset(consumerMsg, metadata)
	}
}

// CommitOffsets manually commits message offsets
func (conn *ProtoConnectionFields) CommitOffsets() error {
	if conn.multiplexer != nil && conn.multiplexer.Consumer != nil {
		return conn.multiplexer.Consumer.CommitOffsets()
	}
	return fmt.Errorf("cannot commit offsets, consumer not available")
}

// sendSyncMessage sends a message using the sync API. If manual mode is chosen, the appropriate producer will be used.
func (conn *ProtoConnectionFields) sendSyncMessage(topic string, partition int32, key string, value proto.Message, manualMode bool) (offset int64, err error) {
	data, err := conn.serializer.Marshal(value)
	if err != nil {
		return 0, err
	}

	if manualMode {
		msg, err := conn.multiplexer.manSyncProducer.SendMsgToPartition(topic, partition, sarama.StringEncoder(key), sarama.ByteEncoder(data))
		if err != nil {
			return 0, err
		}
		return msg.Offset, err
	}
	msg, err := conn.multiplexer.hashSyncProducer.SendMsgToPartition(topic, partition, sarama.StringEncoder(key), sarama.ByteEncoder(data))
	if err != nil {
		return 0, err
	}
	return msg.Offset, err
}

// sendAsyncMessage sends a message using the async API. If manual mode is chosen, the appropriate producer will be used.
func (conn *ProtoConnectionFields) sendAsyncMessage(topic string, partition int32, key string, value proto.Message, manualMode bool,
	meta interface{}, successClb func(messaging.ProtoMessage), errClb func(messaging.ProtoMessageErr)) error {
	data, err := conn.serializer.Marshal(value)
	if err != nil {
		return err
	}
	succByteClb := func(msg *client.ProducerMessage) {
		protoMsg := &client.ProtoProducerMessage{
			ProducerMessage: msg,
			Serializer:      conn.serializer,
		}
		successClb(protoMsg)
	}

	errByteClb := func(msg *client.ProducerError) {
		protoMsg := &client.ProtoProducerMessageErr{
			ProtoProducerMessage: &client.ProtoProducerMessage{
				ProducerMessage: msg.ProducerMessage,
				Serializer:      conn.serializer,
			},
			Err: msg.Err,
		}
		errClb(protoMsg)
	}

	if manualMode {
		auxMeta := &asyncMeta{successClb: succByteClb, errorClb: errByteClb, usersMeta: meta}
		conn.multiplexer.manAsyncProducer.SendMsgToPartition(topic, partition, sarama.StringEncoder(key), sarama.ByteEncoder(data), auxMeta)
		return nil
	}
	auxMeta := &asyncMeta{successClb: succByteClb, errorClb: errByteClb, usersMeta: meta}
	conn.multiplexer.hashAsyncProducer.SendMsgToPartition(topic, partition, sarama.StringEncoder(key), sarama.ByteEncoder(data), auxMeta)
	return nil
}
