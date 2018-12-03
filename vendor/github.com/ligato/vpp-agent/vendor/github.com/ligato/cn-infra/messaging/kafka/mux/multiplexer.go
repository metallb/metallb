package mux

import (
	"fmt"
	"sync"

	"github.com/Shopify/sarama"

	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/messaging/kafka/client"
	"github.com/ligato/cn-infra/utils/safeclose"
)

// Multiplexer encapsulates clients to kafka cluster (SyncProducer, AsyncProducer (both of them
// with 'hash' and 'manual' partitioner), consumer). It allows to create multiple Connections
// that use multiplexer's clients for communication with kafka cluster. The aim of Multiplexer
// is to decrease the number of connections needed. The set of topics to be consumed by
// Connections needs to be selected before the underlying consumer in Multiplexer is started.
// Once the Multiplexer's consumer has been started new topics can not be added.
type Multiplexer struct {
	logging.Logger

	// consumer used by the Multiplexer (bsm/sarama cluster)
	Consumer *client.Consumer

	// producers available for this mux
	multiplexerProducers

	// client config
	config *client.Config

	// name is used for identification of stored last consumed offset in kafka. This allows
	// to follow up messages after restart.
	name string

	// guards access to mapping and started flag
	rwlock sync.RWMutex

	// started denotes whether the multiplexer is dispatching the messages or accepting subscriptions to
	// consume a topic. Once the multiplexer is started, new subscription can not be added.
	started bool

	// Mapping provides the mapping of subscribed consumers. Subscription contains topic, partition and offset to consume,
	// as well as dynamic/manual mode flag
	mapping []*consumerSubscription

	// factory that crates Consumer used in the Multiplexer
	consumerFactory func(topics []string, groupId string) (*client.Consumer, error)
}

// ConsumerSubscription contains all information about subscribed kafka consumer/watcher
type consumerSubscription struct {
	// in manual mode, multiplexer is distributing messages according to topic, partition and offset. If manual
	// mode is off, messages are distributed using topic only
	manual bool
	// topic to watch on
	topic string
	// partition to watch on in manual mode
	partition int32
	// partition consumer created only in manual mode. Its value is stored in subscription (after all required handlers
	// are started) in order to be properly closed if required
	partitionConsumer *sarama.PartitionConsumer
	// offset to watch on in manual mode
	offset int64
	// name identifies the connection
	connectionName string
	// sends message to subscribed channel
	byteConsMsg func(*client.ConsumerMessage)
}

// asyncMeta is auxiliary structure used by Multiplexer to distribute consumer messages
type asyncMeta struct {
	successClb func(*client.ProducerMessage)
	errorClb   func(error *client.ProducerError)
	usersMeta  interface{}
}

// multiplexerProducers groups all mux producers
type multiplexerProducers struct {
	// hashSyncProducer with hash partitioner used by the Multiplexer
	hashSyncProducer *client.SyncProducer
	// manSyncProducer with manual partitioner used by the Multiplexer
	manSyncProducer *client.SyncProducer
	// hashAsyncProducer with hash used by the Multiplexer
	hashAsyncProducer *client.AsyncProducer
	// manAsyncProducer with manual used by the Multiplexer
	manAsyncProducer *client.AsyncProducer
}

// NewMultiplexer creates new instance of Kafka Multiplexer
func NewMultiplexer(consumerFactory ConsumerFactory, producers multiplexerProducers, clientCfg *client.Config,
	name string, log logging.Logger) *Multiplexer {
	if clientCfg.Logger == nil {
		clientCfg.Logger = log
	}
	cl := &Multiplexer{consumerFactory: consumerFactory,
		Logger:               log,
		name:                 name,
		mapping:              []*consumerSubscription{},
		multiplexerProducers: producers,
		config:               clientCfg,
	}

	go cl.watchAsyncProducerChannels()
	if producers.manAsyncProducer != nil && producers.manAsyncProducer.Config != nil {
		go cl.watchManualAsyncProducerChannels()
	}
	return cl
}

func (mux *Multiplexer) watchAsyncProducerChannels() {
	for {
		select {
		case err := <-mux.hashAsyncProducer.Config.ErrorChan:
			mux.Println("AsyncProducer (hash): failed to produce message", err.Err)
			errMsg := err.ProducerMessage

			if errMeta, ok := errMsg.Metadata.(*asyncMeta); ok && errMeta.errorClb != nil {
				err.ProducerMessage.Metadata = errMeta.usersMeta
				errMeta.errorClb(err)
			}
		case success := <-mux.hashAsyncProducer.Config.SuccessChan:

			if succMeta, ok := success.Metadata.(*asyncMeta); ok && succMeta.successClb != nil {
				success.Metadata = succMeta.usersMeta
				succMeta.successClb(success)
			}
		case <-mux.hashAsyncProducer.GetCloseChannel():
			mux.Debug("AsyncProducer (hash): closing watch loop")
		}
	}
}

