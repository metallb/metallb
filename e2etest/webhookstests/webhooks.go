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
	"go.universe.tf/metallb/e2etest/pkg/config"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	metallbconfig "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var ConfigUpdater config.Updater

var _ = ginkgo.Describe("Webhooks", func() {
	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.AfterEach(func() {
		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.Context("For IPAddressPool", func() {
		ginkgo.It("Should recognize overlapping addresses in two AddressPools", func() {
			ginkgo.By("Creating first IPAddressPool")
			resources := metallbconfig.ClusterResources{
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
			framework.ExpectNoError(err)

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
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("overlaps with already defined CIDR"))

			ginkgo.By("Creating second valid IPAddressPool")
			resources.Pools[1].Spec.Addresses = []string{"1.1.1.101-1.1.1.200"}
			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("Updating second IPAddressPool addresses to overlapping addresses defined by network prefix")
			resources.Pools[1].Spec.Addresses = []string{"1.1.1.0/24"}
			err = ConfigUpdater.Update(resources)
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("overlaps with already defined CIDR"))
		})
	})

	ginkgo.Context("for Legacy AddressPool", func() {
		ginkgo.It("Should recognize overlapping addresses in two AddressPools", func() {
			ginkgo.By("Creating first AddrssPool")
			resources := metallbconfig.ClusterResources{
				LegacyAddressPools: []metallbv1beta1.AddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "webhooks-test1",
						},
						Spec: metallbv1beta1.AddressPoolSpec{
							Addresses: []string{
								"1.1.1.1-1.1.1.100",
							},
							Protocol: string(metallbconfig.Layer2),
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("Creating second AddressPool with overlapping addresses defined by address range")
			resources.LegacyAddressPools = append(resources.LegacyAddressPools,
				metallbv1beta1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "webhooks-test2",
					},
					Spec: metallbv1beta1.AddressPoolSpec{
						Addresses: []string{
							"1.1.1.15-1.1.1.20",
						},
						Protocol: string(metallbconfig.Layer2),
					},
				},
			)
			err = ConfigUpdater.Update(resources)
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("overlaps with already defined CIDR"))

			ginkgo.By("Creating second valid AddressPool")
			resources.LegacyAddressPools[1].Spec.Addresses = []string{"1.1.1.101-1.1.1.200"}
			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("Updating second AddressPool addresses to overlapping addresses defined by network prefix")
			resources.LegacyAddressPools[1].Spec.Addresses = []string{"1.1.1.0/24"}
			err = ConfigUpdater.Update(resources)
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("overlaps with already defined CIDR"))
		})
	})

	ginkgo.Context("for BGPAdvertisement", func() {
		ginkgo.It("Should recognize invalid AggregationLength", func() {
			ginkgo.By("Creating AddressPool")
			resources := metallbconfig.ClusterResources{
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
			framework.ExpectNoError(err)

			ginkgo.By("Creating BGPAdvertisement")
			resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "adv-webhooks-test",
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						AggregationLength: pointer.Int32Ptr(26),
						IPAddressPools:    []string{"pool-webhooks-test"},
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("invalid aggregation length 26: prefix 28 in this pool is more specific than the aggregation length for addresses 1.1.1.0/28"))
		})
	})

	ginkgo.Context("For BGPPeer", func() {
		ginkgo.It("Should reject invalid BGPPeer IP address", func() {
			ginkgo.By("Creating BGPPeer")
			resources := metallbconfig.ClusterResources{
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
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("invalid BGPPeer address"))
		})
	})

	ginkgo.Context("For Community", func() {
		ginkgo.It("Should reject a new invalid Community", func() {
			ginkgo.By("Creating invalid Community")
			resources := metallbconfig.ClusterResources{
				Communities: []metallbv1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test",
						},
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "INVALID_COMMUNITY",
									Value: "99999999:1",
								},
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("invalid first section of community"))
		})

		ginkgo.It("Should reject an update to an invalid Community", func() {
			ginkgo.By("Creating Community")
			resources := metallbconfig.ClusterResources{
				Communities: []metallbv1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test",
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("Updating community")
			resources.Communities[0].Spec = metallbv1beta1.CommunitySpec{
				Communities: []metallbv1beta1.CommunityAlias{
					{
						Name:  "INVALID_COMMUNITY",
						Value: "99999999:1",
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("invalid first section of community"))
		})

		ginkgo.It("Should reject Community duplications", func() {
			ginkgo.By("Creating duplicates in the same Community")
			resources := metallbconfig.ClusterResources{
				Communities: []metallbv1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test",
						},
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "DUP_COMMUNITY",
									Value: "1111:2222",
								},
								{
									Name:  "DUP_COMMUNITY",
									Value: "1111:2222",
								},
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("duplicate definition of community"))

			ginkgo.By("Creating duplicates across two different Communities")
			resources = metallbconfig.ClusterResources{
				Communities: []metallbv1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community-webhooks-test1",
						},
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "DUP_COMMUNITY",
									Value: "1111:2222",
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
									Value: "1111:2222",
								},
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			framework.ExpectError(err)
			Expect(err.Error()).To(ContainSubstring("duplicate definition of community"))
		})
	})
})
