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

package filedb

import (
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/filedb/decoder"
	"github.com/ligato/cn-infra/db/keyval/filedb/filesystem"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/infra"
)

// Plugin filesystem uses host os file system as database to store configuration.
type Plugin struct {
	Deps

	// FileDB client
	client *Client

	// Plugin config
	config *Config
	// Plugin is disabled without config
	disabled bool
	// Read or write proto modelled data
	protoWrapper *kvproto.ProtoWrapper
}

// Deps are fileDB plugin dependencies
type Deps struct {
	infra.PluginDeps
}

// Config is fileDB configuration file structure
type Config struct {
	ConfigPaths []string `json:"configuration-paths"`
	StatusPath  string   `json:"status-path"`
	// TODO possibly add option to store status to the same file as is the config
}

// Init reads file config and creates new client to communicate with file system
func (p *Plugin) Init() error {
	// Read fileDB configuration file
	var err error
	p.config, err = p.getFileDBConfig()
	if err != nil || p.disabled {
		return err
	}

	// Register decoders
	decoders := []decoder.API{decoder.NewJSONDecoder(), decoder.NewYAMLDecoder()}
	if p.client, err = NewClient(p.config.ConfigPaths, p.config.StatusPath, decoders, filesystem.NewFsHandler(), p.Log); err != nil {
		return err
	}

	p.protoWrapper = kvproto.NewProtoWrapper(p.client, &keyval.SerializerJSON{})

	return nil
}

// AfterInit starts file system event watcher
func (p *Plugin) AfterInit() error {
	if !p.disabled {
		p.client.eventWatcher()
	}

	return nil
}

// Close client
func (p *Plugin) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// Disabled returns flag whether plugin is disabled
func (p *Plugin) Disabled() bool {
	return p.disabled
}

// OnConnect executes datasync callback
func (p *Plugin) OnConnect(callback func() error) {
	if err := callback(); err != nil {
		p.Log.Error(err)
	}
}

// String returns string-representation of plugin name
func (p *Plugin) String() string {
	return p.PluginName.String()
}

// NewBroker returns new broker created by proto wrapper
func (p *Plugin) NewBroker(keyPrefix string) keyval.ProtoBroker {
	return p.protoWrapper.NewBroker(keyPrefix)
}

// NewWatcher returns new watcher created by proto wrapper
func (p *Plugin) NewWatcher(keyPrefix string) keyval.ProtoWatcher {
	return p.protoWrapper.NewWatcher(keyPrefix)
}

func (p *Plugin) getFileDBConfig() (*Config, error) {
	var fileDbCfg Config
	found, err := p.Cfg.LoadValue(&fileDbCfg)
	if err != nil {
		return nil, err
	}
	if !found {
		p.Log.Warnf("FileDB config not found, skip loading this plugin")
		p.disabled = true
	}
	return &fileDbCfg, nil
}
