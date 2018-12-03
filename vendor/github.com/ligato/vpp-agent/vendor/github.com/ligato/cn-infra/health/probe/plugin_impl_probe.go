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

package probe

import (
	"encoding/json"
	"net/http"

	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/ligato/cn-infra/infra"
	prom "github.com/ligato/cn-infra/rpc/prometheus"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/unrolled/render"
)

const (
	livenessProbePath  = "/liveness"  // liveness probe URL
	readinessProbePath = "/readiness" // readiness probe URL
)

// Plugin struct holds all plugin-related data.
type Plugin struct {
	Deps
}

// Deps lists dependencies of REST plugin.
type Deps struct {
	infra.PluginDeps
	ServiceLabel servicelabel.ReaderAPI
	StatusCheck  statuscheck.StatusReader // inject
	HTTP         rest.HTTPHandlers        // inject
	Prometheus   prom.API                 // inject
}

// Init does nothing
func (p *Plugin) Init() error {
	return nil
}

// AfterInit registers HTTP handlers for liveness and readiness probes.
func (p *Plugin) AfterInit() error {
	if p.StatusCheck == nil {
		p.Log.Warnf("Unable to register probe handlers, StatusCheck is nil")
		return nil
	}

	if p.HTTP != nil {
		p.Log.Infof("Starting health http-probe on port %v", p.HTTP.GetPort())
		p.HTTP.RegisterHTTPHandler(livenessProbePath, p.livenessProbeHandler, "GET")
		p.HTTP.RegisterHTTPHandler(readinessProbePath, p.readinessProbeHandler, "GET")
	} else {
		p.Log.Info("Unable to register http-probe handler, HTTP is nil")
	}

	if p.Prometheus != nil {
		if err := p.registerPrometheusProbe(); err != nil {
			return err
		}
	} else {
		p.Log.Info("Unable to register prometheus-probe handler, Prometheus is nil")
	}

	return nil
}

// Close frees resources
func (p *Plugin) Close() error {
	return nil
}

// readinessProbeHandler handles k8s readiness probe.
func (p *Plugin) readinessProbeHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ifStat := p.StatusCheck.GetInterfaceStats()
		agentStat := p.StatusCheck.GetAgentStatus()
		agentStat.InterfaceStats = &ifStat
		agentStatJSON, _ := json.Marshal(agentStat)
		if agentStat.State == status.OperationalState_OK {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(agentStatJSON)
	}
}

// livenessProbeHandler handles k8s liveness probe.
func (p *Plugin) livenessProbeHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		stat := p.StatusCheck.GetAgentStatus()
		statJSON, _ := json.Marshal(p.StatusCheck.GetAgentStatus())

		if stat.State == status.OperationalState_INIT || stat.State == status.OperationalState_OK {
			w.WriteHeader(http.StatusOK)
			w.Write(statJSON)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(statJSON)
		}
	}
}
