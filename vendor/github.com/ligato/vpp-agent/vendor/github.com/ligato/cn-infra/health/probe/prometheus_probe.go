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
	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// DefaultHealthPath default Prometheus health metrics URL
	DefaultHealthPath = "/health"

	// Namespace namespace to use for Prometheus health metrics
	Namespace = ""
	// Subsystem subsystem to use for Prometheus health metrics
	Subsystem = ""
	// ServiceLabel label for service field
	ServiceLabel = "service"
	// DependencyLabel label for dependency field
	DependencyLabel = "dependency"
	// BuildVersionLabel label for build version field
	BuildVersionLabel = "build_version"
	// BuildDateLabel label for build date field
	BuildDateLabel = "build_date"

	// ServiceHealthName name of service health metric
	ServiceHealthName = "service_health"

	// ServiceHealthHelp help text for service health metric
	// Adapt Ligato status code for now.
	// TODO: Consolidate with that from the "Common Container Telemetry" proposal.
	// ServiceHealthHelp    string = "The health of the ServiceLabel 0 = INIT, 1 = UP, 2 = DOWN, 3 = OUTAGE"
	ServiceHealthHelp = "The health of the ServiceLabel 0 = INIT, 1 = OK, 2 = ERROR"

	// DependencyHealthName name of dependency health metric
	DependencyHealthName = "service_dependency_health"

	// DependencyHealthHelp help text for dependency health metric
	// Adapt Ligato status code for now.
	// TODO: Consolidate with that from the "Common Container Telemetry" proposal.
	// DependencyHealthHelp string = "The health of the DependencyLabel 0 = INIT, 1 = UP, 2 = DOWN, 3 = OUTAGE"
	DependencyHealthHelp = "The health of the DependencyLabel 0 = INIT, 1 = OK, 2 = ERROR"

	// ServiceInfoName name of service info metric
	ServiceInfoName = "service_info"
	// ServiceInfoHelp help text for service info metric
	ServiceInfoHelp = "Build info for the service.  Value is always 1, build info is in the tags."
)

func (p *Plugin) registerPrometheusProbe() error {
	err := p.Prometheus.NewRegistry(DefaultHealthPath, promhttp.HandlerOpts{})
	if err != nil {
		return err
	}
	p.Prometheus.RegisterGaugeFunc(DefaultHealthPath,
		Namespace, Subsystem,
		ServiceHealthName, ServiceHealthHelp,
		prometheus.Labels{
			ServiceLabel: p.getServiceLabel(),
		},
		p.getServiceHealth,
	)
	agentStatus := p.StatusCheck.GetAgentStatus()
	p.Prometheus.RegisterGaugeFunc(DefaultHealthPath,
		Namespace, Subsystem,
		ServiceInfoName, ServiceInfoHelp,
		prometheus.Labels{
			ServiceLabel:      p.getServiceLabel(),
			BuildVersionLabel: agentStatus.BuildVersion,
			BuildDateLabel:    agentStatus.BuildDate,
		},
		func() float64 { return 1 },
	)
	allPluginStatusMap := p.StatusCheck.GetAllPluginStatus()
	for k, v := range allPluginStatusMap {
		p.Log.Infof("k=%v, v=%v, state=%v", k, v, v.State)
		p.Prometheus.RegisterGaugeFunc(DefaultHealthPath,
			Namespace, Subsystem,
			DependencyHealthName, DependencyHealthHelp,
			prometheus.Labels{
				ServiceLabel:    p.getServiceLabel(),
				DependencyLabel: k,
			},
			p.getDependencyHealth(k, v),
		)
	}
	return nil
}

// getServiceHealth returns agent health status
func (p *Plugin) getServiceHealth() float64 {
	agentStatus := p.StatusCheck.GetAgentStatus()
	// Adapt Ligato status code for now.
	// TODO: Consolidate with that from the "Common Container Telemetry" proposal.
	health := float64(agentStatus.State)
	p.Log.Infof("ServiceHealth: %v", health)
	return health
}

// getDependencyHealth returns plugin health status
func (p *Plugin) getDependencyHealth(pluginName string, pluginStatus *status.PluginStatus) func() float64 {
	p.Log.Infof("DependencyHealth for plugin %v: %v", pluginName, float64(pluginStatus.State))

	return func() float64 {
		health := float64(pluginStatus.State)
		p.Log.Infof("Dependency Health %v: %v", pluginName, health)
		return health
	}
}

func (p *Plugin) getServiceLabel() string {
	if p.ServiceLabel != nil {
		return p.ServiceLabel.GetAgentLabel()
	}
	return ""
}
