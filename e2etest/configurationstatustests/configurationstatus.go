// SPDX-License-Identifier:Apache-2.0

// Package configurationstatustests ...
package configurationstatustests

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/k8sreporter"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"

	"go.universe.tf/e2etest/pkg/metallb"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	ConfigUpdater config.Updater
	Reporter      *k8sreporter.KubernetesReporter
)

var _ = ginkgo.Describe("ConfigurationState", func() {
	var allNodes *corev1.NodeList

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration including configurationstatus/config-status")
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Getting all nodes")
		cs := k8sclient.New()
		allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(allNodes.Items)).To(BeNumerically(">", 0))
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
	})

	ginkgo.It("should have condition frrk8sReconcilerValid true when valid FRRK8S-MODE config applied", func() {
		resources := config.Resources{
			Peers: []metallbv1beta2.BGPPeer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-peer",
					},
					Spec: metallbv1beta2.BGPPeerSpec{
						MyASN:   64512,
						ASN:     64513,
						Address: "10.0.0.1",
					},
				},
			},
		}

		ginkgo.By("Applying valid BGP configuration")
		err := ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Checking all speakers report frrk8sReconcilerValid condition true")
		Eventually(func() error {
			var errs []error
			for _, node := range allNodes.Items {
				want := metav1.Condition{
					Type:   "frrk8sReconcilerValid",
					Status: metav1.ConditionTrue,
					Reason: "SyncStateSuccess",
				}
				if err := checkCondition("speaker-"+node.Name, want); err != nil {
					errs = append(errs, fmt.Errorf("condition check failed for node %s: %w", node.Name, err))
				}
			}
			return errors.Join(errs...)
		}, 30*time.Second, 5*time.Second).Should(Succeed())
	})

	ginkgo.It("should have condition frrk8sReconcilerValid false when FRRConfiguration conflicts with MetalLB FRRK8S-MODE config", func() {
		var err error
		nodeName := allNodes.Items[0].Name
		ginkgo.By("Waiting for FRRConfiguration to be empty for node " + nodeName)
		Eventually(func() error {
			frrConfig := &frrv1beta1.FRRConfiguration{}
			if err := ConfigUpdater.Client().Get(context.Background(), types.NamespacedName{
				Name:      "metallb-" + nodeName,
				Namespace: metallb.Namespace,
			}, frrConfig); err != nil {
				return err
			}
			if len(frrConfig.Spec.BGP.Routers) > 0 {
				return fmt.Errorf("FRRConfiguration still has %d routers", len(frrConfig.Spec.BGP.Routers))
			}
			return nil
		}, 30*time.Second, 1*time.Second).Should(Succeed())

		ginkgo.By("Creating FRRConfiguration directly with ASN 65000 for node" + nodeName)
		directFRRConfig := frrv1beta1.FRRConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "external",
				Namespace: metallb.FRRK8sNamespace,
			},
			Spec: frrv1beta1.FRRConfigurationSpec{
				NodeSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/hostname": nodeName,
					},
				},
				BGP: frrv1beta1.BGPConfig{
					Routers: []frrv1beta1.Router{
						{
							ASN: 65000,
							Neighbors: []frrv1beta1.Neighbor{
								{
									ASN:     65001,
									Address: "192.168.1.1",
								},
							},
						},
					},
				},
			},
		}

		err = ConfigUpdater.Client().Create(context.Background(), &directFRRConfig)
		Expect(err).NotTo(HaveOccurred(), "applying direct FRRConfig")
		ginkgo.DeferCleanup(func() error {
			err := ConfigUpdater.Client().Delete(context.Background(), &directFRRConfig)
			Expect(err).NotTo(HaveOccurred())

			return nil
		})

		ginkgo.By("Applying MetalLB BGP configuration with different ASN 64512")
		resources := config.Resources{
			Peers: []metallbv1beta2.BGPPeer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-peer",
					},
					Spec: metallbv1beta2.BGPPeerSpec{
						MyASN:   64512,
						ASN:     64513,
						Address: "10.0.0.1",
					},
				},
			},
		}
		err = ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())
		ginkgo.DeferCleanup(func() error {
			err := ConfigUpdater.Remove(resources)
			Expect(err).NotTo(HaveOccurred())
			return nil
		})

		wantCondition := metav1.Condition{
			Type:    "frrk8sReconcilerValid",
			Status:  metav1.ConditionFalse,
			Reason:  "ConfigError",
			Message: "failed to create or update frr configuration: admission webhook \"frrconfigurationsvalidationwebhook.metallb.io\" denied the request: different asns (65000 != 64512) specified for same vrf: \nresource is invalid for node " + nodeName,
		}

		ginkgo.By("Checking frrk8sReconciler reports error condition for conflicting configuration")
		Eventually(func() error {
			return checkCondition("speaker-"+nodeName, wantCondition)
		}, 30*time.Second, 5*time.Second).Should(Succeed())

		// TODO
		// ginkgo.By("Verifying Ready condition is False when component fails")
		// Eventually(func() error {
		// 	want := metav1.Condition{
		// 		Type:   "Ready",
		// 		Status: metav1.ConditionFalse,
		// 		Reason: "speaker-" + nodeName + "/frrk8sReconcilerValid",
		// 	}
		// 	return checkCondition(want)
		// }, 30*time.Second, 5*time.Second).Should(Succeed())
	})

	ginkgo.It("should have condition poolReconcilerValid false when two clients apply overlapping IPAddressPools", func() {
		ginkgo.By("Client 1: Applying first IPAddressPool")
		resources1 := config.Resources{
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "client1-pool",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							"192.168.10.0/24",
						},
					},
				},
			},
		}

		err := ConfigUpdater.Update(resources1)
		Expect(err).NotTo(HaveOccurred())
		ginkgo.DeferCleanup(func() error {
			err := ConfigUpdater.Remove(resources1)
			Expect(err).NotTo(HaveOccurred())
			return nil
		})

		ginkgo.By("Verifying poolReconciler reports success for first pool")
		Eventually(func() error {
			want := metav1.Condition{
				Type:   "poolReconcilerValid",
				Status: metav1.ConditionTrue,
				Reason: "SyncStateSuccess",
			}
			return checkCondition("controller", want)
		}, 30*time.Second, 5*time.Second).Should(Succeed())

		ginkgo.By("Client 2: Applying second IPAddressPool with overlapping range")
		resources2 := config.Resources{
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "client2-pool",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							"192.168.10.100-192.168.10.200",
						},
					},
				},
			},
		}

		err = ConfigUpdater.Update(resources2)
		Expect(err).NotTo(HaveOccurred())
		ginkgo.DeferCleanup(func() error {
			err := ConfigUpdater.Remove(resources2)
			Expect(err).NotTo(HaveOccurred())
			return nil
		})

		wantCondition := metav1.Condition{
			Type:    "poolReconcilerValid",
			Status:  metav1.ConditionFalse,
			Reason:  "ConfigError",
			Message: "failed to parse configuration: CIDR \"192.168.10.100/32\" in pool \"client2-pool\" overlaps with already defined CIDR \"192.168.10.0/24\"",
		}

		ginkgo.By("Checking poolReconciler reports error condition for overlapping pools")
		Eventually(func() error {
			return checkCondition("controller", wantCondition)
		}, 30*time.Second, 5*time.Second).Should(Succeed())
	})

})