func (mux *Multiplexer) watchManualAsyncProducerChannels() {
	for {
		select {
		case err := <-mux.manAsyncProducer.Config.ErrorChan:
			mux.Println("AsyncProducer (manual): failed to produce message", err.Err)
			errMsg := err.ProducerMessage

			if errMeta, ok := errMsg.Metadata.(*asyncMeta); ok && errMeta.errorClb != nil {
				err.ProducerMessage.Metadata = errMeta.usersMeta
				errMeta.errorClb(err)
			}
		case success := <-mux.manAsyncProducer.Config.SuccessChan:

			if succMeta, ok := success.Metadata.(*asyncMeta); ok && succMeta.successClb != nil {
				success.Metadata = succMeta.usersMeta
				succMeta.successClb(success)
			}
		case <-mux.manAsyncProducer.GetCloseChannel():
			mux.Debug("AsyncProducer (manual): closing watch loop")
		}

	}
}

// Start should be called once all the Connections have been subscribed
// for topic consumption. An attempt to start consuming a topic after the multiplexer is started
// returns an error.
func (mux *Multiplexer) Start() error {
	mux.rwlock.Lock()
	defer mux.rwlock.Unlock()
	var err error

	if mux.started {
		return fmt.Errorf("multiplexer has been started already")
	}

	// block further Consumer consumers
	mux.started = true

	var hashTopics, manTopics []string

	for _, subscription := range mux.mapping {
		if subscription.manual {
			manTopics = append(manTopics, subscription.topic)
			continue
		}
		hashTopics = append(hashTopics, subscription.topic)
	}

	mux.config.SetRecvMessageChan(make(chan *client.ConsumerMessage))
	mux.config.GroupID = mux.name
	mux.config.SetInitialOffset(sarama.OffsetOldest)
	mux.config.Topics = append(hashTopics, manTopics...)

	// create consumer
	mux.WithFields(logging.Fields{"hashTopics": hashTopics, "manualTopics": manTopics}).Debugf("Consuming started")
	mux.Consumer, err = client.NewConsumer(mux.config, nil)
	if err != nil {
		return err
	}

	if len(hashTopics) == 0 {
		mux.Debug("No topics for hash partitioner")
	} else {
		mux.WithFields(logging.Fields{"topics": hashTopics}).Debugf("Consuming (hash) started")
		mux.Consumer.StartConsumerHandlers()
	}

	if len(manTopics) == 0 {
		mux.Debug("No topics for manual partitioner")
	} else {
		mux.WithFields(logging.Fields{"topics": manTopics}).Debugf("Consuming (manual) started")
		for _, sub := range mux.mapping {
			if sub.manual {
				sConsumer := mux.Consumer.SConsumer
				if sConsumer == nil {
					return fmt.Errorf("consumer for manual partition is not available")
				}
				partitionConsumer, err := sConsumer.ConsumePartition(sub.topic, sub.partition, sub.offset)
				if err != nil {
					return err
				}
				// Store partition consumer in subscription so it can be closed lately
				sub.partitionConsumer = &partitionConsumer
				mux.Logger.WithFields(logging.Fields{"topic": sub.topic, "partition": sub.partition, "offset": sub.offset}).Info("Partition sConsumer started")
				mux.Consumer.StartConsumerManualHandlers(partitionConsumer)
			}
		}

	}

	go mux.genericConsumer()
	go mux.manualConsumer(mux.Consumer)

	return err
}

// Close cleans up the resources used by the Multiplexer
func (mux *Multiplexer) Close() {
	safeclose.Close(
		mux.Consumer,
		mux.hashSyncProducer,
		mux.hashAsyncProducer,
		mux.manSyncProducer,
		mux.manAsyncProducer)
}

// NewBytesConnection creates instance of the BytesConnectionStr that provides access to shared
// Multiplexer's clients with hash partitioner.
func (mux *Multiplexer) NewBytesConnection(name string) *BytesConnectionStr {
	return &BytesConnectionStr{BytesConnectionFields{multiplexer: mux, name: name}}
}

// NewBytesManualConnection creates instance of the BytesManualConnectionStr that provides access to shared
// Multiplexer's clients with manual partitioner.
func (mux *Multiplexer) NewBytesManualConnection(name string) *BytesManualConnectionStr {
	return &BytesManualConnectionStr{BytesConnectionFields{multiplexer: mux, name: name}}
}

