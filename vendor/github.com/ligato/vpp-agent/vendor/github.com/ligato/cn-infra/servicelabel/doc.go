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

// Package servicelabel provides support for creating/retrieving an
// identifier (a service label) for a CN-Infra based app.
//
//     p := serviceLabel.Plugin{}
//     // initialization plugin handled by agent core
//
// To retrieve service label of the VNF instance, run:
//     label = p.GetAgentLabel()
//
// To retrieve prefix that can be used to access configuration of the VNF instance in key-value datastore, run:
//     prefix = p.GetAgentPrefix()
//
// To retrieve prefix for a different VNF instance, run:
//    otherPrefix = p.GetDifferentAgentPrefix(differentLabel)
//
// To retrieve prefix that identifies configuration of all instances:
//    allInstances = p.GetAllAgentsPrefix()
package servicelabel
