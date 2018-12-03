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

package restsync

import (
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging/logrus"

	"net/http"

	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/datasync/syncbase"
	"github.com/unrolled/render"
)

// Just a shortcut to make following code more readable.
type registerHTTPHandler func(path string, handler func(formatter *render.Render) http.HandlerFunc,
	methods ...string) *mux.Route

// NewAdapter is a constructor.
func NewAdapter(registerHTTPHandler registerHTTPHandler, localtransp *syncbase.Registry) *Adapter {
	return &Adapter{registerHTTPHandler: registerHTTPHandler, base: localtransp}
}

// Adapter is a REST transport adapter in front of Agent Plugins.
type Adapter struct {
	registerHTTPHandler registerHTTPHandler
	base                *syncbase.Registry
}

// RegisterTestHandler is used for runtime testing:
//   > curl -X GET http://localhost:<port>/log/list
func (adapter *Adapter) RegisterTestHandler() {
	adapter.registerHTTPHandler("/restsync/test", testHandler, "GET")
}

// Watch registers HTTP handlers - basically bridges them with local dbadapter.
func (adapter *Adapter) Watch(resyncName string, changeChan chan datasync.ChangeEvent,
	resyncChan chan datasync.ResyncEvent, keyPrefixes ...string) (datasync.WatchRegistration, error) {

	logrus.DefaultLogger().Debug("REST KeyValProtoWatcher WatchData ", resyncName, " ", keyPrefixes)

	for _, keyPrefix := range keyPrefixes {
		adapter.registerHTTPHandler(keyPrefix+"{suffix}", adapter.putMessage, "PUT")
		adapter.registerHTTPHandler(keyPrefix+"{suffix}", adapter.delMessage, "DELETE")
		//TODO adapter.registerHTTPHandler(keyPrefix + "{suffix}", getMessage, "GET")
		//TODO httpmux.RegisterHTTPHandler("/vpprestcon/resync", putResync, "PUT")
	}

	return adapter.base.Watch(resyncName, changeChan, resyncChan, keyPrefixes...)
}
