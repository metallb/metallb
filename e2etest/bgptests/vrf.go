// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	frrconfig "go.universe.tf/metallb/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	testservice "go.universe.tf/metallb/e2etest/pkg/service"
	"go.universe.tf/metallb/internal/ipfamily"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = ginkgo.Describe("VRF", func() {
	var cs clientset.Interface
	var f *framework.Framework
	var containers []*frrcontainer.FRR

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentGinkgoTestDescription().Failed {
			dumpBGPInfo(ReportPath, ginkgo.CurrentGinkgoTestDescription().TestText, cs, f)
			k8s.DumpInfo(Reporter, ginkgo.CurrentGinkgoTestDescription().TestText)
		}
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)

		containers = append(VRFFRRContainers, FRRContainers...)
		for _, c := range containers {
			err := c.UpdateBGPConfigFile(frrconfig.Empty)
			framework.ExpectNoError(err)
		}
	})

	f = framework.NewDefaultFramework("bgp-vrf")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
	})

	table.DescribeTable("A service of protocol load balancer should work with ETP=cluster", func(pairingIPFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {
		_, svc := setupBGPService(f, pairingIPFamily, poolAddresses, containers, func(svc *corev1.Service) {
			testservice.TrafficPolicyCluster(svc)
			tweak(svc)
		})
		defer testservice.Delete(cs, svc)

		allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		framework.ExpectNoError(err)
		validateDesiredLB(svc)

		for _, c := range containers {
			validateService(cs, svc, allNodes.Items, c)
		}
	},
		table.FEntry("IPV4", ipfamily.IPv4, []string{v4PoolAddresses}, func(_ *corev1.Service) {}),
		table.Entry("IPV6", ipfamily.IPv6, []string{v6PoolAddresses}, func(_ *corev1.Service) {}),
		table.Entry("DUALSTACK", ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.DualStack(svc)
			}),
		table.Entry("IPV4 - request IPv4 via custom annotation", ipfamily.IPv4, []string{v4PoolAddresses},
			func(svc *corev1.Service) {
				testservice.WithSpecificIPs(svc, "192.168.10.100")
			}),
		table.Entry("DUALSTACK - request Dual Stack via custom annotation", ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.DualStack(svc)
				testservice.WithSpecificIPs(svc, "192.168.10.100", "fc00:f853:ccd:e799::")
			}),
	)
},
)
