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

package syncbase

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging/logrus"
)

// Adapter implements datasync.TransportAdapter but allows the Watch/ Put functions to be optionally implemented.
type Adapter struct {
	Watcher   datasync.KeyValProtoWatcher
	Publisher datasync.KeyProtoValWriter
}

// Watch uses Kafka KeyValProtoWatcher Topic KeyValProtoWatcher.
func (adapter *Adapter) Watch(resyncName string, changeChan chan datasync.ChangeEvent,
	resyncChan chan datasync.ResyncEvent, keyPrefixes ...string) (datasync.WatchRegistration, error) {

	if adapter.Watcher != nil {
		return adapter.Watcher.Watch(resyncName, changeChan, resyncChan, keyPrefixes...)
	}
	logrus.DefaultLogger().Debug("KeyValProtoWatcher is nil")

	return nil, nil
}

// Put uses Kafka KeyValProtoWatcher Topic KeyProtoValWriter.
func (adapter *Adapter) Put(key string, data proto.Message) error {
	if adapter.Publisher != nil {
		return adapter.Publisher.Put(key, data)
	}

	return nil
}