// NewProtoConnection creates instance of the ProtoConnection that provides access to shared
// Multiplexer's clients with hash partitioner.
func (mux *Multiplexer) NewProtoConnection(name string, serializer keyval.Serializer) *ProtoConnection {
	return &ProtoConnection{ProtoConnectionFields{multiplexer: mux, serializer: serializer, name: name}}
}

// NewProtoManualConnection creates instance of the ProtoConnectionFields that provides access to shared
// Multiplexer's clients with manual partitioner.
func (mux *Multiplexer) NewProtoManualConnection(name string, serializer keyval.Serializer) *ProtoManualConnection {
	return &ProtoManualConnection{ProtoConnectionFields{multiplexer: mux, serializer: serializer, name: name}}
}

// Propagates incoming messages to respective channels.
func (mux *Multiplexer) propagateMessage(msg *client.ConsumerMessage) {
	mux.rwlock.RLock()
	defer mux.rwlock.RUnlock()

	if msg == nil {
		return
	}

	// Find subscribed topics. Note: topic can be subscribed for both dynamic and manual consuming
	for _, subscription := range mux.mapping {
		if msg.Topic == subscription.topic {
			// Clustered mode - message is consumed only on right partition and offset
			if subscription.manual {
				if msg.Partition == subscription.partition && msg.Offset >= subscription.offset {
					mux.Debug("offset ", msg.Offset, string(msg.Value), string(msg.Key), msg.Partition)
					subscription.byteConsMsg(msg)
				}
			} else {
				// Non-manual mode
				// if we are not able to write into the channel we should skip the receiver
				// and report an error to avoid deadlock
				mux.Debug("offset ", msg.Offset, string(msg.Value), string(msg.Key), msg.Partition)
				subscription.byteConsMsg(msg)
			}
		}
	}
}

// genericConsumer handles incoming messages to the multiplexer and distributes them among the subscribers.
func (mux *Multiplexer) genericConsumer() {
	mux.Debug("Generic Consumer started")
	for {
		select {
		case <-mux.Consumer.GetCloseChannel():
			mux.Debug("Closing Consumer")
			return
		case msg := <-mux.Consumer.Config.RecvMessageChan:
			// 'hash' partitioner messages will be marked
			mux.propagateMessage(msg)
		case err := <-mux.Consumer.Config.RecvErrorChan:
			mux.Error("Received partitionConsumer error ", err)
		}
	}
}

// manualConsumer takes a consumer (even a post-init created) and handles incoming messages for them.
func (mux *Multiplexer) manualConsumer(consumer *client.Consumer) {
	mux.Debug("Generic Consumer started")
	for {
		select {
		case <-consumer.GetCloseChannel():
			mux.Debug("Closing Consumer")
			return
		case msg := <-consumer.Config.RecvMessageChan:
			mux.Debug("Kafka message received")
			// 'later-stage' Consumer does not consume 'hash' messages, none of them is marked
			mux.propagateMessage(msg)
		case err := <-consumer.Config.RecvErrorChan:
			mux.Error("Received partitionConsumer error ", err)
		}
	}
}

// Remove consumer subscription on given topic. If there is no such a subscription, return error.
func (mux *Multiplexer) stopConsuming(topic string, name string) error {
	mux.rwlock.Lock()
	defer mux.rwlock.Unlock()

	var wasError error
	var topicFound bool
	for index, subs := range mux.mapping {
		if !subs.manual && subs.topic == topic && subs.connectionName == name {
			topicFound = true
			mux.mapping = append(mux.mapping[:index], mux.mapping[index+1:]...)
		}
	}
	if !topicFound {
		wasError = fmt.Errorf("topic %s was not consumed by '%s'", topic, name)
	}
	return wasError
}

// Remove consumer subscription on given topic, partition and initial offset. If there is no such a subscription
// (all fields must match), return error.
func (mux *Multiplexer) stopConsumingPartition(topic string, partition int32, offset int64, name string) error {
	mux.rwlock.Lock()
	defer mux.rwlock.Unlock()

	var wasError error
	var topicFound bool
	// Remove consumer from subscription
	for index, subs := range mux.mapping {
		if subs.manual && subs.topic == topic && subs.partition == partition && subs.offset == offset && subs.connectionName == name {
			topicFound = true
			mux.mapping = append(mux.mapping[:index], mux.mapping[index+1:]...)
		}
		// Close the partition consumer related to the subscription
		safeclose.Close(subs.partitionConsumer)
	}
	if !topicFound {
		wasError = fmt.Errorf("topic %s, partition %v and offset %v was not consumed by '%s'",
			topic, partition, offset, name)
	}
	// Stop partition consumer
	return wasError
}
