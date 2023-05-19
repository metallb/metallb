// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"github.com/onsi/ginkgo/v2"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/e2etest/pkg/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	"go.universe.tf/metallb/e2etest/pkg/ipfamily"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	testservice "go.universe.tf/metallb/e2etest/pkg/service"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

func setupBGPService(f *framework.Framework, pairingIPFamily ipfamily.Family, poolAddresses []string, peers []*frrcontainer.FRR, tweak testservice.Tweak) (*e2eservice.TestJig, *corev1.Service) {
	cs := f.ClientSet
	resources := config.Resources{
		Pools: []metallbv1beta1.IPAddressPool{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bgp-test",
				},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: poolAddresses,
				},
			},
		},
	}

	err := ConfigUpdater.Update(resources)
	framework.ExpectNoError(err)

	svc, jig := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)

	ginkgo.By("Checking the service gets an ip assigned")
	for _, i := range svc.Status.LoadBalancer.Ingress {
		ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
		ingressIP := e2eservice.GetIngressPoint(&i)
		err = config.ValidateIPInRange(resources.Pools, ingressIP)
		framework.ExpectNoError(err)
	}

	resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{
		{ObjectMeta: metav1.ObjectMeta{Name: "empty"}},
	}
	resources.Peers = metallb.PeersForContainers(peers, pairingIPFamily)

	for _, c := range peers {
		err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
		framework.ExpectNoError(err)
	}

	err = ConfigUpdater.Update(resources)
	framework.ExpectNoError(err)

	for _, c := range peers {
		validateFRRPeeredWithAllNodes(cs, c, pairingIPFamily)
	}
	return jig, svc
}
