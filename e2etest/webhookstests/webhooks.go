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
	"fmt"
	"strings"
	"time"

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

var _ = ginkgo.Describe("WEBHOOKS", func() {
	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.Context("Validate AddressPool Webhook", func() {
		ginkgo.AfterEach(func() {
			// Clean previous configuration.
			err := ConfigUpdater.Clean()
			framework.ExpectNoError(err)
		})

		ginkgo.It("Should recognize overlapping addresses in two AddressPools", func() {
			ginkgo.By("Creating first AddressPool")
			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "webhooks-test1",
						},
						Spec: metallbv1beta1.IPPoolSpec{
							Addresses: []string{
								"1.1.1.1-1.1.1.100",
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("Creating second AddressPool with overlapping addresses defined by address range")
			resources.Pools = append(resources.Pools, metallbv1beta1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "webhooks-test2",
				},
				Spec: metallbv1beta1.IPPoolSpec{
					Addresses: []string{
						"1.1.1.15-1.1.1.20",
					},
				},
			})
			err = ConfigUpdater.Update(resources)
			framework.ExpectNotEqual(err, BeNil())

			if !strings.Contains(fmt.Sprint(err), "overlaps with already defined CIDR") {
				Expect(err).ToNot(HaveOccurred())
			}

			ginkgo.By("Creating second valid AddressPool")
			resources.Pools[1].Spec.Addresses = []string{"1.1.1.101-1.1.1.200"}
			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("Updating second AddressPool addresses to overlapping addresses defined by network prefix")
			resources.Pools[1].Spec.Addresses = []string{"1.1.1.0/24"}
			err = ConfigUpdater.Update(resources)
			framework.ExpectNotEqual(err, BeNil())

			if !strings.Contains(fmt.Sprint(err), "overlaps with already defined CIDR") {
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})

	ginkgo.Context("Validate BGPAdvertisement Webhook", func() {
		ginkgo.AfterEach(func() {
			ginkgo.By("Clearing any previous configuration")

			err := ConfigUpdater.Clean()
			framework.ExpectNoError(err)
		})

		ginkgo.It("Should recognize invalid AggregationLength", func() {
			ginkgo.By("Creating AddressPool")
			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool-webhooks-test",
						},
						Spec: metallbv1beta1.IPPoolSpec{
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
						IPPools:           []string{"pool-webhooks-test"},
					},
				},
			}

			Eventually(func() error {
				err = ConfigUpdater.Update(resources)
				return err
			}, 1*time.Minute, 1*time.Second).ShouldNot(BeNil())

			if !strings.Contains(fmt.Sprint(err), "invalid aggregation length 26: prefix 28 in this pool is more specific than the aggregation length for addresses 1.1.1.0/28") {
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})

	ginkgo.Context("Validate BGPPeer Webhook", func() {
		ginkgo.AfterEach(func() {
			ginkgo.By("Clearing any previous configuration")

			err := ConfigUpdater.Clean()
			framework.ExpectNoError(err)
		})

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
			framework.ExpectNotEqual(err, BeNil())

			if !strings.Contains(fmt.Sprint(err), "Invalid BGPPeer address") {
				Expect(err).ToNot(HaveOccurred())
			}

			ginkgo.By("Updating BGPPeer to use valid peer address")
			resources.Peers[0].Spec.Address = "1.1.1.1"
			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)
		})

		ginkgo.It("Should reject invalid Keepalive time", func() {
			ginkgo.By("Creating BGPPeer")
			resources := metallbconfig.ClusterResources{
				Peers: []metallbv1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "webhooks-test",
						},
						Spec: metallbv1beta2.BGPPeerSpec{
							Address:       "1.1.1.1",
							ASN:           64500,
							MyASN:         1000,
							KeepaliveTime: metav1.Duration{Duration: 180 * time.Second},
							HoldTime:      metav1.Duration{Duration: 90 * time.Second},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNotEqual(err, BeNil())

			if !strings.Contains(fmt.Sprint(err), "Invalid keepalive time") {
				Expect(err).ToNot(HaveOccurred())
			}

			ginkgo.By("Updating BGPPeer to use valid keepalive time")
			resources.Peers[0].Spec.KeepaliveTime = metav1.Duration{Duration: 90 * time.Second}
			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)
		})

		ginkgo.It("Should allow multiple BGPPeers with the same router-id", func() {
			ginkgo.By("Creating multiple BGPpeers")
			resources := metallbconfig.ClusterResources{
				Peers: []metallbv1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "webhooks-test1",
						},
						Spec: metallbv1beta2.BGPPeerSpec{
							Address:  "1.1.1.1",
							ASN:      64500,
							MyASN:    1000,
							RouterID: "10.10.10.10",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "webhooks-test2",
						},
						Spec: metallbv1beta2.BGPPeerSpec{
							Address:  "2.2.2.2",
							ASN:      64600,
							MyASN:    1000,
							RouterID: "10.10.10.10",
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)
		})
	})
})
