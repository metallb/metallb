// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/service"
	"go.universe.tf/e2etest/pkg/status"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

const preferredLabel = "l2-preferred-node-test"

var _ = ginkgo.Describe("L2", func() {
	var cs clientset.Interface
	var testNamespace string
	var allNodes *corev1.NodeList

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
		err = k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.BeforeEach(func() {
		cs = k8sclient.New()
		var err error
		testNamespace, err = k8s.CreateTestNamespace(cs, "l2pref")
		Expect(err).NotTo(HaveOccurred())

		allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Clearing any previous configuration")
		err = ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		resources := config.Resources{
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "l2-test"},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{IPV4ServiceRange, IPV6ServiceRange},
					},
				},
			},
		}
		err = ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Context("Preferred Node Selector", func() {

		ginkgo.It("should prefer the labeled node and fall back to the baseline election when the label is removed", func() {
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("need at least two nodes")
			}

			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("Applying a baseline L2Advertisement to observe the natural election winner")
			err := ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{
					{ObjectMeta: metav1.ObjectMeta{Name: "preferred"}},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			natural := waitForNaturalWinner(svc)
			candidate := nonNaturalNode(allNodes.Items, natural)
			Expect(candidate).NotTo(BeEmpty())

			ginkgo.By(fmt.Sprintf("Labeling %s (non-natural winner) as preferred", candidate))
			k8s.AddLabelToNode(candidate, preferredLabel, "true", cs)
			defer k8s.RemoveLabelFromNode(candidate, preferredLabel, cs)

			l2adv := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "preferred"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 100,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{preferredLabel: "true"},
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2adv},
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("Validating the preferred node %s announces the service", candidate))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(candidate))

			ginkgo.By(fmt.Sprintf("Removing the preferred label from %s", candidate))
			k8s.RemoveLabelFromNode(candidate, preferredLabel, cs)

			ginkgo.By(fmt.Sprintf("Validating election falls back to the baseline winner %s", natural))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(natural))
		})

		ginkgo.It("should behave like the baseline advertisement when no node matches the preferred selector", func() {
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("need at least two nodes")
			}

			svc, _ := service.CreateWithBackend(cs, testNamespace, "no-match-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("Applying a baseline L2Advertisement to observe the natural election winner")
			err := ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{
					{ObjectMeta: metav1.ObjectMeta{Name: "preferred-nomatch"}},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			natural := waitForNaturalWinner(svc)

			ginkgo.By("Replacing with a no-match PreferredNodeSelector plus a match-all weight 10 selector")
			l2adv := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "preferred-nomatch"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 100,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{"l2-preferred-node-nomatch": "true"},
							},
						},
						{
							// Empty selector matches every node so every candidate
							// gains the same weight; the announcing node must not
							// move because no preference is differentiated.
							Weight:     10,
							Preference: metav1.LabelSelector{},
						},
					},
				},
			}
			err = ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2adv},
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("Validating the announcing node stays at %s (baseline winner)", natural))
			Consistently(announcingNode(svc), 5*time.Second, time.Second).Should(Equal(natural))
		})

		ginkgo.It("should fail over when the preferred node becomes unschedulable and reclaim when it returns", func() {
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("need at least two nodes")
			}

			svc, _ := service.CreateWithBackend(cs, testNamespace, "failover-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("Applying a baseline L2Advertisement to observe the natural election winner")
			err := ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{
					{ObjectMeta: metav1.ObjectMeta{Name: "preferred-failover"}},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			natural := waitForNaturalWinner(svc)
			preferredCandidate := nonNaturalNode(allNodes.Items, natural)
			Expect(preferredCandidate).NotTo(BeEmpty())

			ginkgo.By(fmt.Sprintf("Labeling %s as preferred", preferredCandidate))
			k8s.AddLabelToNode(preferredCandidate, preferredLabel, "true", cs)
			defer k8s.RemoveLabelFromNode(preferredCandidate, preferredLabel, cs)

			l2adv := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "preferred-failover"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 100,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{preferredLabel: "true"},
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2adv},
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("Validating the preferred node %s announces initially", preferredCandidate))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(preferredCandidate))

			ginkgo.By(fmt.Sprintf("Excluding %s from external LBs (removes it from speaker election)", preferredCandidate))
			k8s.AddLabelToNode(preferredCandidate, corev1.LabelNodeExcludeBalancers, "", cs)
			defer k8s.RemoveLabelFromNode(preferredCandidate, corev1.LabelNodeExcludeBalancers, cs)

			ginkgo.By(fmt.Sprintf("Validating failover back to the natural winner %s", natural))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(natural))

			ginkgo.By(fmt.Sprintf("Restoring %s to the eligible speaker set", preferredCandidate))
			k8s.RemoveLabelFromNode(preferredCandidate, corev1.LabelNodeExcludeBalancers, cs)

			ginkgo.By(fmt.Sprintf("Validating the preferred node %s reclaims the announcement", preferredCandidate))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(preferredCandidate))
		})

		ginkgo.It("should re-elect when the L2Advertisement spec changes at runtime", func() {
			if len(allNodes.Items) < 3 {
				ginkgo.Skip("need at least three nodes")
			}

			svc, _ := service.CreateWithBackend(cs, testNamespace, "runtime-spec-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("Applying a baseline L2Advertisement to observe the natural election winner")
			err := ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{
					{ObjectMeta: metav1.ObjectMeta{Name: "runtime-spec"}},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			natural := waitForNaturalWinner(svc)
			preferredA := nonNaturalNode(allNodes.Items, natural)
			Expect(preferredA).NotTo(BeEmpty())
			preferredB := nonNaturalNode(allNodes.Items, natural, preferredA)
			Expect(preferredB).NotTo(BeEmpty())

			const rtLabelKey = "l2-preferred-rt-test"

			ginkgo.By(fmt.Sprintf("Labeling %s=%s=a and %s=%s=b", preferredA, rtLabelKey, preferredB, rtLabelKey))
			k8s.AddLabelToNode(preferredA, rtLabelKey, "a", cs)
			defer k8s.RemoveLabelFromNode(preferredA, rtLabelKey, cs)
			k8s.AddLabelToNode(preferredB, rtLabelKey, "b", cs)
			defer k8s.RemoveLabelFromNode(preferredB, rtLabelKey, cs)

			ginkgo.By(fmt.Sprintf("Phase 1: applying L2Advertisement preferring %s", preferredA))
			l2adv := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "runtime-spec"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 100,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{rtLabelKey: "a"},
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2adv},
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(preferredA))

			ginkgo.By(fmt.Sprintf("Phase 2: updating L2Advertisement to prefer %s", preferredB))
			l2adv.Spec.PreferredNodeSelectors[0].Preference.MatchLabels[rtLabelKey] = "b"
			err = ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2adv},
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(preferredB))

			ginkgo.By("Phase 3: removing PreferredNodeSelectors from the L2Advertisement")
			l2adv.Spec.PreferredNodeSelectors = nil
			err = ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2adv},
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("Validating election returns to the baseline winner %s", natural))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(natural))
		})

		ginkgo.It("should respect nodeSelectors as a hard filter while preferredNodeSelectors orders the eligible set", func() {
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("need at least two nodes")
			}

			nodeA := allNodes.Items[0].Name
			nodeB := allNodes.Items[1].Name

			svc, _ := service.CreateWithBackend(cs, testNamespace, "hard-soft-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By(fmt.Sprintf("Labeling %s as eligible and preferred", nodeA))
			k8s.AddLabelToNode(nodeA, "l2-ns-pref-test", "eligible", cs)
			defer k8s.RemoveLabelFromNode(nodeA, "l2-ns-pref-test", cs)
			k8s.AddLabelToNode(nodeA, "l2-ns-preferred-test", "true", cs)
			defer k8s.RemoveLabelFromNode(nodeA, "l2-ns-preferred-test", cs)

			ginkgo.By(fmt.Sprintf("Labeling %s as eligible only", nodeB))
			k8s.AddLabelToNode(nodeB, "l2-ns-pref-test", "eligible", cs)
			defer k8s.RemoveLabelFromNode(nodeB, "l2-ns-pref-test", cs)

			ginkgo.By("Applying L2Advertisement with nodeSelectors (hard) and preferredNodeSelectors (soft)")
			l2adv := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "hard-soft-filter"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					NodeSelectors: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"l2-ns-pref-test": "eligible"}},
					},
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 100,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{"l2-ns-preferred-test": "true"},
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2adv},
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("Validating %s (eligible AND preferred) announces the service", nodeA))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(nodeA))
		})

		ginkgo.It("should announce from the nodeSelectors set when it is disjoint from preferredNodeSelectors", func() {
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("need at least two nodes")
			}

			nodeEligible := allNodes.Items[0].Name
			nodePreferred := allNodes.Items[1].Name

			svc, _ := service.CreateWithBackend(cs, testNamespace, "disjoint-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By(fmt.Sprintf("Labeling %s as eligible-only and %s as preferred-only", nodeEligible, nodePreferred))
			k8s.AddLabelToNode(nodeEligible, "l2-disjoint-eligible", "true", cs)
			defer k8s.RemoveLabelFromNode(nodeEligible, "l2-disjoint-eligible", cs)
			k8s.AddLabelToNode(nodePreferred, "l2-disjoint-preferred", "true", cs)
			defer k8s.RemoveLabelFromNode(nodePreferred, "l2-disjoint-preferred", cs)

			ginkgo.By("Applying disjoint nodeSelectors / preferredNodeSelectors")
			l2adv := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "disjoint"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					NodeSelectors: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"l2-disjoint-eligible": "true"}},
					},
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 100,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{"l2-disjoint-preferred": "true"},
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2adv},
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("Validating %s (eligible) announces — preferred-only %s is filtered out", nodeEligible, nodePreferred))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(nodeEligible))
			Consistently(announcingNode(svc), 5*time.Second, time.Second).ShouldNot(Equal(nodePreferred))
		})

		ginkgo.It("should sum weights across multiple L2Advertisements targeting the same pool", func() {
			if len(allNodes.Items) < 3 {
				ginkgo.Skip("need at least three nodes")
			}

			svc, _ := service.CreateWithBackend(cs, testNamespace, "multi-ad-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			nodeDouble := allNodes.Items[0].Name
			nodeSingle := allNodes.Items[1].Name

			ginkgo.By(fmt.Sprintf("Labeling %s with both l2-pref-zone and l2-pref-role", nodeDouble))
			k8s.AddLabelToNode(nodeDouble, "l2-pref-zone", "primary", cs)
			defer k8s.RemoveLabelFromNode(nodeDouble, "l2-pref-zone", cs)
			k8s.AddLabelToNode(nodeDouble, "l2-pref-role", "edge", cs)
			defer k8s.RemoveLabelFromNode(nodeDouble, "l2-pref-role", cs)

			ginkgo.By(fmt.Sprintf("Labeling %s with only l2-pref-zone", nodeSingle))
			k8s.AddLabelToNode(nodeSingle, "l2-pref-zone", "primary", cs)
			defer k8s.RemoveLabelFromNode(nodeSingle, "l2-pref-zone", cs)

			ginkgo.By("Applying two L2Advertisements (ad-zone weight 60, ad-role weight 50)")
			adZone := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "ad-zone"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 60,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{"l2-pref-zone": "primary"},
							},
						},
					},
				},
			}
			adRole := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "ad-role"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 50,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{"l2-pref-role": "edge"},
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{adZone, adRole},
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("Validating %s (score 110) announces the service", nodeDouble))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(nodeDouble))

			ginkgo.By(fmt.Sprintf("Removing l2-pref-zone from %s so it now scores only 50 (role)", nodeDouble))
			k8s.RemoveLabelFromNode(nodeDouble, "l2-pref-zone", cs)

			ginkgo.By(fmt.Sprintf("Validating %s (score 60) now beats %s (score 50)", nodeSingle, nodeDouble))
			Eventually(announcingNode(svc), 2*time.Minute, time.Second).Should(Equal(nodeSingle))
		})

		ginkgo.It("should scope preferredNodeSelectors to services matching serviceSelectors", func() {
			if len(allNodes.Items) < 3 {
				ginkgo.Skip("need at least three nodes")
			}

			svcMatched, _ := service.CreateWithBackend(cs, testNamespace, "svc-matched-lb", service.TrafficPolicyCluster,
				service.WithLabels(map[string]string{"app": "matched"}))
			defer func() {
				err := cs.CoreV1().Services(svcMatched.Namespace).Delete(context.TODO(), svcMatched.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			svcUnmatched, _ := service.CreateWithBackend(cs, testNamespace, "svc-unmatched-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svcUnmatched.Namespace).Delete(context.TODO(), svcUnmatched.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("Applying a baseline L2Advertisement to observe the natural winners for both services")
			err := ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{
					{ObjectMeta: metav1.ObjectMeta{Name: "svc-selector-baseline"}},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			naturalMatched := waitForNaturalWinner(svcMatched)
			naturalUnmatched := waitForNaturalWinner(svcUnmatched)

			preferredForMatched := nonNaturalNode(allNodes.Items, naturalMatched, naturalUnmatched)
			Expect(preferredForMatched).NotTo(BeEmpty())

			ginkgo.By(fmt.Sprintf("Labeling %s (non-natural winner for matched svc) as preferred", preferredForMatched))
			k8s.AddLabelToNode(preferredForMatched, "l2-svc-pref-test", "true", cs)
			defer k8s.RemoveLabelFromNode(preferredForMatched, "l2-svc-pref-test", cs)

			ginkgo.By("Adding a scoped L2Advertisement with preferences for matched services")
			scoped := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{Name: "svc-selector-scope"},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					ServiceSelectors: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"app": "matched"}},
					},
					PreferredNodeSelectors: []metallbv1beta1.PreferredNodeSelector{
						{
							Weight: 100,
							Preference: metav1.LabelSelector{
								MatchLabels: map[string]string{"l2-svc-pref-test": "true"},
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{
					{ObjectMeta: metav1.ObjectMeta{Name: "svc-selector-baseline"}},
					scoped,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("Validating matched svc lands on preferred %s", preferredForMatched))
			Eventually(announcingNode(svcMatched), 2*time.Minute, time.Second).Should(Equal(preferredForMatched))

			ginkgo.By(fmt.Sprintf("Validating unmatched svc stays at baseline winner %s", naturalUnmatched))
			Consistently(announcingNode(svcUnmatched), 5*time.Second, time.Second).Should(Equal(naturalUnmatched))
		})
	})
})

func waitForNaturalWinner(svc *corev1.Service) string {
	var winner string
	Eventually(func() error {
		s, err := status.L2ForService(ConfigUpdater.Client(), svc)
		if err != nil {
			return err
		}
		winner = s.Status.Node
		return nil
	}, 2*time.Minute, time.Second).ShouldNot(HaveOccurred())
	return winner
}

func nonNaturalNode(nodes []corev1.Node, exclude ...string) string {
	skip := make(map[string]struct{}, len(exclude))
	for _, e := range exclude {
		skip[e] = struct{}{}
	}
	for _, n := range nodes {
		if _, found := skip[n.Name]; found {
			continue
		}
		return n.Name
	}
	return ""
}

// announcingNode returns a Gomega poller that yields the node currently
// announcing svc, or the error string when the lookup fails so that the
// matcher's failure output is informative.
func announcingNode(svc *corev1.Service) func() string {
	return func() string {
		s, err := status.L2ForService(ConfigUpdater.Client(), svc)
		if err != nil {
			return err.Error()
		}
		return s.Status.Node
	}
}
