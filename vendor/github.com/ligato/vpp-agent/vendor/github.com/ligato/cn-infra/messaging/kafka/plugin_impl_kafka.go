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

package kafka

import (
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/messaging"
	"github.com/ligato/cn-infra/messaging/kafka/client"
	"github.com/ligato/cn-infra/messaging/kafka/mux"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/cn-infra/utils/clienttls"
	"github.com/ligato/cn-infra/utils/safeclose"
)

const topic = "status-check"

// Plugin provides API for interaction with kafka brokers.
type Plugin struct {
	Deps

	mux          *mux.Multiplexer
	subscription chan *client.ConsumerMessage

	// Kafka plugin is using two clients. The first one is using 'hash' (default) partitioner. The second mux
	// uses manual partitioner which allows to send a message to specified partition and watching to desired partition/offset
	hsClient  sarama.Client
	manClient sarama.Client

	disabled bool
}

// Deps groups dependencies injected into the plugin so that they are
// logically separated from other plugin fields.
type Deps struct {
	infra.PluginDeps
	StatusCheck  statuscheck.PluginStatusWriter // inject
	ServiceLabel servicelabel.ReaderAPI
}

// FromExistingMux is used mainly for testing purposes.
func FromExistingMux(mux *mux.Multiplexer) *Plugin {
	return &Plugin{mux: mux}
}

// Init is called at plugin initialization.
func (p *Plugin) Init() (err error) {
	// Prepare topic and  subscription for status check client
	p.subscription = make(chan *client.ConsumerMessage)

	// Get muxCfg data (contains kafka brokers ip addresses)
	muxCfg := &mux.Config{}
	found, err := p.Cfg.LoadValue(muxCfg)
	if !found {
		p.Log.Info("kafka config not found ", p.Cfg.GetConfigName(), " - skip loading this plugin")
		p.disabled = true
		return nil //skip loading the plugin
	}
	if err != nil {
		return err
	}
	// retrieve clientCfg
	clientCfg, err := p.getClientConfig(muxCfg, p.Log, topic)
	if err != nil {
		return err
	}

	// init 'hash' sarama client
	p.hsClient, err = client.NewClient(clientCfg, client.Hash)
	if err != nil {
		return err
	}

	// init 'manual' sarama client
	p.manClient, err = client.NewClient(clientCfg, client.Manual)
	if err != nil {
		return err
	}

	// Initialize both multiplexers to allow both, dynamic and manual mode
	if p.mux == nil {
		name := clientCfg.GroupID
		p.Log.Infof("Group ID is set to %v", name)
		p.mux, err = mux.InitMultiplexerWithConfig(clientCfg, p.hsClient, p.manClient, name, p.Log)
		if err != nil {
			return err
		}
		p.Log.Debug("Default multiplexer initialized")
	}

	return err
}

// AfterInit is called in the second phase of the initialization. The kafka multiplexerNewWatcher
// is started, all consumers have to be subscribed until this phase.
func (p *Plugin) AfterInit() error {
	if p.disabled {
		p.Log.Debugf("kafka plugin disabled, skipping AfterInit")
		return nil
	}

	if p.mux != nil {
		err := p.mux.Start()
		if err != nil {
			return err
		}
	}

	// Register for providing status reports (polling mode)
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, func() (statuscheck.PluginState, error) {
			if p.hsClient == nil || p.hsClient.Closed() {
				return statuscheck.Error, fmt.Errorf("kafka client/consumer not available")
			}
			// Method 'RefreshMetadata()' returns error if kafka server is unavailable
			err := p.hsClient.RefreshMetadata(topic)
			if err == nil {
				return statuscheck.OK, nil
			}
			p.Log.Errorf("Kafka server unavailable")
			return statuscheck.Error, err
		})
	} else {
		p.Log.Warnf("Unable to start status check for kafka")
	}

	return nil
}

// Close is called at plugin cleanup phase.
func (p *Plugin) Close() error {
	return safeclose.Close(p.hsClient, p.manClient, p.mux)
}

// NewBytesConnection returns a new instance of a connection to access kafka brokers. The connection allows to create
// new kafka providers/consumers on multiplexer with hash partitioner.
func (p *Plugin) NewBytesConnection(name string) *mux.BytesConnectionStr {
	return p.mux.NewBytesConnection(name)
}

// NewBytesConnectionToPartition returns a new instance of a connection to access kafka brokers. The connection allows to create
// new kafka providers/consumers on multiplexer with manual partitioner which allows to send messages to specific partition
// in kafka cluster and watch on partition/offset.
func (p *Plugin) NewBytesConnectionToPartition(name string) *mux.BytesManualConnectionStr {
	return p.mux.NewBytesManualConnection(name)
}

