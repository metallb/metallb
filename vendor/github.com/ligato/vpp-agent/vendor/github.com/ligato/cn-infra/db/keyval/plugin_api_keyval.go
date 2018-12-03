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

package keyval

// Root denotes that no prefix is prepended to the keys.
const Root = ""

// KvProtoPlugin provides unifying interface for different key-value datastore
// implementations.
type KvProtoPlugin interface {
	// NewPrefixedBroker returns a ProtoBroker instance that prepends given
	// <keyPrefix> to all keys in its calls.
	// To avoid using a prefix, pass keyval.Root constant as the argument.
	NewBroker(keyPrefix string) ProtoBroker
	// NewPrefixedWatcher returns a ProtoWatcher instance. Given key prefix
	// is prepended to keys during watch subscribe phase.
	// The prefix is removed from the key retrieved by GetKey() in ProtoWatchResp.
	// To avoid using a prefix, pass keyval.Root constant as argument.
	NewWatcher(keyPrefix string) ProtoWatcher
	// Disabled returns true if there was no configuration and therefore agent
	// started without connectivity to a particular data store.
	Disabled() bool
	// OnConnect executes datasync callback if KV plugin is connected. If not, it gathers
	// these functions from all plugins using the specific KV plugin as dependency and
	// if delayed start is allowed, callbacks are executed after successful connection.
	OnConnect(func() error)
	// Returns key value store name.
	String() string
}

// KvBytesPlugin provides unifying interface for different key-value datastore
// implementations.
type KvBytesPlugin interface {
	// NewBroker returns a BytesBroker instance that prepends given
	// <keyPrefix> to all keys in its calls.
	// To avoid using a prefix, pass keyval.Root constant as argument.
	NewBroker(keyPrefix string) BytesBroker
	// NewWatcher returns a BytesWatcher instance. Given <keyPrefix> is
	// prepended to keys during watch subscribe phase.
	// The prefix is removed from the key retrieved by GetKey() in BytesWatchResp.
	// To avoid using a prefix, pass keyval.Root constant as argument.
	NewWatcher(keyPrefix string) BytesWatcher
}
