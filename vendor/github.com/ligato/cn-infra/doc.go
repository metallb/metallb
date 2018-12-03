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

// Package cninfra is the parent package for all packages that are parts
// of the CN-Infra platform - a Golang platform for building cloud-native
// microservices.
//
// The CN-Infra platform is a modular platform comprising a core and a set
// of plugins. The core provides lifecycle management for plugins, the
// plugins provide the functionality of the platform. A plugin can consist
// of one or more Golang packages. Out of the box, the CN-Infra platform
// provides reusable plugins for logging, health checks, messaging (e.g.
// Kafka), a common front-end API and back-end connectivity to various
// data stores (Etcd, Cassandra, Redis, ...), and REST and gRPC APIs.
//
// The CN-Infra platform can be extended by adding a new platform plugin,
// for example a new data store client or a message bus adapter. Also, each
// application built on top of the platform is basically just a set of
// application-specific plugins.
package cninfra