// NewProtoConnection returns a new instance of a connection to access kafka brokers. The connection allows to create
// new kafka providers/consumers on multiplexer with hash partitioner.The connection uses proto-modelled messages.
func (p *Plugin) NewProtoConnection(name string) mux.Connection {
	return p.mux.NewProtoConnection(name, &keyval.SerializerJSON{})
}

// NewProtoManualConnection returns a new instance of a connection to access kafka brokers. The connection allows to create
// new kafka providers/consumers on multiplexer with manual partitioner which allows to send messages to specific partition
// in kafka cluster and watch on partition/offset. The connection uses proto-modelled messages.
func (p *Plugin) NewProtoManualConnection(name string) mux.ManualConnection {
	return p.mux.NewProtoManualConnection(name, &keyval.SerializerJSON{})
}

// NewSyncPublisher creates a publisher that allows to publish messages using synchronous API. The publisher creates
// new proto connection on multiplexer with default partitioner.
func (p *Plugin) NewSyncPublisher(connectionName string, topic string) (messaging.ProtoPublisher, error) {
	return p.NewProtoConnection(connectionName).NewSyncPublisher(topic)
}

// NewSyncPublisherToPartition creates a publisher that allows to publish messages to custom partition using synchronous API.
// The publisher creates new proto connection on multiplexer with manual partitioner.
func (p *Plugin) NewSyncPublisherToPartition(connectionName string, topic string, partition int32) (messaging.ProtoPublisher, error) {
	return p.NewProtoManualConnection(connectionName).NewSyncPublisherToPartition(topic, partition)
}

// NewAsyncPublisher creates a publisher that allows to publish messages using asynchronous API. The publisher creates
// new proto connection on multiplexer with default partitioner.
func (p *Plugin) NewAsyncPublisher(connectionName string, topic string, successClb func(messaging.ProtoMessage), errorClb func(messaging.ProtoMessageErr)) (messaging.ProtoPublisher, error) {
	return p.NewProtoConnection(connectionName).NewAsyncPublisher(topic, successClb, errorClb)
}

// NewAsyncPublisherToPartition creates a publisher that allows to publish messages to custom partition using asynchronous API.
// The publisher creates new proto connection on multiplexer with manual partitioner.
func (p *Plugin) NewAsyncPublisherToPartition(connectionName string, topic string, partition int32, successClb func(messaging.ProtoMessage), errorClb func(messaging.ProtoMessageErr)) (messaging.ProtoPublisher, error) {
	return p.NewProtoManualConnection(connectionName).NewAsyncPublisherToPartition(topic, partition, successClb, errorClb)
}

// NewWatcher creates a watcher that allows to start/stop consuming of messaging published to given topics.
func (p *Plugin) NewWatcher(name string) messaging.ProtoWatcher {
	return p.NewProtoConnection(name)
}

// NewPartitionWatcher creates a watcher that allows to start/stop consuming of messaging published to given topics, offset and partition
func (p *Plugin) NewPartitionWatcher(name string) messaging.ProtoPartitionWatcher {
	return p.NewProtoManualConnection(name)
}

// Disabled if the plugin config was not found
func (p *Plugin) Disabled() (disabled bool) {
	return p.disabled
}

// Receive client config according to kafka config data
func (p *Plugin) getClientConfig(config *mux.Config, logger logging.Logger, topic string) (*client.Config, error) {
	clientCfg := client.NewConfig(logger)
	// Set brokers obtained from kafka config. In case there are none available, use a default one
	if len(config.Addrs) > 0 {
		clientCfg.SetBrokers(config.Addrs...)
	} else {
		clientCfg.SetBrokers(mux.DefAddress)
	}
	// Set group ID obtained from kafka config. In case there is none, use a service label
	if config.GroupID != "" {
		clientCfg.SetGroup(config.GroupID)
	} else {
		clientCfg.SetGroup(p.ServiceLabel.GetAgentLabel())
	}
	clientCfg.SetRecvMessageChan(p.subscription)
	clientCfg.SetInitialOffset(sarama.OffsetNewest)
	clientCfg.SetTopics(topic)
	if config.TLS.Enabled {
		p.Log.Info("TLS enabled")
		tlsConfig, err := clienttls.CreateTLSConfig(config.TLS)
		if err != nil {
			return nil, err
		}
		clientCfg.SetTLS(tlsConfig)
	}
	return clientCfg, nil
}
