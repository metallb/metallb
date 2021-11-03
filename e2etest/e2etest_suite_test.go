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
// https://github.com/ovn-org/ovn-kubernetes/blob/a99cc892576be4f15caceca62a87557572e0a447/test/e2e/e2e_suite_test.go

package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	internalconfig "go.universe.tf/metallb/internal/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/kubernetes/test/e2e/framework"
	e2econfig "k8s.io/kubernetes/test/e2e/framework/config"
)

const (
	defaultTestNameSpace     = "metallb-system"
	defaultContainersNetwork = "kind"
)

var (
	// Use ephemeral port for pod, instead of well-known port (tcp/80).
	servicePodPort    uint
	skipDockerCmd     bool
	ipv4ServiceRange  string
	ipv6ServiceRange  string
	testNameSpace     = defaultTestNameSpace
	containersNetwork = defaultContainersNetwork
	hostIPv4          string
	hostIPv6          string
)

// handleFlags sets up all flags and parses the command line.
func handleFlags() {
	e2econfig.CopyFlags(e2econfig.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.UintVar(&servicePodPort, "service-pod-port", 80, "port number that pod opens, default: 80")
	flag.BoolVar(&skipDockerCmd, "skip-docker", false, "set this to true if the BGP daemon is running on the host instead of in a container")
	flag.StringVar(&ipv4ServiceRange, "ipv4-service-range", "0", "a range of IPv4 addresses for MetalLB to use when running in layer2 mode")
	flag.StringVar(&ipv6ServiceRange, "ipv6-service-range", "0", "a range of IPv6 addresses for MetalLB to use when running in layer2 mode")
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
	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		testNameSpace = ns
	}

	if _, res := os.LookupEnv("RUN_FRR_CONTAINER_ON_HOST_NETWORK"); res == true {
		containersNetwork = "host"
	}

	if ip := os.Getenv("PROVISIONING_HOST_EXTERNAL_IPV4"); len(ip) != 0 {
		hostIPv4 = ip
	}
	if ip := os.Getenv("PROVISIONING_HOST_EXTERNAL_IPV6"); len(ip) != 0 {
		hostIPv6 = ip
	}

	// Validate the IPv4 service range.
	_, err := internalconfig.ParseCIDR(ipv4ServiceRange)
	framework.ExpectNoError(err)

	// Validate the IPv6 service range.
	_, err = internalconfig.ParseCIDR(ipv6ServiceRange)
	framework.ExpectNoError(err)

	cs, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	_, err = cs.CoreV1().ConfigMaps(testNameSpace).Get(context.TODO(), "config", metav1.GetOptions{})
	framework.ExpectEqual(errors.IsNotFound(err), true)

	// Init empty MetalLB ConfigMap.
	_, err = cs.CoreV1().ConfigMaps(testNameSpace).Create(context.TODO(), &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: testNameSpace,
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)
})

var _ = ginkgo.AfterSuite(func() {
	cs, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	err = cs.CoreV1().ConfigMaps(testNameSpace).Delete(context.TODO(), "config", metav1.DeleteOptions{})
	framework.ExpectNoError(err)
})
