// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.universe.tf/metallb/e2etest/pkg/config"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	"go.universe.tf/metallb/e2etest/pkg/routes"
	"go.universe.tf/metallb/e2etest/pkg/wget"
	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

func validateFRRPeeredWithNodes(cs clientset.Interface, c *frrcontainer.FRR, ipFamily string) {
	allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	framework.ExpectNoError(err)

	ginkgo.By(fmt.Sprintf("checking all nodes are peered with the frr instance %s", c.Name))
	Eventually(func() error {
		neighbors, err := frr.NeighborsInfo(c)
		framework.ExpectNoError(err)
		err = frr.NeighborsMatchNodes(allNodes.Items, neighbors, ipFamily)
		return err
	}, 4*time.Minute, 1*time.Second).Should(BeNil())
}

func validateService(cs clientset.Interface, svc *corev1.Service, nodes []corev1.Node, c *frrcontainer.FRR) {
	port := strconv.Itoa(int(svc.Spec.Ports[0].Port))

	if len(svc.Status.LoadBalancer.Ingress) == 2 {
		ip1 := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
		ip2 := net.ParseIP(svc.Status.LoadBalancer.Ingress[1].IP)
		framework.ExpectNotEqual(ip1.To4(), ip2.To4())
	}
	for _, ip := range svc.Status.LoadBalancer.Ingress {

		ingressIP := e2eservice.GetIngressPoint(&ip)
		hostport := net.JoinHostPort(ingressIP, port)
		address := fmt.Sprintf("http://%s/", hostport)

		Eventually(func() error {
			err := wget.Do(address, c)
			if err != nil {
				return err
			}

			frrRoutesV4, frrRoutesV6, err := frr.Routes(c)
			if err != nil {
				return err
			}
			serviceIPFamily := "ipv4"
			frrRoutes, ok := frrRoutesV4[ingressIP]
			if !ok {
				frrRoutes, ok = frrRoutesV6[ingressIP]
				serviceIPFamily = "ipv6"
			}
			if !ok {
				return fmt.Errorf("%s not found in frr routes %v %v", ingressIP, frrRoutesV4, frrRoutesV6)
			}

			err = frr.RoutesMatchNodes(nodes, frrRoutes, serviceIPFamily)
			if err != nil {
				return err
			}

			// The BGP routes will not match the nodes if static routes were added.
			if !(c.Network == multiHopNetwork) {
				advertised := routes.ForIP(ingressIP, c)
				err = routes.MatchNodes(nodes, advertised, serviceIPFamily)
				if err != nil {
					return err
				}
			}

			return nil
		}, 4*time.Minute, 1*time.Second).Should(BeNil())
	}
}

func frrIsPairedOnPods(cs clientset.Interface, n *frrcontainer.FRR, ipFamily string) {
	pods, err := metallb.SpeakerPods(cs)
	framework.ExpectNoError(err)
	podExecutor := executor.ForPod(metallb.Namespace, pods[0].Name, "frr")

	Eventually(func() error {
		addresses := n.AddressesForFamily(ipFamily)
		for _, address := range addresses {
			toParse, err := podExecutor.Exec("vtysh", "-c", fmt.Sprintf("show bgp neighbor %s json", address))
			if err != nil {
				return err
			}
			res, err := frr.NeighborConnected(toParse)
			if err != nil {
				return err
			}
			if !res {
				return fmt.Errorf("expecting neighbor %s to be connected", n.Ipv4)
			}
		}
		return nil
	}, 4*time.Minute, 1*time.Second).Should(BeNil())
}

func checkBFDConfigPropagated(nodeConfig config.BfdProfile, peerConfig bgpfrr.BFDPeer) error {
	if peerConfig.Status != "up" {
		return fmt.Errorf("Peer status not up")
	}
	if peerConfig.RemoteReceiveInterval != int(*nodeConfig.ReceiveInterval) {
		return fmt.Errorf("RemoteReceiveInterval: expecting %d, got %d", *nodeConfig.ReceiveInterval, peerConfig.RemoteReceiveInterval)
	}
	if peerConfig.RemoteTransmitInterval != int(*nodeConfig.TransmitInterval) {
		return fmt.Errorf("RemoteTransmitInterval: expecting %d, got %d", *nodeConfig.TransmitInterval, peerConfig.RemoteTransmitInterval)
	}
	if peerConfig.RemoteEchoInterval != int(*nodeConfig.EchoInterval) {
		return fmt.Errorf("EchoInterval: expecting %d, got %d", *nodeConfig.EchoInterval, peerConfig.RemoteEchoInterval)
	}
	return nil
}
