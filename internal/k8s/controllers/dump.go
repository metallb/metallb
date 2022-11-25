// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"encoding/json"

	"github.com/davecgh/go-spew/spew"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
)

func dumpClusterResources(c *config.ClusterResources) string {
	withNoSecret := config.ClusterResources{
		Pools:              c.Pools,
		Peers:              sanitizeBGPPeer(c.Peers...),
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

func dumpConfig(cfg *config.Config) string {
	toDump := *cfg
	toDump.Peers = make([]*config.Peer, 0)
	for _, p := range cfg.Peers {
		p1 := *p
		p1.Password = "<retracted>"
		toDump.Peers = append(toDump.Peers, &p1)
	}
	return spew.Sdump(toDump)
}

func dumpResource(i interface{}) string {
	toDump, err := json.Marshal(i)
	if err != nil {
		return spew.Sdump(i)
	}
	return string(toDump)
}

func sanitizeBGPPeer(peers ...metallbv1beta2.BGPPeer) []metallbv1beta2.BGPPeer {
	res := make([]metallbv1beta2.BGPPeer, 0)
	for _, p := range peers {
		toAdd := p.DeepCopy()
		toAdd.Spec.Password = "<retracted>"
		res = append(res, *toAdd)
	}
	return res
}
