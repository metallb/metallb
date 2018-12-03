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

package redis

import (
	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
)

const (
	// healthCheckProbeKey is a key used to probe Redis state.
	healthCheckProbeKey = "probe-redis-connection"
)

// Plugin implements redis plugin.
type Plugin struct {
	Deps

	// Plugin is disabled if there is no config file available
	disabled bool
	// Redis connection encapsulation
	connection *BytesConnectionRedis
	// Read/Write proto modelled data
	protoWrapper *kvproto.ProtoWrapper
}

// Deps lists dependencies of the redis plugin.
type Deps struct {
	infra.PluginDeps
	StatusCheck statuscheck.PluginStatusWriter
	Resync      *resync.Plugin // inject
}

// Init retrieves redis configuration and establishes a new connection
// with the redis data store.
// If the configuration file doesn't exist or cannot be read, the returned errora
// will be of os.PathError type. An untyped error is returned in case the file
// doesn't contain a valid YAML configuration.
func (p *Plugin) Init() (err error) {
	redisCfg, err := p.getRedisConfig()
	if err != nil || p.disabled {
		return err
	}

	// Create client according to config
	client, err := ConfigToClient(redisCfg)
	if err != nil {
		return err
	}

	// Uses config file to establish connection with the database
	p.connection, err = NewBytesConnection(client, p.Log)
	if err != nil {
		return err
	}
	p.protoWrapper = kvproto.NewProtoWrapper(p.connection, &keyval.SerializerJSON{})

	// Register for providing status reports (polling mode)
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, func() (statuscheck.PluginState, error) {
			_, _, err := p.NewBroker("/").GetValue(healthCheckProbeKey, nil)
			if err == nil {
				return statuscheck.OK, nil
			}
			return statuscheck.Error, err
		})
	} else {
		p.Log.Warnf("Unable to start status check for redis")
	}

	return nil
}

// Close does nothing for redis plugin.
func (p *Plugin) Close() error {
	return nil
}

// NewBroker creates new instance of prefixed broker that provides API with arguments of type proto.Message.
func (p *Plugin) NewBroker(keyPrefix string) keyval.ProtoBroker {
	return p.protoWrapper.NewBroker(keyPrefix)
}

// NewWatcher creates new instance of prefixed broker that provides API with arguments of type proto.Message.
func (p *Plugin) NewWatcher(keyPrefix string) keyval.ProtoWatcher {
	return p.protoWrapper.NewWatcher(keyPrefix)
}

// Disabled returns *true* if the plugin is not in use due to missing
// redis configuration.
func (p *Plugin) Disabled() (disabled bool) {
	return p.disabled
}

// OnConnect executes callback from datasync
func (p *Plugin) OnConnect(callback func() error) {
	if err := callback(); err != nil {
		p.Log.Error(err)
	}
}

func (p *Plugin) getRedisConfig() (cfg interface{}, err error) {
	found, _ := p.Cfg.LoadValue(&struct{}{})
	if !found {
		p.Log.Info("Redis config not found, skip loading this plugin")
		p.disabled = true
		return
	}
	configFile := p.Cfg.GetConfigName()
	if configFile != "" {
		cfg, err = LoadConfig(configFile)
		if err != nil {
			return
		}
	}
	return
}
