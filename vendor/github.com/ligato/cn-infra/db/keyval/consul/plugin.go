//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package consul

import (
	"github.com/hashicorp/consul/api"
	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
)

const (
	// healthCheckProbeKey is a key used to probe connection state
	healthCheckProbeKey = "/probe-consul-connection"
)

// Config represents configuration for Consul plugin.
type Config struct {
	Address         string `json:"address"`
	ReconnectResync bool   `json:"resync-after-reconnect"`
}

// Plugin implements Consul as plugin.
type Plugin struct {
	Deps

	*Config
	// Plugin is disabled if there is no config file available
	disabled bool
	// Consul client encapsulation
	client *Client
	// Read/Write proto modelled data
	protoWrapper *kvproto.ProtoWrapper

	reconnectResync bool
	lastConnErr     error
}

// Deps lists dependencies of the Consul plugin.
// If injected, Consul plugin will use StatusCheck to signal the connection status.
type Deps struct {
	infra.PluginDeps
	StatusCheck statuscheck.PluginStatusWriter
	Resync      *resync.Plugin
}

// Init initializes Consul plugin.
func (p *Plugin) Init() (err error) {
	if p.Config == nil {
		p.Config, err = p.getConfig()
		if err != nil || p.disabled {
			return err
		}
	}

	clientCfg, err := ConfigToClient(p.Config)
	if err != nil {
		return err
	}
	p.client, err = NewClient(clientCfg)
	if err != nil {
		p.Log.Errorf("Err: %v", err)
		return err
	}

	p.reconnectResync = p.Config.ReconnectResync
	p.protoWrapper = kvproto.NewProtoWrapper(p.client, &keyval.SerializerJSON{})

	// Register for providing status reports (polling mode)
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, p.statusCheckProbe)
	} else {
		p.Log.Warnf("Unable to start status check for consul")
	}

	return nil
}

func (p *Plugin) statusCheckProbe() (statuscheck.PluginState, error) {
	_, _, _, err := p.client.GetValue(healthCheckProbeKey)
	if err != nil {
		p.lastConnErr = err
		return statuscheck.Error, err
	}

	if p.reconnectResync && p.lastConnErr != nil {
		p.Log.Info("Starting resync after Consul reconnect")
		if p.Resync != nil {
			p.Resync.DoResync()
			p.lastConnErr = nil
		} else {
			p.Log.Warn("Expected resync after Consul reconnect could not start beacuse of missing Resync plugin")
		}
	}

	return statuscheck.OK, nil
}

// OnConnect executes callback from datasync
func (p *Plugin) OnConnect(callback func() error) {
	if err := callback(); err != nil {
		p.Log.Error(err)
	}
}

// Close closes Consul plugin.
func (p *Plugin) Close() error {
	return nil
}

// Disabled returns *true* if the plugin is not in use due to missing configuration.
func (p *Plugin) Disabled() bool {
	return p.disabled
}

func (p *Plugin) getConfig() (*Config, error) {
	var cfg Config
	found, err := p.Cfg.LoadValue(&cfg)
	if err != nil {
		return nil, err
	}
	if !found {
		p.Log.Info("Consul config not found, skip loading this plugin")
		p.disabled = true
		return nil, nil
	}
	return &cfg, nil
}

// ConfigToClient transforms Config into api.Config,
// which is ready for use with underlying consul package.
func ConfigToClient(cfg *Config) (*api.Config, error) {
	clientCfg := api.DefaultConfig()
	if cfg.Address != "" {
		clientCfg.Address = cfg.Address
	}
	return clientCfg, nil
}

// NewBroker creates new instance of prefixed broker that provides API with arguments of type proto.Message.
func (p *Plugin) NewBroker(keyPrefix string) keyval.ProtoBroker {
	return p.protoWrapper.NewBroker(keyPrefix)
}

// NewWatcher creates new instance of prefixed broker that provides API with arguments of type proto.Message.
func (p *Plugin) NewWatcher(keyPrefix string) keyval.ProtoWatcher {
	return p.protoWrapper.NewWatcher(keyPrefix)
}
