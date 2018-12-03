// Copyright (c) 2018 Cisco and/or its affiliates.
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

package ifplugin

import (
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
)

// Export for testing

func PropagateIfDetailsToStatus(ifCfg *InterfaceConfigurator) error {
	return ifCfg.propagateIfDetailsToStatus()
}

func ResolveMappings(natCfg *NatConfigurator, nbDNatConfig *nat.Nat44DNat_DNatConfig,
	vppMappings *[]*nat.Nat44DNat_DNatConfig_StaticMapping, vppIDMappings *[]*nat.Nat44DNat_DNatConfig_IdentityMapping) {
	natCfg.resolveMappings(nbDNatConfig, vppMappings, vppIDMappings)
}

func IsIfModified(ifCfg *InterfaceConfigurator, nbIf, vppIf *interfaces.Interfaces_Interface) bool {
	return ifCfg.isIfModified(nbIf, vppIf)
}
