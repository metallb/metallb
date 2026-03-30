// SPDX-License-Identifier:Apache-2.0

package servicetests

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/k8sreporter"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/l2tests"
	"go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
)

// Same image as jigservice backends; upstream Dockerfile installs curl and defaults to `agnhost pause`.
const clientAgnhostImage = "registry.k8s.io/e2e-test-images/agnhost:2.45"

var (
	ConfigUpdater config.Updater
	Reporter      *k8sreporter.KubernetesReporter
)

// lbTestResources builds MetalLB CRs for the table entries (IPAddressPool only).
func lbTestResources(addresses []string) config.Resources {
	return config.Resources{
		Pools: []metallbv1beta1.IPAddressPool{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "lb-int-pool",
				},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: addresses,
				},
			},
		},
	}
}

func svcV4Local(svc *corev1.Service) {
	service.ForceV4(svc)
	service.TrafficPolicyLocal(svc)
}

func svcV6Local(svc *corev1.Service) {
	service.ForceV6(svc)
	service.TrafficPolicyLocal(svc)
}

func svcDualStackLocal(svc *corev1.Service) {
	service.DualStack(svc)
	service.TrafficPolicyLocal(svc)
}

func loadBalancerIngressReady(svc *corev1.Service, fam ipfamily.Family) bool {
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return false
	}
	var has4, has6 bool
	for i := range svc.Status.LoadBalancer.Ingress {
		ip := net.ParseIP(jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[i]))
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			has4 = true
		} else {
			has6 = true
		}
	}
	switch fam {
	case ipfamily.IPv4:
		return has4
	case ipfamily.IPv6:
		return has6
	case ipfamily.DualStack:
		return has4 && has6
	default:
		return false
	}
}

func ingressHostForFamily(svc *corev1.Service, fam ipfamily.Family) string {
	for i := range svc.Status.LoadBalancer.Ingress {
		h := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[i])
		ip := net.ParseIP(h)
		if ip == nil {
			continue
		}
		switch fam {
		case ipfamily.IPv4:
			if ip.To4() != nil {
				return h
			}
		case ipfamily.IPv6:
			if ip.To4() == nil {
				return h
			}
		}
	}
	return ""
}

func podIPMatchingFamily(pod *corev1.Pod, fam ipfamily.Family) string {
	for _, p := range pod.Status.PodIPs {
		ip := net.ParseIP(p.IP)
		if ip == nil {
			continue
		}
		if fam == ipfamily.IPv4 && ip.To4() != nil {
			return p.IP
		}
		if fam == ipfamily.IPv6 && ip.To4() == nil {
			return p.IP
		}
	}
	if pod.Status.PodIP != "" {
		ip := net.ParseIP(pod.Status.PodIP)
		if ip != nil {
			if fam == ipfamily.IPv4 && ip.To4() != nil {
				return pod.Status.PodIP
			}
			if fam == ipfamily.IPv6 && ip.To4() == nil {
				return pod.Status.PodIP
			}
		}
	}
	return ""
}

