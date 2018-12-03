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
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/rpc/rest/security"
	access "github.com/ligato/cn-infra/rpc/rest/security/model/access-security"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/unrolled/render"
)

// Plugin struct holds all plugin-related data.
type Plugin struct {
	Deps

	*Config

	server    *http.Server
	mx        *mux.Router
	formatter *render.Render

	// Access to HTTP security API
	auth security.AuthenticatorAPI
}

// Deps lists the dependencies of the Rest plugin.
type Deps struct {
	infra.PluginDeps

	// Authenticator can be injected in a flavor inject method.
	// If there is no authenticator injected and config contains
	// user password, the default staticAuthenticator is instantiated.
	// By default the authenticator is disabled.
	Authenticator BasicHTTPAuthenticator
}

// Init is the plugin entry point called by Agent Core
// - It prepares Gorilla MUX HTTP Router
func (p *Plugin) Init() (err error) {
	if p.Config == nil {
		p.Config = DefaultConfig()
	}
	if err := PluginConfig(p.Cfg, p.Config, p.PluginName); err != nil {
		return err
	}

	// if there is no injected authenticator and there are credentials defined in the config file
	// instantiate staticAuthenticator otherwise do not use basic Auth
	if p.Authenticator == nil && len(p.Config.ClientBasicAuth) > 0 {
		p.Authenticator, err = newStaticAuthenticator(p.Config.ClientBasicAuth)
		if err != nil {
			return err
		}
	}

	p.mx = mux.NewRouter()
	p.formatter = render.New(render.Options{
		IndentJSON: true,
	})

	// Enable authentication if defined by config
	if p.EnableTokenAuth {
		p.Log.Info("Token authentication for HTTP enabled")
		p.auth = security.NewAuthenticator(p.mx, &security.Settings{
			Users:     p.Users,
			ExpTime:   p.TokenExpiration,
			Cost:      p.PasswordHashCost,
			Signature: p.TokenSignature,
		}, p.Log)
	}

	return err
}

// AfterInit starts the HTTP server.
func (p *Plugin) AfterInit() (err error) {
	cfgCopy := *p.Config

	var handler http.Handler = p.mx
	if p.Authenticator != nil {
		handler = auth(handler, p.Authenticator)
	}

	p.server, err = ListenAndServe(cfgCopy, handler)
	if err != nil {
		return err
	}

	if cfgCopy.UseHTTPS() {
		p.Log.Info("Listening on https://", cfgCopy.Endpoint)
	} else {
		p.Log.Info("Listening on http://", cfgCopy.Endpoint)
	}

	return nil
}

// RegisterHTTPHandler registers HTTP <handler> at the given <path>. Every request is validated if enabled.
func (p *Plugin) RegisterHTTPHandler(path string, provider HandlerProvider, methods ...string) *mux.Route {
	p.Log.Debugf("Registering handler: %s", path)

	if p.Config.EnableTokenAuth {
		return p.mx.Handle(path, p.auth.Validate(provider(p.formatter))).Methods(methods...)
	}
	return p.mx.Handle(path, provider(p.formatter)).Methods(methods...)
}

// RegisterPermissionGroup adds new permission group if token authentication is enabled
func (p *Plugin) RegisterPermissionGroup(group ...*access.PermissionGroup) {
	if p.Config.EnableTokenAuth {
		p.Log.Debugf("Registering permission group(s): %s", group)
		p.auth.AddPermissionGroup(group...)
	}
}

// GetPort returns plugin configuration port
func (p *Plugin) GetPort() int {
	if p.Config != nil {
		return p.Config.GetPort()
	}
	return 0
}

// Close stops the HTTP server.
func (p *Plugin) Close() error {
	return safeclose.Close(p.server)
}
