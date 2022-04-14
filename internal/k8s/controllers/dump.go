// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"encoding/json"

	"github.com/davecgh/go-spew/spew"
	"go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
)

func dumpClusterResources(c *config.ClusterResources) string {
	withNoSecret := config.ClusterResources{
		Pools:              c.Pools,
		Peers:              c.Peers,
		BFDProfiles:        c.BFDProfiles,
		L2Advs:             c.L2Advs,
		BGPAdvs:            c.BGPAdvs,
		LegacyAddressPools: c.LegacyAddressPools,
		Communities:        c.Communities,
	}
	withNoSecret.PasswordSecrets = make(map[string]corev1.Secret)
	for k, s := range c.PasswordSecrets {
		secretToDump := *s.DeepCopy()
		secretToDump.Data = nil
		withNoSecret.PasswordSecrets[k] = secretToDump
	}
	return dumpResource(withNoSecret)
}

func dumpResource(i interface{}) string {
	toDump, err := json.Marshal(i)
	if err != nil {
		return spew.Sdump(i)
	}
	return string(toDump)
}
