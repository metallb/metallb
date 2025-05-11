/*
Copyright 2016 The Kubernetes Authors.
Copyright 2021 The MetalLB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// https://github.com/kubernetes/kubernetes/blob/92aff21558831b829fbc8cbca3d52edc80c01aa3/test/e2e/network/loadbalancer.go#L878

package webhookstests

import (
	"context"
	"fmt"
	"net"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/metallb"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/k8sreporter"

	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/k8sclient"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ConfigUpdater        config.Updater
	ConfigUpdaterOtherNS config.Updater
	Reporter             *k8sreporter.KubernetesReporter
)

var _ = ginkgo.Describe("Webhooks", func() {
	var cs clientset.Interface
	ginkgo.BeforeEach(func() {
		cs = k8sclient.New()

		ginkgo.By("Clearing any previous configuration")
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}

		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Context("For IPAddressPool", func() {
		ginkgo.It("Should recognize overlapping addresses in two AddressPools", func() {
			ginkgo.By("Creating first IPAddressPool")
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "webhooks-test1",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.1.1.1-1.1.1.100",
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating second IPAddressPool with overlapping addresses defined by address range")
			resources.Pools = append(resources.Pools, metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "webhooks-test2",
				},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"1.1.1.15-1.1.1.20",
					},
				},
			})
			err = ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlaps with already defined CIDR"))

			ginkgo.By("Creating second valid IPAddressPool")
			resources.Pools[1].Spec.Addresses = []string{"1.1.1.101-1.1.1.200"}
			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Updating second IPAddressPool addresses to overlapping addresses defined by network prefix")
			resources.Pools[1].Spec.Addresses = []string{"1.1.1.0/24"}
			err = ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("overlaps with already defined CIDR"))
		})

		ginkgo.DescribeTable("IPAddressPool with overlapping addresses of the nodes",
			func(ipFamily ipfamily.Family) {
				nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				nodeIps, err := k8s.NodeIPsForFamily(nodes.Items, ipFamily, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeIps)).To(BeNumerically(">", 0), "empty node ips list")
				nodeIP := net.ParseIP(nodeIps[0])
				cidr := &net.IPNet{
					IP:   nodeIP,
					Mask: net.CIDRMask(32, 32),
				}
				if ipFamily == ipfamily.IPv6 {
					cidr.Mask = net.CIDRMask(128, 128)
				}

				ginkgo.By("Creating IPAddressPool")
				var nodeCIDRs []string
				nodeCIDRs = append(nodeCIDRs, cidr.String())
				resources := config.Resources{
					Pools: []metallbv1beta1.IPAddressPool{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "webhooks-test3",
							},
							Spec: metallbv1beta1.IPAddressPoolSpec{
								Addresses: nodeCIDRs,
							},
						},
					},
				}
				err = ConfigUpdater.Update(resources)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("contains nodeIp"))
			},
			ginkgo.Entry("IPV4", ipfamily.IPv4),
			ginkgo.Entry("IPV6", ipfamily.IPv6),
		)
	})

	ginkgo.Context("for BGPAdvertisement", func() {
		ginkgo.It("Should recognize invalid AggregationLength", func() {
			ginkgo.By("Creating AddressPool")
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool-webhooks-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.1.1.0/28",
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating BGPAdvertisement")
			resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "adv-webhooks-test",
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						AggregationLength: ptr.To(int32(26)),
						IPAddressPools:    []string{"pool-webhooks-test"},
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid aggregation length 26: prefix 28 in this pool is more specific than the aggregation length for addresses 1.1.1.0/28"))
		})
	})

	ginkgo.Context("For BGPPeer", func() {
		ginkgo.It("Should reject invalid BGPPeer IP address", func() {
			ginkgo.By("Creating BGPPeer")
			resources := config.Resources{
				Peers: []metallbv1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "webhooks-test",
						},
						Spec: metallbv1beta2.BGPPeerSpec{
							Address: "1.1.1",
							ASN:     64500,
							MyASN:   1000,
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid BGPPeer address"))
		})
	})

	ginkgo.Context("For Community", func() {
		ginkgo.DescribeTable("reject a new invalid Community", func(community, expectedError string) {
			ginkgo.By("Creating invalid Community")
			resources := config.Resources{
				Communities: []metallbv1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test",
						},
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "INVALID_COMMUNITY",
									Value: community,
								},
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError))
		},
			ginkgo.Entry("in legacy format", "99999999:1", "invalid community value: invalid section"),
			ginkgo.Entry("in large format", "lar:123:9999:123", "expected community to be of format large"))

		ginkgo.DescribeTable("reject an update to an invalid Community", func(community, expectedError string) {
			ginkgo.By("Creating Community")
			resources := config.Resources{
				Communities: []metallbv1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test",
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Updating community")
			resources.Communities[0].Spec = metallbv1beta1.CommunitySpec{
				Communities: []metallbv1beta1.CommunityAlias{
					{
						Name:  "INVALID_COMMUNITY",
						Value: community,
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError))
		},
			ginkgo.Entry("in legacy format", "99999999:1", "invalid community value: invalid section"),
			ginkgo.Entry("in large format", "lar:123:9999:123", "expected community to be of format large"))

		ginkgo.DescribeTable("reject Community duplications", func(community string) {
			ginkgo.By("Creating duplicates in the same Community")
			resources := config.Resources{
				Communities: []metallbv1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test",
						},
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "DUP_COMMUNITY",
									Value: community,
								},
								{
									Name:  "DUP_COMMUNITY",
									Value: community,
								},
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate definition of community"))

			ginkgo.By("Creating duplicates across two different Communities")
			resources = config.Resources{
				Communities: []metallbv1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test1",
						},
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "DUP_COMMUNITY",
									Value: community,
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test2",
						},
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "DUP_COMMUNITY",
									Value: community,
								},
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate definition of community"))
		},
			ginkgo.Entry("in legacy format", "1111:2222"),
			ginkgo.Entry("FRR - in large format", "large:123:9999:123"))
	})

	ginkgo.Context("For BFDProfile", func() {
		testBFDProfile := metallbv1beta1.BFDProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bfdprofile-webhooks-test",
				Namespace: metallb.Namespace,
			},
		}
		testPeer := metallbv1beta2.BGPPeer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bgppeer-webhooks-test",
				Namespace: metallb.Namespace,
			},
			Spec: metallbv1beta2.BGPPeerSpec{
				BFDProfile: testBFDProfile.Name,
				ASN:        1234,
				MyASN:      1234,
				Address:    "1.2.3.4",
			},
		}
		ginkgo.It("Should produce an error when deleting a profile used by a BGPPeer", func() {
			ginkgo.By("Creating BFDProfile and BGPPeer")
			resources := config.Resources{
				BFDProfiles: []metallbv1beta1.BFDProfile{testBFDProfile},
				Peers:       []metallbv1beta2.BGPPeer{testPeer},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Deleting the profile used by BGPPeer")
			err = ConfigUpdater.Client().Delete(context.TODO(), &testBFDProfile, &client.DeleteOptions{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to delete BFDProfile"))

			ginkgo.By("Deleting the BGPPeer")
			err = ConfigUpdater.Client().Delete(context.TODO(), &testPeer, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Deleting the profile not used by BGPPeer")
			err = ConfigUpdater.Client().Delete(context.TODO(), &testBFDProfile, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = ginkgo.DescribeTable("Webhooks namespace validation",
	func(resources *config.Resources) {
		err := ConfigUpdaterOtherNS.Update(*resources)
		Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("resource must be created in %s namespace", metallb.Namespace))))
	},
	ginkgo.Entry("Should reject creating BFDProfile in a different namespace", &config.Resources{
		BFDProfiles: []metallbv1beta1.BFDProfile{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bfdprofile-webhooks-test",
				},
			},
		},
	}),
	ginkgo.Entry("Should reject creating IPAddressPool in a different namespace", &config.Resources{
		Pools: []metallbv1beta1.IPAddressPool{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "webhooks-test1",
				},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
				},
			},
		},
	}),
	ginkgo.Entry("Should reject creating BGPPeer in a different namespace", &config.Resources{
		Peers: []metallbv1beta2.BGPPeer{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "webhooks-test",
				},
				Spec: metallbv1beta2.BGPPeerSpec{
					Address: "1.1.1",
					ASN:     64500,
					MyASN:   1000,
				},
			},
		},
	}),
	ginkgo.Entry("Should reject creating BGPAdvertisement in a different namespace", &config.Resources{
		BGPAdvs: []metallbv1beta1.BGPAdvertisement{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "adv-webhooks-test",
				},
				Spec: metallbv1beta1.BGPAdvertisementSpec{
					AggregationLength: ptr.To(int32(26)),
					IPAddressPools:    []string{"pool-webhooks-test"},
				},
			},
		},
	}),
	ginkgo.Entry("Should reject creating L2Advertisement in a different namespace", &config.Resources{
		L2Advs: []metallbv1beta1.L2Advertisement{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "l2adv-webhooks-test",
				},
			},
		},
	}),
	ginkgo.Entry("Should reject creating Community in a different namespace", &config.Resources{
		Communities: []metallbv1beta1.Community{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "community-webhooks-test",
				},
				Spec: metallbv1beta1.CommunitySpec{
					Communities: []metallbv1beta1.CommunityAlias{
						{
							Name:  "test-community",
							Value: "1234:1",
						},
					},
				},
			},
		},
	}),
)
