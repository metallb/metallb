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

package mock

import (
	"net/http"
	"net/http/httptest"

	"io"

	"github.com/ligato/cn-infra/rpc/rest"
)

// HTTPMock is supposed to be used to mock real HTTP server but have the ability
// to test all other httpmux plugin related code
//
// Example:
//
//    httpmux.FromExistingServer(mock.SetHandler)
//	  mock.NewRequest("GET", "/v1/a", nil)
//
type HTTPMock struct {
	handler http.Handler
}

// SetHandler is called from httpmux plugin during startup (handler is usually the gorilla mux instance)
func (mock *HTTPMock) SetHandler(config rest.Config, handler http.Handler) (httpServer io.Closer, err error) {
	mock.handler = handler
	return &doNothingCloser{}, nil
}

// NewRequest propagates the request to the httpmux
func (mock *HTTPMock) NewRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	recorder := httptest.NewRecorder()
	mock.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}

type doNothingCloser struct{}

// Close does nothing
func (*doNothingCloser) Close() error {
	return nil
}