var _ = ginkgo.Describe("LoadBalancer", func() {
	var cs clientset.Interface
	testNamespace := ""

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
		var err error
		cs = k8sclient.New()
		testNamespace, err = k8s.CreateTestNamespace(cs, "svcint")
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Clearing any previous configuration")
		err = ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.DescribeTable("external load balancer IP reachable from pods with correct client source",
		func(diffNode bool, metallbRes config.Resources, svcTweak func(*corev1.Service)) {
			ctx := context.Background()

			ginkgo.By("Applying MetalLB resources from the table entry")
			err := ConfigUpdater.Update(metallbRes)
			Expect(err).NotTo(HaveOccurred())

			nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			if diffNode && len(nodes.Items) < 2 {
				ginkgo.Skip("Need at least two nodes for different-node case")
			}

			backendNode := nodes.Items[0].Name
			clientNode := backendNode
			if diffNode {
				clientNode = nodes.Items[1].Name
			}

			jig := jigservice.NewTestJig(cs, testNamespace, "metallb-lb")
			svc, err := jig.CreateLoadBalancerService(ctx, svcTweak)
			Expect(err).NotTo(HaveOccurred())

			svcFam, err := ipfamily.ForService(svc)
			Expect(err).NotTo(HaveOccurred())

			_, err = jig.Run(ctx, func(rc *corev1.ReplicationController) {
				rc.Spec.Template.Spec.Containers[0].Args = []string{
					"netexec",
					fmt.Sprintf("--http-port=%d", service.TestServicePort),
					fmt.Sprintf("--udp-port=%d", service.TestServicePort),
				}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(service.TestServicePort)
				rc.Spec.Template.Spec.NodeName = backendNode
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				svc, err = cs.CoreV1().Services(testNamespace).Get(ctx, svc.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return loadBalancerIngressReady(svc, svcFam)
			}, 2*time.Minute, time.Second).Should(BeTrue(), "LoadBalancer ingress was not assigned for the service IP family")

			selector := labels.Set(jig.Labels).AsSelector()
			backendPods, err := cs.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
			Expect(err).NotTo(HaveOccurred())
			Expect(backendPods.Items).NotTo(BeEmpty())
			backendPod := backendPods.Items[0]
			Expect(backendPod.Status.PodIP).NotTo(BeEmpty())

			client := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clientpod",
					Namespace: testNamespace,
				},
				Spec: corev1.PodSpec{
					NodeName: clientNode,
					Containers: []corev1.Container{
						{
							Name:  "agnhost",
							Image: clientAgnhostImage,
							// Image ENTRYPOINT/CMD run `agnhost pause` — keep pod alive for kubectl exec + curl.
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			}
			clientPod, err := k8s.CreatePod(cs, client)
			Expect(err).NotTo(HaveOccurred())
			Expect(clientPod.Status.PodIP).NotTo(BeEmpty())

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))

			assertReachability := func(lbFam ipfamily.Family) {
				lbHost := ingressHostForFamily(svc, lbFam)
				Expect(lbHost).NotTo(BeEmpty(), "ingress IP for family %s", lbFam)
				wantClientIP := podIPMatchingFamily(clientPod, lbFam)
				if wantClientIP == "" {
					ginkgo.Skip(fmt.Sprintf("client pod has no %s address", lbFam))
				}

				baseURL := fmt.Sprintf("http://%s/", net.JoinHostPort(lbHost, port))
				clientExec := executor.ForPod(testNamespace, clientPod.Name, "agnhost")

				ginkgo.By(fmt.Sprintf("Checking reachability via %s load balancer IP", lbFam))
				hostOut, err := clientExec.Exec("curl", "-fsS", "--max-time", "10", baseURL+"hostName")
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(hostOut)).To(Equal(backendPod.Name),
					"netexec /hostName should match the backend pod name (default pod hostname in Kubernetes)")

				ginkgo.By(fmt.Sprintf("Checking client source IP seen by backend (%s)", lbFam))
				clientOut, err := clientExec.Exec("curl", "-fsS", "--max-time", "10", baseURL+"clientip")
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(clientOut)).To(ContainSubstring(wantClientIP))
			}

			switch svcFam {
			case ipfamily.IPv4:
				assertReachability(ipfamily.IPv4)
			case ipfamily.IPv6:
				assertReachability(ipfamily.IPv6)
			case ipfamily.DualStack:
				assertReachability(ipfamily.IPv4)
				assertReachability(ipfamily.IPv6)
			default:
				ginkgo.Fail(fmt.Sprintf("unsupported service IP family %q", svcFam))
			}
		},
		ginkgo.Entry("IPV4, client on same node as backend", false, lbTestResources([]string{l2tests.IPV4ServiceRange}), svcV4Local),
		ginkgo.Entry("IPV4, client on different node than backend", true, lbTestResources([]string{l2tests.IPV4ServiceRange}), svcV4Local),
		ginkgo.Entry("IPV6, client on same node as backend", false, lbTestResources([]string{l2tests.IPV6ServiceRange}), svcV6Local),
		ginkgo.Entry("IPV6, client on different node than backend", true, lbTestResources([]string{l2tests.IPV6ServiceRange}), svcV6Local),
		ginkgo.Entry("DUALSTACK, client on same node as backend", false, lbTestResources([]string{l2tests.IPV4ServiceRange, l2tests.IPV6ServiceRange}), svcDualStackLocal),
		ginkgo.Entry("DUALSTACK, client on different node than backend", true, lbTestResources([]string{l2tests.IPV4ServiceRange, l2tests.IPV6ServiceRange}), svcDualStackLocal),
	)
})
