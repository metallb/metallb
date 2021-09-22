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

	"go.universe.tf/metallb/e2etest/pkg/metrics"

	"github.com/onsi/ginkgo"
	dto "github.com/prometheus/client_model/go"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	"go.universe.tf/metallb/e2etest/pkg/routes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

const frrContainer = "frr"

var _ = ginkgo.Describe("BGP", func() {
	f := framework.NewDefaultFramework("bgp")
	var cs clientset.Interface

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentGinkgoTestDescription().Failed {
			DescribeSvc(f.Namespace.Name)
		}
	})

	ginkgo.It("should work for type=Loadbalancer", func() {
		svc := createServiceWithBackend(cs, f.Namespace.Name)

		defer func() {
			err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
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

		err := wgetRetry(address, exc)
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

	ginkgo.Context("metrics", func() {
		var controllerPod *corev1.Pod
		var speakerPods []*corev1.Pod

		ginkgo.BeforeEach(func() {
			pods, err := cs.CoreV1().Pods("metallb-system").List(context.Background(), metav1.ListOptions{
				LabelSelector: "component=controller",
			})
			framework.ExpectNoError(err)
			framework.ExpectEqual(len(pods.Items), 1, "More than one controller found")
			controllerPod = &pods.Items[0]

			speakers, err := cs.CoreV1().Pods("metallb-system").List(context.Background(), metav1.ListOptions{
				LabelSelector: "component=speaker",
			})
			framework.ExpectNoError(err)
			speakerPods = make([]*corev1.Pod, 0)
			for _, item := range speakers.Items {
				speakerPods = append(speakerPods, &item)
			}
		})

		ginkgo.It("should be exposed by the controller", func() {
			peerAddr := "172.18.0.5:179" // TODO replace this when we create the config locally
			poolName := "dev-env-bgp"    // TODO replace this when we create the config locally
			ginkgo.By("checking the metrics when no service is added")
			controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod)
			framework.ExpectNoError(err)
			validateGaugeValue(0, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
			validateGaugeValue(272, "metallb_allocator_addresses_total", map[string]string{"pool": poolName}, controllerMetrics)

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				speakerMetrics, err := metrics.ForPod(controllerPod, speaker)
				framework.ExpectNoError(err)
				validateGaugeValue(1, "metallb_bgp_session_up", map[string]string{"peer": peerAddr}, speakerMetrics)
				validateGaugeValue(0, "metallb_bgp_announced_prefixes_total", map[string]string{"peer": peerAddr}, speakerMetrics)
			}

			ginkgo.By("creating a service")
			svc := createServiceWithBackend(cs, f.Namespace.Name) // Is a sleep required here?
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ginkgo.By("checking the metrics when a service is added")
			controllerMetrics, err = metrics.ForPod(controllerPod, controllerPod)
			framework.ExpectNoError(err)
			validateGaugeValue(1, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				speakerMetrics, err := metrics.ForPod(controllerPod, speaker)
				framework.ExpectNoError(err)
				validateGaugeValue(1, "metallb_bgp_session_up", map[string]string{"peer": peerAddr}, speakerMetrics)
				validateGaugeValue(1, "metallb_bgp_announced_prefixes_total", map[string]string{"peer": peerAddr}, speakerMetrics)

				updatesTotal, err := metrics.CounterForLabels("metallb_bgp_updates_total", map[string]string{"peer": peerAddr}, speakerMetrics)
				framework.ExpectNoError(err)
				framework.ExpectEqual(updatesTotal >= 1, true, "expecting ", updatesTotal, "greater than 1")

				validateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": speaker.Spec.NodeName, "protocol": "bgp", "service": fmt.Sprintf("%s/%s", f.Namespace.Name, svc.Name)}, speakerMetrics)
			}
		})
	})

})

func createServiceWithBackend(cs clientset.Interface, namespace string) *corev1.Service {
	serviceName := "external-local-lb"
	jig := e2eservice.NewTestJig(cs, namespace, serviceName)
	timeout := e2eservice.GetServiceLoadBalancerCreationTimeout(cs)

	svc, err := jig.CreateLoadBalancerService(timeout, tweakServicePort())
	framework.ExpectNoError(err)
	_, err = jig.Run(tweakRCPort())
	framework.ExpectNoError(err)
	return svc
}

func validateGaugeValue(expectedValue int, metricName string, labels map[string]string, m map[string]*dto.MetricFamily) {
	value, err := metrics.GaugeForLabels(metricName, labels, m)
	framework.ExpectNoError(err)
	framework.ExpectEqual(value, expectedValue, "invalid value for ", metricName, labels)
}
