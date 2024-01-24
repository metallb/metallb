// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConversionIsStable(t *testing.T) {
	peers := []v1beta2.BGPPeer{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "peer1",
				Namespace: "metallb-system",
			},
			Spec: v1beta2.BGPPeerSpec{
				MyASN:      42,
				ASN:        142,
				Address:    "1.2.3.4",
				BFDProfile: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "peer2",
				Namespace: "metallb-system",
			},
			Spec: v1beta2.BGPPeerSpec{
				MyASN:      42,
				ASN:        142,
				Address:    "1.2.3.5",
				BFDProfile: "default",
			},
		},
	}
	bfdProfiles := []v1beta1.BFDProfile{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "metallb-system",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar",
				Namespace: "metallb-system",
			},
		},
	}
	pools := []v1beta1.IPAddressPool{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pool1",
				Namespace: "metallb-system",
			},
			Spec: v1beta1.IPAddressPoolSpec{
				Addresses: []string{
					"10.20.0.0/16",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pool2",
				Namespace: "metallb-system",
			},
			Spec: v1beta1.IPAddressPoolSpec{
				Addresses: []string{
					"10.10.0.0/16",
				},
				AllocateTo: &v1beta1.ServiceAllocation{
					Namespaces: []string{"foo", "bar"},
				},
			},
		},
	}
	bgpAdvs := []v1beta1.BGPAdvertisement{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "adv1",
				Namespace: "metallb-system",
			},
			Spec: v1beta1.BGPAdvertisementSpec{
				Communities: []string{"bar"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "adv2",
				Namespace: "metallb-system",
			},
			Spec: v1beta1.BGPAdvertisementSpec{
				Communities: []string{"bar2"},
			},
		},
	}
	l2Advs := []v1beta1.L2Advertisement{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "l2adv1",
				Namespace: "metallb-system",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "l2adv2",
				Namespace: "metallb-system",
			},
			Spec: v1beta1.L2AdvertisementSpec{
				Interfaces: []string{"foo"},
			},
		},
	}

	communities := []v1beta1.Community{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "community",
				Namespace: "metallb-system",
			},
			Spec: v1beta1.CommunitySpec{
				Communities: []v1beta1.CommunityAlias{
					{
						Name:  "bar",
						Value: "64512:1234",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "community1",
				Namespace: "metallb-system",
			},
			Spec: v1beta1.CommunitySpec{
				Communities: []v1beta1.CommunityAlias{
					{
						Name:  "bar2",
						Value: "64512:1235",
					},
				},
			},
		},
	}
	namespaces := []v1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bar",
			},
		},
	}
	nodes := []v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node2",
			},
		},
	}

	resources := config.ClusterResources{
		Pools:           pools,
		Peers:           peers,
		BFDProfiles:     bfdProfiles,
		L2Advs:          l2Advs,
		BGPAdvs:         bgpAdvs,
		Communities:     communities,
		PasswordSecrets: map[string]v1.Secret{},
		Nodes:           nodes,
		Namespaces:      namespaces,
	}

	firstConfig, err := toConfig(resources, config.DontValidate)

	if err != nil {
		t.Fatalf("conversion failed, err %v", err)
	}
	seed := time.Now().UnixNano()
	rand.New(rand.NewSource(seed))

	// Here we check the stability of the conversion, by shuffling the
	// order of the resources and checking that the output is the same.
	for i := 0; i < 100; i++ {
		shuffleObjects(resources.Peers)
		shuffleObjects(resources.BFDProfiles)
		shuffleObjects(resources.Pools)
		shuffleObjects(resources.BGPAdvs)
		shuffleObjects(resources.L2Advs)
		shuffleObjects(resources.Communities)
		shuffleObjects(resources.Nodes)
		shuffleObjects(resources.Namespaces)

		config, err := toConfig(resources, config.DontValidate)

		if err != nil {
			t.Fatalf("conversion failed, seed %d, %v", seed, err)
		}

		if !reflect.DeepEqual(config, firstConfig) {
			t.Fatalf("conversion is not stable, seed %d, current %v\n previous %v\n", seed, spew.Sdump(config), spew.Sdump(firstConfig))
		}
	}
}

func shuffleObjects[T any](toShuffle []T) {
	rand.Shuffle(len(toShuffle), func(i, j int) { toShuffle[i], toShuffle[j] = toShuffle[j], toShuffle[i] })
}
