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
// https://github.com/kubernetes/kubernetes/blob/92aff21558831b829fbc8cbca3d52edc80c01aa3/test/e2e/network/loadbalancer.go#L878

package e2e

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	"go.universe.tf/metallb/e2etest/pkg/routes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

const frrContainer = "frr"

var _ = ginkgo.Describe("BGP", func() {
	f := framework.NewDefaultFramework("bgp")
	var loadBalancerCreateTimeout time.Duration

	var cs clientset.Interface

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
		loadBalancerCreateTimeout = e2eservice.GetServiceLoadBalancerCreationTimeout(cs)
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentGinkgoTestDescription().Failed {
			DescribeSvc(f.Namespace.Name)
		}
	})

	ginkgo.It("should work for type=Loadbalancer", func() {
		namespace := f.Namespace.Name
		serviceName := "external-local-lb"
		jig := e2eservice.NewTestJig(cs, namespace, serviceName)

		svc, err := jig.CreateLoadBalancerService(loadBalancerCreateTimeout,
			tweakServicePort())
		framework.ExpectNoError(err)

		_, err = jig.Run(tweakRCPort())
		framework.ExpectNoError(err)

		defer func() {
			err = cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
		}()

		port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
		ingressIP := e2eservice.GetIngressPoint(
			&svc.Status.LoadBalancer.Ingress[0])

		hostport := net.JoinHostPort(ingressIP, port)
		address := fmt.Sprintf("http://%s/", hostport)

		exc := executor.ForContainer(frrContainer)
		if skipDockerCmd {
			ginkgo.By(fmt.Sprintf("checking connectivity to %s", address))
			exc = executor.Host
		} else {
			ginkgo.By(fmt.Sprintf("checking connectivity to %s with docker", address))
		}

		err = wgetRetry(address, exc)
		framework.ExpectNoError(err)

		allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		framework.ExpectNoError(err)

		advertised := routes.ForIP(ingressIP, exc)
		err = routes.MatchNodes(allNodes.Items, advertised)
		framework.ExpectNoError(err)

		neighbors, err := frr.NeighborsInfo(exc)
		framework.ExpectNoError(err)

		err = frr.NeighborsMatchNodes(allNodes.Items, neighbors)
		framework.ExpectNoError(err)

		frrRoutes, _, err := frr.Routes(exc)
		framework.ExpectNoError(err)

		routes, ok := frrRoutes[ingressIP]
		framework.ExpectEqual(ok, true, ingressIP, "not found in frr routes")
		err = frr.RoutesMatchNodes(allNodes.Items, routes)
		framework.ExpectNoError(err)
	})
})
