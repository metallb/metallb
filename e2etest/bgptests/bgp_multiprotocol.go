// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	testservice "go.universe.tf/metallb/e2etest/pkg/service"
	metallbconfig "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/ipfamily"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	frrconfig "go.universe.tf/metallb/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = ginkgo.Describe("BGP Multiprotocol", func() {
	var cs clientset.Interface
	var f *framework.Framework

	emptyBGPAdvertisement := metallbv1beta1.BGPAdvertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty",
		},
	}

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			dumpBGPInfo(ReportPath, ginkgo.CurrentGinkgoTestDescription().TestText, cs, f)
			k8s.DumpInfo(Reporter, ginkgo.CurrentGinkgoTestDescription().TestText)
		}
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)

		for _, c := range FRRContainers {
			err := c.UpdateBGPConfigFile(frrconfig.Empty)
			framework.ExpectNoError(err)
		}
	})

	f = framework.NewDefaultFramework("bgp")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
	})

	ginkgo.Context("Multiprotocol", func() {
		ginkgo.DescribeTable("should advertise both ipv4 and ipv6 addresses with", func(pairingFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {
			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "mp-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: poolAddresses,
						},
					},
				},
				Peers:   metallb.PeersForContainers(FRRContainers, pairingFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
					container.MultiProtocol = frrconfig.MultiProtocolEnabled
				})
				framework.ExpectNoError(err)
			}

			svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)
			defer testservice.Delete(cs, svc)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingFamily)
			}
			for _, c := range FRRContainers {
				validateService(cs, svc, allNodes.Items, c)
			}
		},
			ginkgo.Entry("DUALSTACK - via ipv4",
				ipfamily.IPv4, []string{v4PoolAddresses, v6PoolAddresses}, func(svc *corev1.Service) {
					testservice.TrafficPolicyCluster(svc)
					testservice.DualStack(svc)
				}),
			ginkgo.Entry("DUALSTACK - via ipv6",
				ipfamily.IPv6, []string{v4PoolAddresses, v6PoolAddresses}, func(svc *corev1.Service) {
					testservice.TrafficPolicyCluster(svc)
					testservice.DualStack(svc)
				}),
			ginkgo.Entry("DUALSTACK - via both, advertising ipv6 only",
				ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses}, func(svc *corev1.Service) {
					testservice.TrafficPolicyCluster(svc)
					testservice.DualStack(svc)
					testservice.ForceV6(svc)
				}),
			ginkgo.Entry("DUALSTACK - via both, advertising ipv4 only",
				ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses}, func(svc *corev1.Service) {
					testservice.TrafficPolicyCluster(svc)
					testservice.DualStack(svc)
					testservice.ForceV4(svc)
				}),
		)

		ginkgo.DescribeTable("should propagate the localpreference and the communities to both ipv4 and ipv6 addresses",
			func(ipFamily ipfamily.Family) {
				emptyAdvertisement := metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{
						Name: "empty",
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						IPAddressPools: []string{"bgp-with-no-advertisement"},
					},
				}

				pool := metallbv1beta1.IPAddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "bgp-with-advertisement",
						Labels: map[string]string{"test": "bgp-with-advertisement"},
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{"192.168.10.0/24",
							"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18"},
					},
				}

				resources := metallbconfig.ClusterResources{
					Peers: metallb.PeersForContainers(FRRContainers, ipFamily),
					Pools: []metallbv1beta1.IPAddressPool{pool},
					BGPAdvs: []metallbv1beta1.BGPAdvertisement{
						emptyAdvertisement,
						{
							ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
							Spec: metallbv1beta1.BGPAdvertisementSpec{
								LocalPref:      50,
								Communities:    []string{CommunityNoAdv},
								IPAddressPools: []string{"bgp-with-advertisement"},
							},
						},
					},
				}

				for _, c := range FRRContainers {
					err := frrcontainer.PairWithNodes(cs, c, ipFamily, func(container *frrcontainer.FRR) {
						container.MultiProtocol = frrconfig.MultiProtocolEnabled
					})
					framework.ExpectNoError(err)
				}

				err := ConfigUpdater.Update(resources)
				framework.ExpectNoError(err)

				for _, c := range FRRContainers {
					validateFRRPeeredWithAllNodes(cs, c, ipFamily)
				}

				svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "service-with-adv",
					testservice.TrafficPolicyCluster,
					testservice.DualStack)

				defer testservice.Delete(cs, svc)

				allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				framework.ExpectNoError(err)

				for _, c := range FRRContainers {
					validateService(cs, svc, allNodes.Items, c)
					for _, ip := range svc.Status.LoadBalancer.Ingress {
						ingressIP := e2eservice.GetIngressPoint(&ip)
						Eventually(func() error {
							addressFamily := ipfamily.ForAddress(net.ParseIP(ingressIP))
							routes, err := frr.RoutesForCommunity(c, CommunityNoAdv, addressFamily)
							if err != nil {
								return err
							}
							if _, ok := routes[ingressIP]; !ok {
								return fmt.Errorf("%s route not found for community %s", ingressIP, CommunityNoAdv)
							}
							// LocalPref check is only valid for iBGP sessions
							if strings.Contains(c.Name, "ibgp") {
								localPrefix, err := frr.LocalPrefForPrefix(c, ingressIP, addressFamily)
								if err != nil {
									return err
								}
								if localPrefix != 50 {
									return fmt.Errorf("%s %s not matching local pref", c.Name, ingressIP)
								}
							}
							return nil
						}, 1*time.Minute, 1*time.Second).Should(BeNil())
					}
				}
			},
			ginkgo.Entry("with DUALSTACK via ipv4",
				ipfamily.IPv4),
			ginkgo.Entry("with DUALSTACK via ipv6",
				ipfamily.IPv6),
		)
	})
})
