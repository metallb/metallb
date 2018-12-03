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

package cassandra

import (
	"errors"

	"github.com/ligato/cn-infra/db/sql"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/willfaught/gockle"
)

//
const (
	probeCassandraConnection = "SELECT keyspace_name FROM system_schema.keyspaces"
)

// Plugin implements Plugin interface therefore can be loaded with other plugins
type Plugin struct {
	Deps

	clientConfig *ClientConfig
	session      gockle.Session
}

// Deps is here to group injected dependencies of plugin
// to not mix with other plugin fields.
type Deps struct {
	infra.PluginDeps
	StatusCheck statuscheck.PluginStatusWriter // inject
}

var (
	// ErrMissingVisitorEntity is error returned when visitor is missing entity.
	ErrMissingVisitorEntity = errors.New("cassandra: visitor is missing entity")

	// ErrMissingEntityField is error returned when visitor entity is missing field.
	ErrMissingEntityField = errors.New("cassandra: visitor entity is missing field")

	// ErrUnexportedEntityField is error returned when visitor entity has unexported field.
	ErrUnexportedEntityField = errors.New("cassandra: visitor entity with unexported field")

	// ErrInvalidEndpointConfig is error returned when endpoint and port are not in valid format.
	ErrInvalidEndpointConfig = errors.New("cassandra: invalid configuration, endpoint and port not in valid format")
)

// Init is called at plugin startup. The session to Cassandra is established.
func (p *Plugin) Init() (err error) {
	if p.session != nil {
		return nil // skip initialization
	}

	// Retrieve config
	var cfg Config
	found, err := p.Cfg.LoadValue(&cfg)
	// need to be strict about config presence for ETCD
	if !found {
		p.Log.Info("cassandra client config not found ", p.Cfg.GetConfigName(),
			" - skip loading this plugin")
		return nil
	}
	if err != nil {
		return err
	}

	// Init session
	p.clientConfig, err = ConfigToClientConfig(&cfg)
	if err != nil {
		return err
	}

	if p.session == nil && p.clientConfig != nil {
		session, err := CreateSessionFromConfig(p.clientConfig)
		if err != nil {
			return err
		}

		p.session = gockle.NewSession(session)
	}

	// Register for providing status reports (polling mode)
	if p.StatusCheck != nil {
		if p.session != nil {
			p.StatusCheck.Register(p.PluginName, func() (statuscheck.PluginState, error) {
				broker := p.NewBroker()
				err := broker.Exec(`select keyspace_name from system_schema.keyspaces`)
				if err == nil {
					return statuscheck.OK, nil
				}
				return statuscheck.Error, err
			})
		} else {
			p.Log.Warnf("Cassandra connection not available")
		}
	} else {
		p.Log.Warnf("Unable to start status check for Cassandra")
	}

	return nil
}

// AfterInit is called by the Agent Core after all plugins have been initialized.
func (p *Plugin) AfterInit() error {
	return nil
}

// FromExistingSession is used mainly for testing
func FromExistingSession(session gockle.Session) *Plugin {
	return &Plugin{session: session}
}

// NewBroker returns a Broker instance to work with Cassandra Data Base
func (p *Plugin) NewBroker() sql.Broker {
	return NewBrokerUsingSession(p.session)
}

// Close resources
func (p *Plugin) Close() error {
	safeclose.Close(p.session)
	return nil
}
