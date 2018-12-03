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

package grpcsync

import (
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging/logrus"

	//TODO "github.com/gorilla/rpc/json"

	"github.com/ligato/cn-infra/datasync/syncbase"
	"github.com/ligato/cn-infra/datasync/syncbase/msg"
	"google.golang.org/grpc"
)

// Adapter is a gRPC transport adapter in front of Agent Plugins.
type Adapter struct {
	base   *syncbase.Registry
	server *grpc.Server
}

// NewAdapter creates a new instance of Adapter.
func NewAdapter(grpcServer *grpc.Server) *Adapter {
	//TODO grpcServer.RegisterCodec(json.NewCodec(), "application/json")
	adapter := &Adapter{
		base:   syncbase.NewRegistry(),
		server: grpcServer,
	}
	msg.RegisterDataMsgServiceServer(grpcServer, &DataMsgServiceServer{adapter})

	return adapter
}

// Watch registers HTTP handlers - basically bridges them with local dbadapter.
func (adapter *Adapter) Watch(resyncName string, changeChan chan datasync.ChangeEvent,
	resyncChan chan datasync.ResyncEvent, keyPrefixes ...string) (datasync.WatchRegistration, error) {

	logrus.DefaultLogger().Debug("GRPC KeyValProtoWatcher WatchData ", resyncName, " ", keyPrefixes)

	return adapter.base.Watch(resyncName, changeChan, resyncChan, keyPrefixes...)
}

// Close closes the gRPC server.
func (adapter *Adapter) Close() error {
	adapter.server.Stop()
	return nil
}
