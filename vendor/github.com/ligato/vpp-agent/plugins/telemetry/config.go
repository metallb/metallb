//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package telemetry

import "time"

// Config file representation for telemetry plugin
type Config struct {
	// Custom polling interval, default value is 30s
	PollingInterval time.Duration `json:"polling-interval"`
	// Allows to disable plugin
	Disabled bool `json:"disabled"`
}

// getConfig returns telemetry plugin file configuration if exists
func (p *Plugin) getConfig() (*Config, error) {
	config := &Config{}
	found, err := p.Cfg.LoadValue(config)
	if err != nil {
		return nil, err
	}
	if !found {
		p.Log.Debug("Telemetry config not found")
		return nil, nil
	}
	p.Log.Debug("Telemetry config found")
	return config, err
}
