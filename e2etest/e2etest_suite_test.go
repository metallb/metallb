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

package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"go.universe.tf/e2etest/bgptests"
	"go.universe.tf/e2etest/l2tests"
	testsconfig "go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/iprange"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/service"
	"go.universe.tf/e2etest/webhookstests"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"
	e2econfig "k8s.io/kubernetes/test/e2e/framework/config"
)

var (
	skipDockerCmd       bool
	useOperator         bool
	reportPath          string
	updater             testsconfig.Updater
	updaterOtherNS      testsconfig.Updater
	prometheusNamespace string
	nodeNics            string
	localNics           string
	externalContainers  string
	runOnHost           bool
	bgpNativeMode       bool
	frrImage            string
	hostContainerMode   string
)

// handleFlags sets up all flags and parses the command line.
func handleFlags() {
	e2econfig.CopyFlags(e2econfig.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	/*
		Using framework.RegisterClusterFlags(flag.CommandLine) results in a panic:
		"flag redefined: kubeconfig".
		This happens because controller-runtime registers the kubeconfig flag as well.
		To solve this we set the framework's kubeconfig directly via the KUBECONFIG env var
		instead of letting it call the flag. Since we also use the provider flag it is handled manually.
	*/
	flag.StringVar(&framework.TestContext.Provider, "provider", "", "The name of the Kubernetes provider (gce, gke, local, skeleton (the fallback if not set), etc.)")
	framework.TestContext.KubeConfig = os.Getenv(clientcmd.RecommendedConfigPathEnvVar)

	flag.IntVar(&service.TestServicePort, "service-pod-port", 80, "port number that pod opens, default: 80")
	flag.BoolVar(&skipDockerCmd, "skip-docker", false, "set this to true if the BGP daemon is running on the host instead of in a container")
	flag.StringVar(&l2tests.IPV4ServiceRange, "ipv4-service-range", "0", "a range of IPv4 addresses for MetalLB to use when running in layer2 mode")
	flag.StringVar(&l2tests.IPV6ServiceRange, "ipv6-service-range", "0", "a range of IPv6 addresses for MetalLB to use when running in layer2 mode")
	flag.StringVar(&nodeNics, "node-nics", "", "node's interfaces list separated by comma and used when running in interface selector")
	flag.StringVar(&localNics, "local-nics", "", "local interfaces list separated by comma and used when running in interface selector")
	flag.BoolVar(&useOperator, "use-operator", false, "set this to true to run the tests using operator custom resources")
	flag.StringVar(&reportPath, "report-path", "/tmp/report", "the path to be used to dump test failure information")
	flag.StringVar(&prometheusNamespace, "prometheus-namespace", "monitoring", "the namespace prometheus is running in (if running)")
	flag.StringVar(&externalContainers, "external-containers", "", "a comma separated list of external containers names to use for the test. (valid parameters are: ibgp-single-hop / ibgp-multi-hop / ebgp-single-hop / ebgp-multi-hop)")
	flag.BoolVar(&bgpNativeMode, "bgp-native-mode", false, "says if we are testing against a deployment using bgp native mode")
	flag.StringVar(&frrImage, "frr-image", "quay.io/frrouting/frr:8.5.2", "the image to use for the external frr containers")
	flag.StringVar(&hostContainerMode, "host-bgp-mode", string(bgptests.IBGPMode), "tells whether to run the host container in ebgp or ibgp mode")

	flag.Parse()

	if _, res := os.LookupEnv("RUN_FRR_CONTAINER_ON_HOST_NETWORK"); res {
		runOnHost = true
	}
}

func TestMain(m *testing.M) {
	// Register test flags, then parse flags.
	handleFlags()
	if testing.Short() {
		return
	}

	framework.AfterReadingAllFlags(&framework.TestContext)

	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	if testing.Short() {
		return
	}

	gomega.RegisterFailHandler(framework.Fail)
	ginkgo.RunSpecs(t, "E2E Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	// Make sure the framework's kubeconfig is set.
	framework.ExpectNotEqual(framework.TestContext.KubeConfig, "", fmt.Sprintf("%s env var not set", clientcmd.RecommendedConfigPathEnvVar))

	// Validate the IPv4 service range.
	_, err := iprange.Parse(l2tests.IPV4ServiceRange)
	framework.ExpectNoError(err)

	// Validate the IPv6 service range.
	_, err = iprange.Parse(l2tests.IPV6ServiceRange)
	framework.ExpectNoError(err)

	cs, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	switch {
	case externalContainers != "":
		bgptests.FRRContainers, err = bgptests.ExternalContainersSetup(externalContainers, cs)
		framework.ExpectNoError(err)
	case runOnHost:
		hostBGPMode := bgptests.HostBGPMode(hostContainerMode)
		if hostBGPMode != bgptests.EBGPMode && hostBGPMode != bgptests.IBGPMode {
			panic("host bgpmode " + hostContainerMode + " not supported")
		}
		bgptests.FRRContainers, err = bgptests.HostContainerSetup(frrImage, hostBGPMode)
		framework.ExpectNoError(err)
	default:
		bgptests.FRRContainers, err = bgptests.KindnetContainersSetup(cs, frrImage)
		framework.ExpectNoError(err)
		if !bgpNativeMode {
			vrfFRRContainers, err := bgptests.VRFContainersSetup(cs, frrImage)
			framework.ExpectNoError(err)
			bgptests.FRRContainers = append(bgptests.FRRContainers, vrfFRRContainers...)
		}
	}

	clientconfig, err := framework.LoadConfig()
	framework.ExpectNoError(err)

	updater, err = testsconfig.UpdaterForCRs(clientconfig, metallb.Namespace)
	framework.ExpectNoError(err)

	// for testing namespace validation, we need an existing namespace that's different from the
	// metallb installation namespace
	otherNamespace := fmt.Sprintf("%s-other", metallb.Namespace)
	err = updater.Client().Create(context.Background(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: otherNamespace,
		},
	})
	// ignore failure if namespace already exists, fail for any other errors
	if err != nil && !errors.IsAlreadyExists(err) {
		framework.ExpectNoError(err)
	}
	updaterOtherNS, err = testsconfig.UpdaterForCRs(clientconfig, otherNamespace)
	framework.ExpectNoError(err)

	reporter := k8s.InitReporter(framework.TestContext.KubeConfig, reportPath, metallb.Namespace)

	bgptests.ConfigUpdater = updater
	l2tests.ConfigUpdater = updater
	webhookstests.ConfigUpdater = updater
	webhookstests.ConfigUpdaterOtherNS = updaterOtherNS
	bgptests.Reporter = reporter
	bgptests.ReportPath = reportPath
	l2tests.Reporter = reporter
	webhookstests.Reporter = reporter
	bgptests.PrometheusNamespace = prometheusNamespace
	l2tests.PrometheusNamespace = prometheusNamespace
	l2tests.NodeNics = strings.Split(nodeNics, ",")
	l2tests.LocalNics = strings.Split(localNics, ",")
})

var _ = ginkgo.AfterSuite(func() {
	cs, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	err = bgptests.InfraTearDown(cs)
	framework.ExpectNoError(err)
	if !bgpNativeMode {
		err = bgptests.InfraTearDownVRF(cs)
		framework.ExpectNoError(err)
	}
	err = updater.Clean()
	framework.ExpectNoError(err)

	// delete the namespace created for testing namespace validation
	nsSpec := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: updaterOtherNS.Namespace(),
		},
	}
	err = updaterOtherNS.Client().Delete(context.Background(), &nsSpec)
	// ignore failure if namespace does not exist, fail for any other errors
	if err != nil && !errors.IsNotFound(err) {
		framework.ExpectNoError(err)
	}
	err = updaterOtherNS.Clean()
	framework.ExpectNoError(err)

})
