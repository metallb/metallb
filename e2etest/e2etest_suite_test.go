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
	"path"
	"strings"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	"go.universe.tf/metallb/e2etest/bgptests"
	"go.universe.tf/metallb/e2etest/l2tests"
	testsconfig "go.universe.tf/metallb/e2etest/pkg/config"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	"go.universe.tf/metallb/e2etest/pkg/service"
	"go.universe.tf/metallb/e2etest/webhookstests"
	internalconfig "go.universe.tf/metallb/internal/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"k8s.io/kubernetes/test/e2e/framework"
	e2econfig "k8s.io/kubernetes/test/e2e/framework/config"
)

var (
	skipDockerCmd       bool
	ipv4ForContainers   string
	ipv6ForContainers   string
	useOperator         bool
	reportPath          string
	updater             testsconfig.Updater
	updaterOtherNS      testsconfig.Updater
	prometheusNamespace string
	nodeNics            string
	localNics           string
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
	flag.StringVar(&ipv4ForContainers, "ips-for-containers-v4", "0", "a comma separated list of IPv4 addresses available for containers")
	flag.StringVar(&ipv6ForContainers, "ips-for-containers-v6", "0", "a comma separated list of IPv6 addresses available for containers")
	flag.StringVar(&l2tests.IPV4ServiceRange, "ipv4-service-range", "0", "a range of IPv4 addresses for MetalLB to use when running in layer2 mode")
	flag.StringVar(&l2tests.IPV6ServiceRange, "ipv6-service-range", "0", "a range of IPv6 addresses for MetalLB to use when running in layer2 mode")
	flag.StringVar(&nodeNics, "node-nics", "", "node's interfaces list separated by comma and used when running in interface selector")
	flag.StringVar(&localNics, "local-nics", "", "local interfaces list separated by comma and used when running in interface selector")
	flag.BoolVar(&useOperator, "use-operator", false, "set this to true to run the tests using operator custom resources")
	flag.StringVar(&reportPath, "report-path", "/tmp/report", "the path to be used to dump test failure information")
	flag.StringVar(&prometheusNamespace, "prometheus-namespace", "monitoring", "the namespace prometheus is running in (if running)")
	flag.Parse()
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
	// Run tests through the Ginkgo runner with output to console + JUnit for reporting
	var r []ginkgo.Reporter
	if framework.TestContext.ReportDir != "" {
		klog.Infof("Saving reports to %s", framework.TestContext.ReportDir)
		if err := os.MkdirAll(framework.TestContext.ReportDir, 0755); err != nil {
			klog.Errorf("Failed creating report directory: %v", err)
		} else {
			r = append(r, reporters.NewJUnitReporter(path.Join(framework.TestContext.ReportDir, fmt.Sprintf("junit_%v%02d.xml", framework.TestContext.ReportPrefix, config.GinkgoConfig.ParallelNode))))
		}
	}
	gomega.RegisterFailHandler(framework.Fail)
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "E2E Suite", r)
}

var _ = ginkgo.BeforeSuite(func() {
	// Make sure the framework's kubeconfig is set.
	framework.ExpectNotEqual(framework.TestContext.KubeConfig, "", fmt.Sprintf("%s env var not set", clientcmd.RecommendedConfigPathEnvVar))

	// Validate the IPv4 service range.
	_, err := internalconfig.ParseCIDR(l2tests.IPV4ServiceRange)
	framework.ExpectNoError(err)

	// Validate the IPv6 service range.
	_, err = internalconfig.ParseCIDR(l2tests.IPV6ServiceRange)
	framework.ExpectNoError(err)

	cs, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	framework.ExpectNoError(err)
	v4Addresses := strings.Split(ipv4ForContainers, ",")
	v6Addresses := strings.Split(ipv6ForContainers, ",")
	bgptests.FRRContainers, err = bgptests.InfraSetup(v4Addresses, v6Addresses, cs)
	framework.ExpectNoError(err)

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

	err = bgptests.InfraTearDown(bgptests.FRRContainers, cs)
	framework.ExpectNoError(err)
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
