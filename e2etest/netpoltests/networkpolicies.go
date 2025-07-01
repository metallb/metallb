// SPDX-License-Identifier:Apache-2.0
package netpoltests

import (
	"fmt"
	"net"
	"strconv"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
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
		probe, err = k8s.CreatePod(cs, tpod)
		Expect(err).NotTo(HaveOccurred())

		controller, err = metallb.ControllerPod(cs)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.It("only allowed traffic", func() {
		ginkgo.By("checking ingress - probe pod to any port on the controller other than webhook,metricshttps should be timeout")

		ingressPorts := map[string]string{
			"8888": "TIMEOUT", // Add random port to check for being blocked
		}
		for _, c := range controller.Spec.Containers {
			for _, p := range c.Ports {
				port := strconv.Itoa(int(p.ContainerPort))
				if p.Name == "monitoring" { // the non-https port is not whitelisted in the network policies
					ingressPorts[port] = "TIMEOUT"
					continue
				}
				ingressPorts[port] = "" // Agnhost connect return empty on success
			}
		}

		controllerIP := controller.Status.PodIP
		probeExec := executor.ForPod(probe.Namespace, probe.Name, "netpol")
		for port, action := range ingressPorts {
			out, err := probeExec.Exec("./agnhost", "connect", net.JoinHostPort(controllerIP, port), "--timeout", "5s")
			if action != "" {
				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring(action), fmt.Sprintf("connect to port %s returned %s not %s", port, out, action))
				continue
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring(""), fmt.Sprintf("connect to port %s returned %s", port, out))
		}

		ginkgo.By("checking egress - controller pod to any port on the probe other port than API should be timeout")
		egressPorts := map[string]string{
			"8888": "TIMEOUT", // Add random port to check for being blocked
			"6443": "REFUSED", // Until we restrict egress only to k8s service
		}

		probeIP := probe.Status.PodIP
		ctrlExec, err := executor.ForPodDebug(cs, controller.Namespace, controller.Name, "controller", agnhostImage)
		Expect(err).NotTo(HaveOccurred())

		for port, action := range egressPorts {
			out, err := ctrlExec.Exec("./agnhost", "connect", net.JoinHostPort(probeIP, port), "--timeout", "5s")
			if action != "" {
				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring(action), fmt.Sprintf("connect to port %s returned %s not %s", port, out, action))
				continue
			}
			Expect(err).NotTo(HaveOccurred())
		}
	})
})
