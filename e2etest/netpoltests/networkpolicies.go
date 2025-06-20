// SPDX-License-Identifier:Apache-2.0
package netpoltests

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

const agnhostImage = "registry.k8s.io/e2e-test-images/agnhost:2.47"

var _ = ginkgo.Describe("Networkpolicies", func() {
	var (
		cs         clientset.Interface
		probe      *corev1.Pod
		controller *corev1.Pod
	)

	ginkgo.BeforeEach(func() {
		cs = k8sclient.New()

		got, err := cs.NetworkingV1().NetworkPolicies(metallb.Namespace).List(context.TODO(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		want, err := expectedPolicies()
		Expect(err).NotTo(HaveOccurred())

		Expect(len(got.Items)).To(Equal(len(want)),
			"Want %d network policies, got %d", len(want), len(got.Items))

		for _, wantPolicy := range want {
			found := false
			for _, gotPolicy := range got.Items {
				if gotPolicy.Name == wantPolicy.Name && gotPolicy.Namespace == wantPolicy.Namespace {
					diff := cmp.Diff(wantPolicy.Spec, gotPolicy.Spec)
					Expect(diff).To(BeEmpty(), "Policy spec mismatch for %s/%s:\n%s",
						wantPolicy.Namespace, wantPolicy.Name, diff)

					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Policy %s/%s not found", wantPolicy.Namespace, wantPolicy.Name)
		}

		probeNamespace, err := k8s.CreateTestNamespace(cs, "test-netpol")
		Expect(err).NotTo(HaveOccurred())
		ginkgo.DeferCleanup(func() {
			err = k8s.DeleteNamespace(cs, probeNamespace)
			Expect(err).NotTo(HaveOccurred())
		})
		tpod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "netpol-test",
				Namespace: probeNamespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "netpol",
					Image: agnhostImage,
				}},
			},
		}
		probe, err = metallb.CreatePod(cs, tpod)
		Expect(err).NotTo(HaveOccurred())

		controller, err = metallb.ControllerPod(cs)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.It("only allowed traffic", func() {
		ginkgo.By("checking ingress - probe pod to any port on the controller other than webhook should be timeout")
		controllerIP := controller.Status.PodIP
		probeExec := executor.ForPod(probe.Namespace, probe.Name, "netpol")
		out, err := probeExec.Exec("./agnhost", "connect", net.JoinHostPort(controllerIP, "7472"), "--timeout", "5s")
		Expect(err).To(HaveOccurred())
		Expect(out).To(ContainSubstring("TIMEOUT"), out)
		out, err = probeExec.Exec("./agnhost", "connect", net.JoinHostPort(controllerIP, "9443"), "--timeout", "5s")
		Expect(err).NotTo(HaveOccurred(), out) // Until we manage to allow traffic only from k8s API

		ginkgo.By("checking egress - controller pod to any port on the probe other port than API should be timeout")
		probeIP := probe.Status.PodIP
		ctrlExec, err := executor.ForPodDebug(cs, controller.Namespace, controller.Name, "controller", agnhostImage)
		Expect(err).NotTo(HaveOccurred())
		out, err = ctrlExec.Exec("./agnhost", "connect", net.JoinHostPort(probeIP, "8080"), "--timeout", "5s")
		Expect(err).To(HaveOccurred())
		Expect(out).To(ContainSubstring("TIMEOUT")) // If no netpol then REFUSED
		out, err = ctrlExec.Exec("./agnhost", "connect", net.JoinHostPort(probeIP, "6443"), "--timeout", "5s")
		Expect(err).To(HaveOccurred())
		Expect(out).To(ContainSubstring("REFUSED")) // until we restrict only to k8s svc
	})
})

func expectedPolicies() ([]networkingv1.NetworkPolicy, error) {
	labels := "{app: metallb, component: controller}"
	if _, ok := os.LookupEnv("CONTROLLER_SELECTOR"); ok {
		labels = "{app.kubernetes.io/component: controller, app.kubernetes.io/instance: metallb, app.kubernetes.io/name: metallb}"
	}
	controllerPolicy := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: controller
  namespace: metallb-system
spec:
  podSelector:
    matchLabels: %s
  egress:
    - ports:
      - protocol: TCP
        port: 6443
  ingress:
    - ports:
      - protocol: TCP
        port: metricshttps
      - protocol: TCP
        port: webhook-server
  policyTypes:
    - Egress
    - Ingress`, labels)

	denyAllPolicy := `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: metallb-system
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress`

	var ret []networkingv1.NetworkPolicy

	for _, p := range []string{controllerPolicy, denyAllPolicy} {
		var policy networkingv1.NetworkPolicy
		if err := yaml.Unmarshal([]byte(p), &policy); err != nil {
			return nil, err
		}
		ret = append(ret, policy)
	}

	return ret, nil
}