func waitFor(configStatusName, conditionType string, expectedStatus metav1.ConditionStatus) error {
	return wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 50*time.Second, true,
		func(ctx context.Context) (bool, error) {
			var configStatus metallbv1beta1.ConfigurationState
			if err := ConfigUpdater.Client().Get(ctx, types.NamespacedName{
				Name:      configStatusName,
				Namespace: metallb.Namespace,
			}, &configStatus); err != nil {
				return false, nil
			}

			cond := meta.FindStatusCondition(configStatus.Status.Conditions, conditionType)
			if cond == nil {
				return false, nil
			}

			return cond.Status == expectedStatus, nil
		})
}

func checkCondition(configStatusName string, want metav1.Condition) error {
	k8sClient := ConfigUpdater.Client()
	var configStatus metallbv1beta1.ConfigurationState
	if err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      configStatusName,
		Namespace: metallb.Namespace,
	}, &configStatus); err != nil {
		return fmt.Errorf("failed to get ConfigurationState: %w", err)
	}

	got := meta.FindStatusCondition(configStatus.Status.Conditions, want.Type)
	if got == nil {
		return fmt.Errorf("condition %s not found", want.Type)
	}

	opts := cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime", "ObservedGeneration")
	if diff := cmp.Diff(want, *got, opts); diff != "" {
		return fmt.Errorf("condition mismatch (-want +got):\n%s", diff)
	}

	return nil
}
