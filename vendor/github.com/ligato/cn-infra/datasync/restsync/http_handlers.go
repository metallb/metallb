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
	"net/http"

	"github.com/unrolled/render"

	"io/ioutil"

	"github.com/ligato/cn-infra/datasync/kvdbsync/local"
)

// putMessage is only a stub prepared for later implementation.
func (adapter *Adapter) putMessage(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := req.GetBody()
		if err != nil {
			var data []byte
			data, err = ioutil.ReadAll(body)
			defer req.Body.Close()

			if err != nil {
				localtxn := local.NewBytesTxn(adapter.base.PropagateChanges)
				localtxn.Put(req.RequestURI, data)
				err = localtxn.Commit()
			}
		}

		if err != nil {
			formatter.JSON(w, http.StatusInternalServerError, struct{ Test string }{err.Error()})
		} else {
			formatter.JSON(w, http.StatusOK, struct{ Test string }{"OK"})
		}
	}
}

// delMessage only calls local dbadapter delete.
func (adapter *Adapter) delMessage(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		localtxn := local.NewBytesTxn(adapter.base.PropagateChanges)
		localtxn.Delete(req.RequestURI)
		err := localtxn.Commit()

		if err != nil {
			formatter.JSON(w, http.StatusInternalServerError, struct{ Test string }{err.Error()})
		} else {
			formatter.JSON(w, http.StatusOK, struct{ Test string }{"OK"})
		}
	}
}

// Simple test handler is used to test that everything is integrated propely in runtime.
func testHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		formatter.JSON(w, http.StatusOK, struct{ Test string }{"This is a test"})
	}
}
