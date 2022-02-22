// SPDX-License-Identifier:Apache-2.0

package metallb

import (
	"fmt"
	"os"

	"go.universe.tf/metallb/e2etest/pkg/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	"go.universe.tf/metallb/internal/ipfamily"
)

const (
	defaultNameSpace = "metallb-system"
)

var Namespace = defaultNameSpace

func init() {
	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		Namespace = ns
	}
}

// PeersForContainers returns the metallb config peers related to the given containers.
func PeersForContainers(containers []*frrcontainer.FRR, ipFamily ipfamily.Family) []config.Peer {
	var peers []config.Peer

	for i, c := range containers {
		addresses := c.AddressesForFamily(ipFamily)
		holdTime := ""
		if i > 0 {
			holdTime = fmt.Sprintf("%ds", i*180)
		}
		ebgpMultihop := false
		if c.NeighborConfig.MultiHop && c.NeighborConfig.ASN != c.RouterConfig.ASN {
			ebgpMultihop = true
		}
		for _, address := range addresses {
			peers = append(peers, config.Peer{
				Addr:         address,
				ASN:          c.RouterConfig.ASN,
				MyASN:        c.NeighborConfig.ASN,
				Port:         c.RouterConfig.BGPPort,
				Password:     c.RouterConfig.Password,
				HoldTime:     holdTime,
				EBGPMultiHop: ebgpMultihop,
			})
		}
	}
	return peers
}

// WithBFD sets the given bfd profile to the peers.
func WithBFD(peers []config.Peer, bfdProfile string) []config.Peer {
	for i := range peers {
		peers[i].BFDProfile = bfdProfile
	}
	return peers
}

// WithRouterID sets the given routerID to the peers.
func WithRouterID(peers []config.Peer, routerID string) []config.Peer {
	for i := range peers {
		peers[i].RouterID = routerID
	}
	return peers
}
