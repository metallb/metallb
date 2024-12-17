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
	. "github.com/onsi/gomega"

	"go.universe.tf/e2etest/bgptests"
	"go.universe.tf/e2etest/l2tests"
	testsconfig "go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/executor"
	frrprovider "go.universe.tf/e2etest/pkg/frr/provider"
	"go.universe.tf/e2etest/pkg/iprange"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/service"
	"go.universe.tf/e2etest/webhookstests"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	useOperator         bool
	reportPath          string
	updater             testsconfig.Updater
	updaterOtherNS      testsconfig.Updater
	prometheusNamespace string
	nodeNics            string
	localNics           string
	externalContainers  string
	runOnHost           bool
	bgpMode             string
	frrImage            string
	hostContainerMode   string
	withVRF             bool
	kubectlPath         string
	frrK8sNamespace     string
)

// handleFlags sets up all flags and parses the command line.
func handleFlags() {
	flag.IntVar(&service.TestServicePort, "service-pod-port", 80, "port number that pod opens, default: 80")
	flag.StringVar(&l2tests.IPV4ServiceRange, "ipv4-service-range", "0", "a range of IPv4 addresses for MetalLB to use when running in layer2 mode")
	flag.StringVar(&l2tests.IPV6ServiceRange, "ipv6-service-range", "0", "a range of IPv6 addresses for MetalLB to use when running in layer2 mode")
	flag.StringVar(&nodeNics, "node-nics", "", "node's interfaces list separated by comma and used when running in interface selector")
	flag.StringVar(&localNics, "local-nics", "", "local interfaces list separated by comma and used when running in interface selector")
	flag.BoolVar(&useOperator, "use-operator", false, "set this to true to run the tests using operator custom resources")
	flag.StringVar(&reportPath, "report-path", "/tmp/report", "the path to be used to dump test failure information")
	flag.StringVar(&prometheusNamespace, "prometheus-namespace", "monitoring", "the namespace prometheus is running in (if running)")
	flag.StringVar(&externalContainers, "external-containers", "", "a comma separated list of external containers names to use for the test. (valid parameters are: ibgp-single-hop / ibgp-multi-hop / ebgp-single-hop / ebgp-multi-hop)")
	flag.StringVar(&frrImage, "frr-image", "quay.io/frrouting/frr:9.1.0", "the image to use for the external frr containers")
	flag.StringVar(&hostContainerMode, "host-bgp-mode", string(bgptests.IBGPMode), "tells whether to run the host container in ebgp or ibgp mode")
	flag.BoolVar(&withVRF, "with-vrf", false, "runs the tests against containers reacheable via linux vrfs. More coverage, but might not work depending on the OS")
	flag.StringVar(&bgpMode, "bgp-mode", "", "says which bgp mode we are testing against. valid options are: native, frr, frr-k8s, frr-k8s-external")
	flag.StringVar(&frrK8sNamespace, "frr-k8s-namespace", metallb.Namespace, "the namespace frr-k8s is running in, defaults to metallb's")
	flag.StringVar(&executor.Kubectl, "kubectl", "kubectl", "the path for the kubectl binary")

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

	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	if testing.Short() {
		return
	}
	RegisterFailHandler(ginkgo.Fail)

	ginkgo.RunSpecs(t, "E2E Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	log.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.UseDevMode(true)))

	// Validate the IPv4 service range.
	_, err := iprange.Parse(l2tests.IPV4ServiceRange)
	Expect(err).NotTo(HaveOccurred())

	// Validate the IPv6 service range.
	_, err = iprange.Parse(l2tests.IPV6ServiceRange)
	Expect(err).NotTo(HaveOccurred())

	cs := k8sclient.New()

	switch {
	case externalContainers != "":
		bgptests.FRRContainers, err = bgptests.ExternalContainersSetup(externalContainers, cs)
		Expect(err).NotTo(HaveOccurred())
	case runOnHost:
		hostBGPMode := bgptests.HostBGPMode(hostContainerMode)
		if hostBGPMode != bgptests.EBGPMode && hostBGPMode != bgptests.IBGPMode {
			panic("host bgpmode " + hostContainerMode + " not supported")
		}
		bgptests.FRRContainers, err = bgptests.HostContainerSetup(frrImage, hostBGPMode)
		Expect(err).NotTo(HaveOccurred())
	default:
		bgptests.FRRContainers, err = bgptests.KindnetContainersSetup(cs, frrImage)
		Expect(err).NotTo(HaveOccurred())
		if withVRF {
			vrfFRRContainers, err := bgptests.VRFContainersSetup(cs, frrImage)
			Expect(err).NotTo(HaveOccurred())
			bgptests.FRRContainers = append(bgptests.FRRContainers, vrfFRRContainers...)
		}
	}

	clientconfig := k8sclient.RestConfig()

	updater, err = testsconfig.UpdaterForCRs(clientconfig, metallb.Namespace)
	Expect(err).NotTo(HaveOccurred())

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
		Expect(err).NotTo(HaveOccurred())
	}
	updaterOtherNS, err = testsconfig.UpdaterForCRs(clientconfig, otherNamespace)
	Expect(err).NotTo(HaveOccurred())

	switch bgpMode {
	case "native":
		bgptests.FRRProvider = nil
	case "frr":
		bgptests.FRRProvider, err = frrprovider.NewFRRMode(clientconfig)
		Expect(err).NotTo(HaveOccurred())
	case "frr-k8s":
		fallthrough
	case "frr-k8s-external":
		bgptests.FRRProvider, err = frrprovider.NewFRRK8SMode(clientconfig, frrK8sNamespace)
		Expect(err).NotTo(HaveOccurred())
	default:
		ginkgo.Fail(fmt.Sprintf("unsupported --bgp-mode %s - supported options are: native, frr, frr-k8s, frr-k8s-external", bgpMode))
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		ginkgo.Fail("KUBECONFIG not set")
	}

	reporter := k8s.InitReporter(kubeconfig, reportPath, metallb.Namespace, frrK8sNamespace)

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
	cs := k8sclient.New()

	err := bgptests.InfraTearDown(cs)
	Expect(err).NotTo(HaveOccurred())
	if withVRF {
		err = bgptests.InfraTearDownVRF(cs)
		Expect(err).NotTo(HaveOccurred())
	}
	err = updater.Clean()
	Expect(err).NotTo(HaveOccurred())

	// delete the namespace created for testing namespace validation
	nsSpec := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: updaterOtherNS.Namespace(),
		},
	}
	err = updaterOtherNS.Client().Delete(context.Background(), &nsSpec)
	// ignore failure if namespace does not exist, fail for any other errors
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred())
	}
	err = updaterOtherNS.Clean()
	Expect(err).NotTo(HaveOccurred())

})
