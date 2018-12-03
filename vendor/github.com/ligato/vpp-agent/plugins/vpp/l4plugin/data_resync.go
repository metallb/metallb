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

package l4plugin

import (
	"github.com/go-errors/errors"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l4"
)

// ResyncAppNs configures app namespaces to the empty VPP
func (c *AppNsConfigurator) ResyncAppNs(appNamespaces []*l4.AppNamespaces_AppNamespace) error {
	// Re-initialize cache
	c.clearMapping()

	if len(appNamespaces) > 0 {
		for _, appNs := range appNamespaces {
			if err := c.ConfigureAppNamespace(appNs); err != nil {
				return errors.Errorf("app-ns resync error: failed to configure application namespace %s: %v",
					appNs.NamespaceId, err)
			}
		}
	}
	c.log.Info("Application namespace resync done.")

	return nil
}

// ResyncFeatures sets initial L4Features flag
func (c *AppNsConfigurator) ResyncFeatures(l4Features *l4.L4Features) error {
	if l4Features != nil {
		if err := c.ConfigureL4FeatureFlag(l4Features); err != nil {
			return errors.Errorf("app-ns resync error: failed to configure L4: %v", err)
		}
	}
	c.log.Info("L4 features resync done.")

	return nil
}
