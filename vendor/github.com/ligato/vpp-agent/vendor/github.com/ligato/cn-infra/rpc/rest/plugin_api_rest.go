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

package rest

import (
	"net/http"

	"github.com/gorilla/mux"
	access "github.com/ligato/cn-infra/rpc/rest/security/model/access-security"
	"github.com/unrolled/render"
)

// HandlerProvider is a function used for registering handlers via HTTPHandlers
type HandlerProvider func(formatter *render.Render) http.HandlerFunc

// HTTPHandlers defines the API exposed by the REST plugin.
// Use this interface to declare dependency on the REST functionality, i.e.:
//
// type Deps struct {
//     HTTP rest.HTTPHandlers // inject plugin implementing RegisterHTTPHandler
//     // other dependencies ...
// }
//
type HTTPHandlers interface {
	// RegisterHTTPHandler propagates to Gorilla mux
	RegisterHTTPHandler(path string, provider HandlerProvider, methods ...string) *mux.Route

	// RegisterPermissionGroup registers new permission groups for users
	RegisterPermissionGroup(group ...*access.PermissionGroup)

	// GetPort returns configured port number (for debugging purposes)
	GetPort() int
}

// BasicHTTPAuthenticator is a delegate that implements basic HTTP authentication
type BasicHTTPAuthenticator interface {
	// Authenticate returns true if user is authenticated successfully, false otherwise.
	Authenticate(user string, pass string) bool
}
