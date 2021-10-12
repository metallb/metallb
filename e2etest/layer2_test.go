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

	"github.com/onsi/ginkgo"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/mac"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

var _ = ginkgo.Describe("L2", func() {
	f := framework.NewDefaultFramework("l2")

	var cs clientset.Interface

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
	})

	ginkgo.AfterEach(func() {
		// Clean previous configuration.
		err := updateConfigMap(cs, configFile{})
		framework.ExpectNoError(err)

		if ginkgo.CurrentGinkgoTestDescription().Failed {
			DescribeSvc(f.Namespace.Name)
		}
	})

	ginkgo.Context("type=Loadbalancer", func() {
		ginkgo.BeforeEach(func() {
			configData := configFile{
				Pools: []addressPool{
					{
						Name:     "l2-test",
						Protocol: Layer2,
						Addresses: []string{
							ipv4ServiceRange,
							ipv6ServiceRange,
						},
					},
				},
			}
			err := updateConfigMap(cs, configData)
			framework.ExpectNoError(err)
		})

		ginkgo.AfterEach(func() {

		})

		ginkgo.It("should work for ExternalTrafficPolicy=Cluster", func() {
			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeCluster)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			ginkgo.By("checking connectivity to its external VIP")

			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)
			err := wgetRetry(address, executor.Host)
			framework.ExpectNoError(err)
		})

		ginkgo.It("should work for ExternalTrafficPolicy=Local", func() {
			svc, jig := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeLocal)
			err := jig.Scale(5)
			framework.ExpectNoError(err)

			epNodes, err := jig.ListNodesWithEndpoint() // Only nodes with an endpoint could be advertising the IP
			framework.ExpectNoError(err)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			ginkgo.By("checking connectivity to its external VIP")

			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)
			err = wgetRetry(address, executor.Host)
			framework.ExpectNoError(err)

			err = mac.UpdateNodeCache(epNodes, executor.Host)
			framework.ExpectNoError(err)

			macAddr, err := mac.ForIP(ingressIP, executor.Host)
			framework.ExpectNoError(err)

			advNode, err := mac.MatchNode(epNodes, macAddr, executor.Host)
			framework.ExpectNoError(err)

			for i := 0; i < 5; i++ {
				name, err := getEndpointHostName(hostport, executor.Host)
				framework.ExpectNoError(err)

				pod, err := cs.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), name, metav1.GetOptions{})
				framework.ExpectNoError(err)
				framework.ExpectEqual(pod.Spec.NodeName == advNode.Name, true, "traffic arrived to a pod not from the announcing node")
			}

		})

	})
})

// Relies on the endpoint being an agnhost netexec pod.
func getEndpointHostName(ep string, exec executor.Executor) (string, error) {
	res, err := exec.Exec("wget", "-O-", "-q", fmt.Sprintf("http://%s/hostname", ep))
	if err != nil {
		return "", err
	}

	return res, nil
}
