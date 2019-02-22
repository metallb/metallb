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

package vpp_punt

import (
	"github.com/ligato/vpp-agent/pkg/models"
)

// ModuleName is the module name used for models.
const ModuleName = "vpp"

var (
	ModelToHost = models.Register(&ToHost{}, models.Spec{
		Module:  ModuleName,
		Type:    "tohost",
		Version: "v2",
	}, models.WithNameTemplate(
		"l3/{{.L3Protocol}}/l4/{{.L4Protocol}}/port/{{.Port}}",
	))
	ModelIPRedirect = models.Register(&IPRedirect{}, models.Spec{
		Module:  ModuleName,
		Type:    "ipredirect",
		Version: "v2",
	}, models.WithNameTemplate(
		"l3/{{.L3Protocol}}/tx/{{.TxInterface}}",
	))
)

// ToHostKey returns key representing punt to host/socket configuration.
func ToHostKey(l3Proto L3Protocol, l4Proto L4Protocol, port uint32) string {
	return models.Key(&ToHost{
		L3Protocol: l3Proto,
		L4Protocol: l4Proto,
		Port:       port,
	})
}

// IPRedirectKey returns key representing IP punt redirect configuration.
func IPRedirectKey(l3Proto L3Protocol, txIf string) string {
	return models.Key(&IPRedirect{
		L3Protocol:  l3Proto,
		TxInterface: txIf,
	})
}
