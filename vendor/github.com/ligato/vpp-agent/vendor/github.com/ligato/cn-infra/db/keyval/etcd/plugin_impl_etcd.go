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

package etcd

import (
	"fmt"
	"sync"
	"time"

	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/utils/safeclose"
)

const (
	// healthCheckProbeKey is a key used to probe Etcd state
	healthCheckProbeKey = "/probe-etcd-connection"
	// ETCD reconnect interval
	defaultReconnectInterval = 2 * time.Second
)

// Plugin implements etcd plugin.
type Plugin struct {
	Deps

	sync.Mutex

	// Plugin is disabled if there is no config file available
	disabled bool
	// Set if connected to ETCD db
	connected bool
	// ETCD connection encapsulation
	connection *BytesConnectionEtcd
	// Read/Write proto modelled data
	protoWrapper *kvproto.ProtoWrapper

	// plugin config
	config *Config

	// List of callback functions, used in case ETCD is not connected immediately. All plugins using
	// ETCD as dependency add their own function if cluster is not reachable. After connection, all
	// functions are executed.
	onConnection []func() error

	autoCompactDone chan struct{}
	lastConnErr     error
}

// Deps lists dependencies of the etcd plugin.
// If injected, etcd plugin will use StatusCheck to signal the connection status.
type Deps struct {
	infra.PluginDeps
	StatusCheck statuscheck.PluginStatusWriter // inject
	Resync      *resync.Plugin
}

// Init retrieves ETCD configuration and establishes a new connection
// with the etcd data store.
// If the configuration file doesn't exist or cannot be read, the returned error
// will be of os.PathError type. An untyped error is returned in case the file
// doesn't contain a valid YAML configuration.
// The function may also return error if TLS connection is selected and the
// CA or client certificate is not accessible(os.PathError)/valid(untyped).
// Check clientv3.New from coreos/etcd for possible errors returned in case
// the connection cannot be established.
func (p *Plugin) Init() (err error) {
	// Read ETCD configuration file. Returns error if does not exists.
	p.config, err = p.getEtcdConfig()
	if err != nil || p.disabled {
		return err
	}

	// Transforms .yaml config to ETCD client configuration
	etcdClientCfg, err := ConfigToClient(p.config)
	if err != nil {
		return err
	}

	// Uses config file to establish connection with the database
	p.connection, err = NewEtcdConnectionWithBytes(*etcdClientCfg, p.Log)

	// Register for providing status reports (polling mode).
	if p.StatusCheck != nil {
		p.StatusCheck.Register(p.PluginName, p.statusCheckProbe)
	} else {
		p.Log.Warnf("Unable to start status check for etcd")
	}
	if err != nil && p.config.AllowDelayedStart {
		// If the connection cannot be established during init, keep trying in another goroutine (if allowed) and
		// end the init
		go p.etcdReconnectionLoop(etcdClientCfg)
		return nil
	} else if err != nil {
		// If delayed start is not allowed, return error
		return fmt.Errorf("error connecting to ETCD: %v", err)
	}

	// If successful, configure and return
	p.configureConnection()

	// Mark p as connected at this point
	p.connected = true

	return nil
}

