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

package vpp_stn

import (
	"github.com/ligato/vpp-agent/pkg/models"
)

// ModuleName is the module name used for models.
const ModuleName = "vpp.stn"

var (
	ModelRule = models.Register(&Rule{}, models.Spec{
		Module:  ModuleName,
		Type:    "rule",
		Version: "v2",
	}, models.WithNameTemplate("{{.Interface}}/ip/{{.IpAddress}}"))
)

// Key returns the prefix used in the ETCD to store a VPP STN config
// of a particular STN rule in selected VPP instance.
func Key(ifName, ipAddr string) string {
	return models.Key(&Rule{
		Interface: ifName,
		IpAddress: ipAddr,
	})
}
