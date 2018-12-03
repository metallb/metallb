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

package messaging

import (
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
)

// Mux defines API for the plugins that use access to kafka brokers.
type Mux interface {
	// Creates new Kafka synchronous publisher sending messages to given topic.
	// Partitioner has to be set to 'hash' (default) or 'random' scheme,
	// otherwise an error is thrown.
	NewSyncPublisher(connName string, topic string) (ProtoPublisher, error)

	// Creates new Kafka synchronous publisher sending messages to given topic
	// and partition. Partitioner has to be set to 'manual' scheme,
	// otherwise an error is thrown.
	NewSyncPublisherToPartition(connName string, topic string, partition int32) (ProtoPublisher, error)

	// Creates new Kafka asynchronous publisher sending messages to given topic.
	// Partitioner has to be set to 'hash' (default) or 'random' scheme,
	// otherwise an error is thrown.
	NewAsyncPublisher(connName string, topic string, successClb func(ProtoMessage), errorClb func(err ProtoMessageErr)) (ProtoPublisher, error)

	// Creates new Kafka asynchronous publisher sending messages to given topic
	// and partition. Partitioner has to be set to 'manual' scheme,
	// otherwise an error is thrown.
	NewAsyncPublisherToPartition(connName string, topic string, partition int32,
		successClb func(ProtoMessage), errorClb func(err ProtoMessageErr)) (ProtoPublisher, error)

	// Initializes new watcher which can start/stop watching on topic,
	NewWatcher(subscriberName string) ProtoWatcher

	// Initializes new watcher which can start/stop watching on topic,
	// eventually partition and offset.
	NewPartitionWatcher(subscriberName string) ProtoPartitionWatcher

	// Disabled if the plugin config was not found.
	Disabled() (disabled bool)
}

// ProtoPublisher allows to publish a message of type proto.Message into
// messaging system.
type ProtoPublisher interface {
	datasync.KeyProtoValWriter
}

// ProtoWatcher allows to subscribe for receiving of messages published
// to selected topics.
type ProtoWatcher interface {
	OffsetHandler
	// Watch starts consuming all selected <topics>.
	// Returns error if 'manual' partitioner scheme is chosen
	// Callback <msgCallback> is called for each delivered message.
	Watch(msgCallback func(ProtoMessage), topics ...string) error

	// StopWatch cancels the previously created subscription for consuming
	// a given <topic>.
	StopWatch(topic string) error
}

// ProtoPartitionWatcher allows to subscribe for receiving of messages published
// to selected topics, partitions and offsets
type ProtoPartitionWatcher interface {
	OffsetHandler
	// WatchPartition starts consuming specific <partition> of a selected <topic>
	// from a given <offset>. Offset is the oldest message index consumed,
	// all previously published messages are ignored.
	// Callback <msgCallback> is called for each delivered message.
	WatchPartition(msgCallback func(ProtoMessage), topic string, partition int32, offset int64) error

	// StopWatchPartition cancels the previously created subscription
	// for consuming a given <topic>/<partition>/<offset>.
	// Return error if such a combination is not subscribed
	StopWatchPartition(topic string, partition int32, offset int64) error
}

// ProtoMessage exposes parameters of a single message received from messaging
// system.
type ProtoMessage interface {
	keyval.ProtoKvPair

	// GetTopic returns the name of the topic from which the message
	// was consumed.
	GetTopic() string

	// GetTopic returns the index of the partition from which the message
	// was consumed.
	GetPartition() int32
	GetOffset() int64
}

// ProtoMessageErr represents a message that was not published successfully
// to a messaging system.
type ProtoMessageErr interface {
	ProtoMessage

	// Error returns an error instance describing the cause of the failed
	// delivery.
	Error() error
}

// OffsetHandler allows to mark offset or commit
type OffsetHandler interface {
	// MarkOffset marks the message received by a consumer as processed.
	MarkOffset(msg ProtoMessage, metadata string)
	// CommitOffsets manually commits marked offsets.
	CommitOffsets() error
}
