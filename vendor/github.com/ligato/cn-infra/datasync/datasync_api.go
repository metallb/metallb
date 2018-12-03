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

package datasync

import (
	"io"
	"time"

	"github.com/gogo/protobuf/proto"
)

// DefaultNotifTimeout defines the default timeout for datasync notification delivery.
const DefaultNotifTimeout = 2 * time.Second

// KeyValProtoWatcher is used by plugins to subscribe to both data change
// events and data resync events. Multiple keys can be specified and the
// caller will be subscribed to events on each key.
// See README.md for description of the Events.
type KeyValProtoWatcher interface {
	// Watch using ETCD or any other data transport.
	// <resyncName> is used for the name of the RESYNC subcription.
	// <changeChan> channel is used for delivery of data CHANGE events.
	// <resyncChan> channel is used for delivery of data RESYNC events.
	// <keyPrefix> is a variable list of keys to watch on.
	Watch(resyncName string, changeChan chan ChangeEvent, resyncChan chan ResyncEvent,
		keyPrefixes ...string) (WatchRegistration, error)
}

// KeyProtoValWriter allows plugins to push their data changes to a data store.
type KeyProtoValWriter interface {
	// Put <data> to ETCD or to any other key-value based data transport
	// (from other Agent Plugins) under the key <key>.
	// See options.go for a list of available options.
	Put(key string, data proto.Message, opts ...PutOption) error
}

// WatchRegistration is a facade that avoids importing the io.Closer package
// into Agent plugin implementations.
type WatchRegistration interface {
	// Add <keyPrefix> to adapter subscription under specific <resyncName>.
	// If called on registration returned by composite watcher, register
	// <keyPrefix> to all adapters. Returns error if there is no subscription
	// with provided resync name or prefix already exists
	Register(resyncName string, keyPrefix string) error

	// Unregister <keyPrefix> from adapter subscription. If called on registration
	// returned by composite watcher, unregister <keyPrefix> from all adapters
	Unregister(keyPrefix string) error

	io.Closer
}
