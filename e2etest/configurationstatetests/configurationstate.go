// SPDX-License-Identifier:Apache-2.0

// Package configurationstatetests contains end-to-end tests for the ConfigurationState API.
package configurationstatetests

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ConfigUpdater config.Updater
	Reporter      *k8sreporter.KubernetesReporter

	validStatus = metallbv1beta1.ConfigurationStateStatus{
		Result:       metallbv1beta1.ConfigurationResultValid,
		ErrorSummary: "",
	}
)

var _ = ginkgo.Describe("ConfigurationState", func() {
	var allNodes *corev1.NodeList

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Getting all nodes")
		cs := k8sclient.New()
		allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(allNodes.Items)).To(BeNumerically(">", 0))

		ginkgo.By("Verifying all ConfigurationStates exist and are valid")
		Eventually(func() error {
			return allStatesExist(allNodes)
		}, 30*time.Second, 2*time.Second).Should(Succeed())
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
	})

	ginkgo.It("speaker should have invalid result when BGPPeer references secret with wrong type", func() {
		stateName := "speaker-" + allNodes.Items[0].Name
		wantStatus := metallbv1beta1.ConfigurationStateStatus{
			Result:       metallbv1beta1.ConfigurationResultInvalid,
			ErrorSummary: "configuration error: parsing peer peer1 secret type mismatch on \"metallb-system\"/\"bgp-password\", type \"kubernetes.io/basic-auth\" is expected \nfailed to parse peer peer1 password secret",
		}

		ginkgo.By("Creating secret with wrong type and BGPPeer referencing it")
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bgp-password",
				Namespace: metallb.Namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"password": "mypassword",
			},
		}
		err := ConfigUpdater.Client().Create(context.Background(), secret)
		Expect(err).NotTo(HaveOccurred())
		ginkgo.DeferCleanup(func() {
			ConfigUpdater.Client().Delete(context.Background(), secret)
		})

		resources := config.Resources{
			Peers: []metallbv1beta2.BGPPeer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "peer1",
					},
					Spec: metallbv1beta2.BGPPeerSpec{
						MyASN:   64512,
						ASN:     64513,
						Address: "192.168.1.1",
						PasswordSecret: corev1.SecretReference{
							Name: "bgp-password",
						},
					},
				},
			},
		}

		err = ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Verifying status has invalid result with error message")
		Eventually(func() error {
			return stateMatches(stateName, wantStatus)
		}, 30*time.Second, 5*time.Second).Should(Succeed())

		ginkgo.By("Recreating secret with correct type")
		err = ConfigUpdater.Client().Delete(context.Background(), secret) // field is immutable, need to delete first
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Ensuring secret is fully deleted")
		Eventually(func() bool {
			var s corev1.Secret
			err := ConfigUpdater.Client().Get(context.Background(), types.NamespacedName{
				Name:      "bgp-password",
				Namespace: metallb.Namespace,
			}, &s)
			return apierrors.IsNotFound(err)
		}, 30*time.Second, 1*time.Second).Should(Equal(true))

		secret.ResourceVersion = ""
		secret.UID = ""
		secret.Type = corev1.SecretTypeBasicAuth
		err = ConfigUpdater.Client().Create(context.Background(), secret)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Verifying status has valid result")
		Eventually(func() error {
			return stateMatches(stateName, validStatus)
		}, 60*time.Second, 5*time.Second).Should(Succeed())
	})

	ginkgo.It("speaker should have invalid result when BFD profile is missing", func() {
		stateName := "speaker-" + allNodes.Items[0].Name
		wantStatus := metallbv1beta1.ConfigurationStateStatus{
			Result:       metallbv1beta1.ConfigurationResultInvalid,
			ErrorSummary: "configuration error: peer peer1 referencing non existing bfd profile my-bfd-profile",
		}

		ginkgo.By("Creating BGPPeer referencing non-existent BFD profile")
		resources := config.Resources{
			Peers: []metallbv1beta2.BGPPeer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "peer1",
					},
					Spec: metallbv1beta2.BGPPeerSpec{
						MyASN:      64512,
						ASN:        64513,
						Address:    "192.168.1.1",
						BFDProfile: "my-bfd-profile",
					},
				},
			},
		}

		err := ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Verifying status has invalid result with error message")
		Eventually(func() error {
			return stateMatches(stateName, wantStatus)
		}, 30*time.Second, 5*time.Second).Should(Succeed())

		ginkgo.By("Creating missing BFD profile")
		resources.BFDProfiles = []metallbv1beta1.BFDProfile{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-bfd-profile",
				},
			},
		}

		err = ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Verifying status has valid result")
		Eventually(func() error {
			return stateMatches(stateName, validStatus)
		}, 60*time.Second, 5*time.Second).Should(Succeed())
	})

})

func allStatesExist(allNodes *corev1.NodeList) error {
	k8sClient := ConfigUpdater.Client()

	want := []metallbv1beta1.ConfigurationState{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "controller",
				Namespace: metallb.Namespace,
				Labels: map[string]string{
					"metallb.io/component-type": "controller",
				},
			},
			Status: validStatus,
		},
	}

	for _, node := range allNodes.Items {
		want = append(want, metallbv1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "speaker-" + node.Name,
				Namespace: metallb.Namespace,
				Labels: map[string]string{
					"metallb.io/component-type": "speaker",
					"metallb.io/node-name":      node.Name,
				},
			},
			Status: validStatus,
		})
	}

	var got metallbv1beta1.ConfigurationStateList
	if err := k8sClient.List(context.Background(), &got, client.InNamespace(metallb.Namespace)); err != nil {
		return fmt.Errorf("failed to list ConfigurationStates: %w", err)
	}

	opts := []cmp.Option{
		cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields"),
		cmpopts.IgnoreFields(metallbv1beta1.ConfigurationState{}, "TypeMeta"),
		cmpopts.IgnoreFields(metallbv1beta1.ConfigurationStateStatus{}, "Conditions"),
		cmpopts.SortSlices(func(a, b metallbv1beta1.ConfigurationState) bool {
			return a.Name < b.Name
		}),
	}
	if diff := cmp.Diff(want, got.Items, opts...); diff != "" {
		return fmt.Errorf("ConfigurationState list mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func stateMatches(stateName string, wantStatus metallbv1beta1.ConfigurationStateStatus) error {
	k8sClient := ConfigUpdater.Client()
	var got metallbv1beta1.ConfigurationState
	if err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      stateName,
		Namespace: metallb.Namespace,
	}, &got); err != nil {
		return fmt.Errorf("failed to get ConfigurationState: %w", err)
	}

	if got.Status.Result != wantStatus.Result {
		return fmt.Errorf("expected Result=%s, got Result=%s, ErrorSummary=%s", wantStatus.Result, got.Status.Result, got.Status.ErrorSummary)
	}
	if got.Status.ErrorSummary != wantStatus.ErrorSummary {
		return fmt.Errorf("expected ErrorSummary=%q, got ErrorSummary=%q", wantStatus.ErrorSummary, got.Status.ErrorSummary)
	}
	return nil
}
