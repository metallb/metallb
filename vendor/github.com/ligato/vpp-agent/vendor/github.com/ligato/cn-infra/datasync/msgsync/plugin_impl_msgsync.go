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

package msgsync

import (
	"errors"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/messaging"
)

// Plugin implements KeyProtoValWriter that propagates protobuf messages
// to a particular topic (unless the messaging.Mux is not disabled).
type Plugin struct {
	Deps

	Config
	adapter messaging.ProtoPublisher
}

// Deps groups dependencies injected into the plugin so that they are
// logically separated from other plugin fields.
type Deps struct {
	infra.PluginDeps
	Messaging messaging.Mux
}

// Config groups configurations fields. It can be extended with other fields
// (such as sync/async, partition...).
type Config struct {
	Topic string
}

// Init does nothing.
func (p *Plugin) Init() error {
	return nil
}

// AfterInit uses provided MUX connection to build new publisher.
func (p *Plugin) AfterInit() error {
	if !p.Messaging.Disabled() {
		cfg := p.Config
		p.Cfg.LoadValue(&cfg)

		if cfg.Topic != "" {
			var err error
			p.adapter, err = p.Messaging.NewSyncPublisher("msgsync-connection", cfg.Topic)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Put propagates this call to a particular messaging Publisher.
//
// This method is supposed to be called in PubPlugin.AfterInit() or later (even from different go routine).
func (p *Plugin) Put(key string, data proto.Message, opts ...datasync.PutOption) error {
	if p.Messaging.Disabled() {
		return nil
	}

	if p.adapter != nil {
		return p.adapter.Put(key, data, opts...)
	}

	return errors.New("Transport adapter is not ready yet. (Probably called before AfterInit)")
}

// Close resources.
func (p *Plugin) Close() error {
	return nil
}