// Close shutdowns the connection.
func (p *Plugin) Close() error {
	return safeclose.Close(p.autoCompactDone)
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
// etcd configuration.
func (p *Plugin) Disabled() (disabled bool) {
	return p.disabled
}

// OnConnect executes callback if plugin is connected, or gathers functions from all plugin with ETCD as dependency
func (p *Plugin) OnConnect(callback func() error) {
	p.Lock()
	defer p.Unlock()

	if p.connected {
		if err := callback(); err != nil {
			p.Log.Errorf("callback for OnConnect failed: %v", err)
		}
	} else {
		p.onConnection = append(p.onConnection, callback)
	}
}

// GetPluginName returns name of the plugin
func (p *Plugin) GetPluginName() infra.PluginName {
	return p.PluginName
}

// PutIfNotExists puts given key-value pair into etcd if there is no value set for the key. If the put was successful
// succeeded is true. If the key already exists succeeded is false and the value for the key is untouched.
func (p *Plugin) PutIfNotExists(key string, value []byte) (succeeded bool, err error) {
	if p.connection != nil {
		return p.connection.PutIfNotExists(key, value)
	}
	return false, fmt.Errorf("connection is not established")
}

// Compact compatcs the ETCD database to the specific revision
func (p *Plugin) Compact(rev ...int64) (toRev int64, err error) {
	if p.connection != nil {
		return p.connection.Compact(rev...)
	}
	return 0, fmt.Errorf("connection is not established")
}

// Method starts loop which attempt to connect to the ETCD. If successful, send signal callback with resync,
// which will be started when datasync confirms successful registration
func (p *Plugin) etcdReconnectionLoop(clientCfg *ClientConfig) {
	var err error
	// Set reconnect interval
	interval := p.config.ReconnectInterval
	if interval == 0 {
		interval = defaultReconnectInterval
	}
	p.Log.Infof("ETCD server %s not reachable in init phase. Agent will continue to try to connect every %d second(s)",
		p.config.Endpoints, interval)
	for {
		time.Sleep(interval)

		p.Log.Infof("Connecting to ETCD %v ...", p.config.Endpoints)
		p.connection, err = NewEtcdConnectionWithBytes(*clientCfg, p.Log)
		if err != nil {
			continue
		}
		p.setupPostInitConnection()
		return
	}
}

func (p *Plugin) setupPostInitConnection() {
	p.Log.Infof("ETCD server %s connected", p.config.Endpoints)

	p.Lock()
	defer p.Unlock()

	// Configure connection and set as connected
	p.configureConnection()
	p.connected = true

	// Execute callback functions (if any)
	for _, callback := range p.onConnection {
		if err := callback(); err != nil {
			p.Log.Errorf("callback for OnConnect failed: %v", err)
		}
	}

	// Call resync if any callback was executed. Otherwise there is nothing to resync
	if p.Resync != nil && len(p.onConnection) > 0 {
		p.Resync.DoResync()
	}
	p.Log.Debugf("Etcd reconnection loop ended")
}

// If ETCD is connected, complete all other procedures
func (p *Plugin) configureConnection() {
	if p.config.AutoCompact > 0 {
		if p.config.AutoCompact < time.Duration(time.Minute*60) {
			p.Log.Warnf("Auto compact option for ETCD is set to less than 60 minutes!")
		}
		p.startPeriodicAutoCompact(p.config.AutoCompact)
	}
	p.protoWrapper = kvproto.NewProtoWrapper(p.connection, &keyval.SerializerJSON{})
}

// ETCD status check probe function
func (p *Plugin) statusCheckProbe() (statuscheck.PluginState, error) {
	if p.connection == nil {
		p.connected = false
		return statuscheck.Error, fmt.Errorf("no ETCD connection available")
	}
	if _, _, _, err := p.connection.GetValue(healthCheckProbeKey); err != nil {
		p.lastConnErr = err
		p.connected = false
		return statuscheck.Error, err
	}
	if p.config.ReconnectResync && p.lastConnErr != nil {
		if p.Resync != nil {
			p.Resync.DoResync()
			p.lastConnErr = nil
		} else {
			p.Log.Warn("Expected resync after ETCD reconnect could not start beacuse of missing Resync plugin")
		}
	}
	p.connected = true
	return statuscheck.OK, nil
}

func (p *Plugin) getEtcdConfig() (*Config, error) {
	var etcdCfg Config
	found, err := p.Cfg.LoadValue(&etcdCfg)
	if err != nil {
		return nil, err
	}
	if !found {
		p.Log.Info("ETCD config not found, skip loading this plugin")
		p.disabled = true
	}
	return &etcdCfg, nil
}

func (p *Plugin) startPeriodicAutoCompact(period time.Duration) {
	p.autoCompactDone = make(chan struct{})
	go func() {
		p.Log.Infof("Starting periodic auto compacting every %v", period)
		for {
			select {
			case <-time.After(period):
				p.Log.Debugf("Executing auto compact")
				if toRev, err := p.connection.Compact(); err != nil {
					p.Log.Errorf("Periodic auto compacting failed: %v", err)
				} else {
					p.Log.Infof("Auto compacting finished (to revision %v)", toRev)
				}
			case <-p.autoCompactDone:
				return
			}
		}
	}()
}
