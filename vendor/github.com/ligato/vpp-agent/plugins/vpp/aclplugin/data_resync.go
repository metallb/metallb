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

package aclplugin

import (
	"github.com/go-errors/errors"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
)

// Resync writes ACLs to the empty VPP.
func (c *ACLConfigurator) Resync(nbACLs []*acl.AccessLists_Acl) error {
	// Re-initialize cache
	c.clearMapping()

	// Retrieve existing IpACL config
	vppIPACLs, err := c.aclHandler.DumpIPACL(c.ifIndexes)
	if err != nil {
		return errors.Errorf("ACL resync error: failed to dump IP ACLs: %v", err)
	}
	// Retrieve existing MacIpACL config
	vppMacIPACLs, err := c.aclHandler.DumpMACIPACL(c.ifIndexes)
	if err != nil {
		return errors.Errorf("ACL resync error: failed to dump MAC IP ACLs: %v", err)
	}

	// Remove all configured VPP ACLs
	// Note: due to inability to dump ACL interfaces, it is not currently possible to correctly
	// calculate difference between configs
	for _, vppIPACL := range vppIPACLs {

		// ACL with IP-type rules uses different binary call to create/remove than MACIP-type.
		// Check what type of rules is in the ACL
		ipRulesExist := len(vppIPACL.ACL.Rules) > 0 && vppIPACL.ACL.Rules[0].GetMatch().GetIpRule() != nil

		if ipRulesExist {
			if err := c.aclHandler.DeleteIPACL(vppIPACL.Meta.Index); err != nil {
				return errors.Errorf("ACL resync error: failed to remove IP ACL %s: %v", vppIPACL.ACL.AclName, err)
			}
			// Unregister.
			c.l3l4AclIndexes.UnregisterName(vppIPACL.ACL.AclName)
			c.log.Debugf("ACL %s unregistered from L3/L4 mapping", vppIPACL.ACL.AclName)
			continue
		}
	}
	for _, vppMacIPACL := range vppMacIPACLs {
		ipRulesExist := len(vppMacIPACL.ACL.Rules) > 0 && vppMacIPACL.ACL.Rules[0].GetMatch().GetMacipRule() != nil
		if ipRulesExist {
			if err := c.aclHandler.DeleteMacIPACL(vppMacIPACL.Meta.Index); err != nil {
				return errors.Errorf("ACL resync error: failed to delete MAC IP ACL %s: %v", vppMacIPACL.ACL.AclName, err)
			}
			// Unregister.
			c.l2AclIndexes.UnregisterName(vppMacIPACL.ACL.AclName)
			c.log.Debugf("ACL %s unregistered from L2 mapping", vppMacIPACL.ACL.AclName)
			continue
		}
	}

	// Configure new ACLs
	for _, nbACL := range nbACLs {
		if err := c.ConfigureACL(nbACL); err != nil {
			c.log.Error(err)
			return errors.Errorf("ACL resync error: failed to configure ACL %s: %v", nbACL.AclName, err)
		}
	}

	c.log.Info("ACL resync done")

	return nil
}
