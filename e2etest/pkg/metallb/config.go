// SPDX-License-Identifier:Apache-2.0

package metallb

import (
	"fmt"
	"os"
	"time"

	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	"go.universe.tf/metallb/internal/ipfamily"
	corev1 "k8s.io/api/core/v1"
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
func PeersForContainers(containers []*frrcontainer.FRR, ipFamily ipfamily.Family, tweak ...func(p *metallbv1beta2.BGPPeer)) []metallbv1beta2.BGPPeer {
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
			peer := metallbv1beta2.BGPPeer{
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
				}}
			for _, f := range tweak {
				f(&peer)
			}
			peers = append(peers, peer)
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

func BGPPeerSecretReferences(containers []*frrcontainer.FRR) map[string]corev1.Secret {
	secretMap := make(map[string]corev1.Secret)
	for _, c := range containers {
		name := GetBGPPeerSecretName(c.RouterConfig.ASN, c.RouterConfig.BGPPort)
		secretMap[name] = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Type: corev1.SecretTypeBasicAuth,
			Data: map[string][]byte{"password": []byte(c.RouterConfig.Password)},
		}
	}
	return secretMap
}

func GetBGPPeerSecretName(asn uint32, port uint16) string {
	return fmt.Sprintf("bgppeer-%d-%d-secret", asn, port)
}
