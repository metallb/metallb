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

// Package restsync implements (in ALPHA VERSION) the datasync API for the
// HTTP/REST transport. This implementation is special (compared to dbsync
// or bussync) because it does not use any intermediate persistence between
// the client and the server. Therefore, the client does remote calls to each
// individual server/agent instance (and needs to know its IP address & port).
package restsync
