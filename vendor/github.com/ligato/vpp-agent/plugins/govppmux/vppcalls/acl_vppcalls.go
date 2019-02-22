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

package vppcalls

import (
	"fmt"

	govppapi "git.fd.io/govpp.git/api"
	aclapi "github.com/ligato/vpp-agent/plugins/vpp/binapi/acl"
)

// GetACLPluginVersion retrieves ACL plugin version.
func GetACLPluginVersion(ch govppapi.Channel) (string, error) {
	req := &aclapi.ACLPluginGetVersion{}
	reply := &aclapi.ACLPluginGetVersionReply{}

	if err := ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return "", fmt.Errorf("failed to get VPP ACL plugin version: %v", err)
	}

	version := fmt.Sprintf("%d.%d", reply.Major, reply.Minor)

	return version, nil
}