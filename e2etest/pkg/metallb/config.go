// SPDX-License-Identifier:Apache-2.0

package metallb

import (
	"fmt"
	"os"
	"time"

	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	"go.universe.tf/metallb/internal/ipfamily"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func PeersForContainers(containers []*frrcontainer.FRR, ipFamily ipfamily.Family) []metallbv1beta2.BGPPeer {
	var peers []metallbv1beta2.BGPPeer

	for i, c := range containers {
		addresses := c.AddressesForFamily(ipFamily)
		holdTime := 3 * time.Second
		if i > 0 {
			holdTime = time.Duration(i) * 180 * time.Second
		}
		ebgpMultihop := false
		if c.NeighborConfig.MultiHop && c.NeighborConfig.ASN != c.RouterConfig.ASN {
			ebgpMultihop = true
		}
		for i, address := range addresses {
			peers = append(peers, metallbv1beta2.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name: c.Name + fmt.Sprint(i), // Otherwise the peers will override
				},
				Spec: metallbv1beta2.BGPPeerSpec{
					Address:      address,
					ASN:          c.RouterConfig.ASN,
					MyASN:        c.NeighborConfig.ASN,
					Port:         c.RouterConfig.BGPPort,
					Password:     c.RouterConfig.Password,
					HoldTime:     metav1.Duration{Duration: holdTime},
					EBGPMultiHop: ebgpMultihop,
				},
			})
		}
	}
	return peers
}

// WithBFD sets the given bfd profile to the peers.
func WithBFD(peers []metallbv1beta2.BGPPeer, bfdProfile string) []metallbv1beta2.BGPPeer {
	for i := range peers {
		peers[i].Spec.BFDProfile = bfdProfile
	}
	return peers
}

// WithRouterID sets the given routerID to the peers.
func WithRouterID(peers []metallbv1beta2.BGPPeer, routerID string) []metallbv1beta2.BGPPeer {
	for i := range peers {
		peers[i].Spec.RouterID = routerID
	}
	return peers
}
