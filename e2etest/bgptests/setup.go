// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	frrcontainer "go.universe.tf/e2etest/pkg/frr/container"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/metallb"
	testservice "go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
)

func setupBGPService(f *framework.Framework, pairingIPFamily ipfamily.Family, poolAddresses []string, peers []*frrcontainer.FRR, tweak testservice.Tweak) (*jigservice.TestJig, *corev1.Service) {
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
	Expect(err).NotTo(HaveOccurred())

	svc, jig := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)

	ginkgo.By("Checking the service gets an ip assigned")
	for _, i := range svc.Status.LoadBalancer.Ingress {
		ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
		ingressIP := jigservice.GetIngressPoint(&i)
		err = config.ValidateIPInRange(resources.Pools, ingressIP)
		Expect(err).NotTo(HaveOccurred())
	}

	resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{
		{ObjectMeta: metav1.ObjectMeta{Name: "empty"}},
	}
	resources.Peers = metallb.PeersForContainers(peers, pairingIPFamily)

	for _, c := range peers {
		err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
		Expect(err).NotTo(HaveOccurred())
	}

	err = ConfigUpdater.Update(resources)
	Expect(err).NotTo(HaveOccurred())

	for _, c := range peers {
		validateFRRPeeredWithAllNodes(cs, c, pairingIPFamily)
	}
	return jig, svc
}
