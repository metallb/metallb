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

package servicelabel

// default service key prefix, can be changed in the build time using ldflgs, e.g.
// -ldflags '-X github.com/ligato/cn-infra/servicelabel.agentPrefix=/xyz/'
var agentPrefix = "/vnf-agent/"

// MicroserviceLabelEnvVar label is inferred from the flag name.
const MicroserviceLabelEnvVar = "MICROSERVICE_LABEL"

// ReaderAPI allows to read microservice label and key prefix associated with
// this Agent instance.
// The key prefix is supposed to be prepended to all keys used to store/read
// data in any key-value datastore. The intent is to give a common prefix
// to all keys used by a single agent.
// Furthermore, different agents have different prefixes assigned, hence there
// is no overlap of key spaces in-between agents.
type ReaderAPI interface {
	// GetAgentLabel return the microservice label associated with this Agent
	// instance.
	GetAgentLabel() string

	// GetAgentPrefix returns the string that is supposed to be used
	// as the key prefix for the configuration "subtree" of the current Agent
	// instance (e.g. in ETCD).
	GetAgentPrefix() string
	// GetDifferentAgentPrefix returns the key prefix used by (another) Agent
	// instance from microservice labelled as <microserviceLabel>.
	GetDifferentAgentPrefix(microserviceLabel string) string

	// GetAllAgentsPrefix returns the part of the key prefix common to all
	// prefixes of all agents.
	GetAllAgentsPrefix() string
}

// GetAllAgentsPrefix returns the part of the key prefix common to all
// prefixes of all agents.
func GetAllAgentsPrefix() string {
	return agentPrefix
}

// GetDifferentAgentPrefix returns the key prefix used by (another) Agent
// instance from microservice labelled as <microserviceLabel>.
func GetDifferentAgentPrefix(microserviceLabel string) string {
	return agentPrefix + microserviceLabel + "/"
}
