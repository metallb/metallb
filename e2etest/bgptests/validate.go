// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	"go.universe.tf/metallb/e2etest/pkg/routes"
	"go.universe.tf/metallb/e2etest/pkg/wget"
	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	"go.universe.tf/metallb/internal/ipfamily"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

func validateFRRPeeredWithAllNodes(cs clientset.Interface, c *frrcontainer.FRR, ipFamily ipfamily.Family) {
	allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	framework.ExpectNoError(err)
	validateFRRPeeredWithNodes(cs, allNodes.Items, c, ipFamily)
}

func validateFRRNotPeeredWithNodes(cs clientset.Interface, nodes []corev1.Node, c *frrcontainer.FRR, ipFamily ipfamily.Family) {
	for _, node := range nodes {
		ginkgo.By(fmt.Sprintf("checking node %s is not peered with the frr instance %s", node.Name, c.Name))
		Eventually(func() error {
			neighbors, err := frr.NeighborsInfo(c)
			framework.ExpectNoError(err)
			err = frr.NeighborsMatchNodes([]corev1.Node{node}, neighbors, ipFamily, c.RouterConfig.VRF)
			return err
		}, 4*time.Minute, 1*time.Second).Should(MatchError(ContainSubstring("not established")))
	}
}

func validateFRRPeeredWithNodes(cs clientset.Interface, nodes []corev1.Node, c *frrcontainer.FRR, ipFamily ipfamily.Family) {
	ginkgo.By(fmt.Sprintf("checking nodes are peered with the frr instance %s", c.Name))
	Eventually(func() error {
		neighbors, err := frr.NeighborsInfo(c)
		framework.ExpectNoError(err)
		err = frr.NeighborsMatchNodes(nodes, neighbors, ipFamily, c.RouterConfig.VRF)
		if err != nil {
			return fmt.Errorf("failed to match neighbors for %s, %w", c.Name, err)
		}
		return nil
	}, 4*time.Minute, 1*time.Second).Should(BeNil())
}

func validateService(cs clientset.Interface, svc *corev1.Service, nodes []corev1.Node, c *frrcontainer.FRR) {
	Eventually(func() error {
		return validateServiceNoWait(cs, svc, nodes, c)
	}, 4*time.Minute, 1*time.Second).Should(BeNil())
}

func validateServiceNoWait(cs clientset.Interface, svc *corev1.Service, nodes []corev1.Node, c *frrcontainer.FRR) error {
	port := strconv.Itoa(int(svc.Spec.Ports[0].Port))

	if len(svc.Status.LoadBalancer.Ingress) == 2 {
		ip1 := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
		ip2 := net.ParseIP(svc.Status.LoadBalancer.Ingress[1].IP)
		framework.ExpectNotEqual(ip1.To4(), ip2.To4())
	}
	for _, ip := range svc.Status.LoadBalancer.Ingress {
		ingressIP := e2eservice.GetIngressPoint(&ip)

		// TODO: in case of VRF there's currently no host wiring to the service.
		// We only validate the routes are propagated correctly but
		// we don't try to hit the service.
		if c.RouterConfig.VRF == "" {
			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)
			err := wget.Do(address, c)
			if err != nil {
				return err
			}
		}

		frrRoutesV4, frrRoutesV6, err := frr.Routes(c)
		if err != nil {
			return err
		}
		serviceIPFamily := ipfamily.IPv4
		frrRoutes, ok := frrRoutesV4[ingressIP]
		if !ok {
			frrRoutes, ok = frrRoutesV6[ingressIP]
			serviceIPFamily = ipfamily.IPv6
		}
		if !ok {
			return fmt.Errorf("%s not found in frr routes %v %v", ingressIP, frrRoutesV4, frrRoutesV6)
		}
		if !strings.EqualFold(frrRoutes.Origin, "IGP") {
			return fmt.Errorf("route for %s not set with igp origin", ingressIP)
		}

		err = frr.RoutesMatchNodes(nodes, frrRoutes, serviceIPFamily, c.RouterConfig.VRF)
		if err != nil {
			return err
		}

		// The BGP routes will not match the nodes if static routes were added.
		if c.Network != defaultNextHopSettings.multiHopNetwork &&
			c.Network != vrfNextHopSettings.multiHopNetwork {
			advertised := routes.ForIP(ingressIP, c)
			err = routes.MatchNodes(nodes, advertised, serviceIPFamily, c.RouterConfig.VRF)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func frrIsPairedOnPods(cs clientset.Interface, n *frrcontainer.FRR, ipFamily ipfamily.Family) {
	pods, err := metallb.SpeakerPods(cs)
	framework.ExpectNoError(err)
	podExecutor := executor.ForPod(metallb.Namespace, pods[0].Name, "frr")

	Eventually(func() error {
		addresses := n.AddressesForFamily(ipFamily)
		for _, address := range addresses {
			vrfSelector := ""
			if n.RouterConfig.VRF != "" {
				vrfSelector = fmt.Sprintf("vrf %s", n.RouterConfig.VRF)
			}
			toParse, err := podExecutor.Exec("vtysh", "-c", fmt.Sprintf("show bgp %s neighbor %s json", vrfSelector, address))
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

func checkBFDConfigPropagated(nodeConfig metallbv1beta1.BFDProfile, peerConfig bgpfrr.BFDPeer) error {
	if peerConfig.Status != "up" {
		return fmt.Errorf("Peer status not up")
	}
	if peerConfig.RemoteReceiveInterval != int(*nodeConfig.Spec.ReceiveInterval) {
		return fmt.Errorf("RemoteReceiveInterval: expecting %d, got %d", *nodeConfig.Spec.ReceiveInterval, peerConfig.RemoteReceiveInterval)
	}
	if peerConfig.RemoteTransmitInterval != int(*nodeConfig.Spec.TransmitInterval) {
		return fmt.Errorf("RemoteTransmitInterval: expecting %d, got %d", *nodeConfig.Spec.TransmitInterval, peerConfig.RemoteTransmitInterval)
	}
	if peerConfig.RemoteEchoReceiveInterval != int(*nodeConfig.Spec.EchoInterval) {
		return fmt.Errorf("EchoInterval: expecting %d, got %d", *nodeConfig.Spec.EchoInterval, peerConfig.RemoteEchoReceiveInterval)
	}
	return nil
}

func validateDesiredLB(svc *corev1.Service) {
	desiredLbIPs := svc.Annotations["metallb.universe.tf/loadBalancerIPs"]
	if desiredLbIPs == "" {
		return
	}
	framework.ExpectEqual(desiredLbIPs, strings.Join(getIngressIPs(svc.Status.LoadBalancer.Ingress), ","))
}

func checkServiceOnlyOnNodes(cs clientset.Interface, svc *corev1.Service, expectedNodes []corev1.Node, ipFamily ipfamily.Family) {
	if len(expectedNodes) == 0 {
		return
	}
	ip := svc.Status.LoadBalancer.Ingress[0].IP

	for _, c := range FRRContainers {
		nodeIps, err := k8s.NodeIPsForFamily(expectedNodes, ipFamily, c.RouterConfig.VRF)
		framework.ExpectNoError(err)
		validateService(cs, svc, expectedNodes, c)
		Eventually(func() error {
			routes, err := frr.RoutesForFamily(c, ipFamily)
			if len(routes[ip].NextHops) != len(nodeIps) {
				return fmt.Errorf("%s: invalid number of routes for %s: expecting %d got %d", c.Name, ip, len(nodeIps), len(routes[ip].NextHops))
			}

		OUTER:
			for _, n := range routes[ip].NextHops {
				for _, ip := range nodeIps {
					if n.String() == ip {
						continue OUTER
					}
				}
				return fmt.Errorf("UnexpectedIP found %s, nodes %s in container %s for service %s", n.String(), nodeIps, c.Name, ip)
			}
			return err
		}, time.Minute, time.Second).Should(Not(HaveOccurred()))
	}
}

func checkServiceNotOnNodes(cs clientset.Interface, svc *corev1.Service, expectedNodes []corev1.Node, ipFamily ipfamily.Family) {
	if len(expectedNodes) == 0 {
		return
	}
	ip := svc.Status.LoadBalancer.Ingress[0].IP

	for _, c := range FRRContainers {
		nodeIps, err := k8s.NodeIPsForFamily(expectedNodes, ipFamily, c.RouterConfig.VRF)
		framework.ExpectNoError(err)
		Eventually(func() bool {
			routes, err := frr.RoutesForFamily(c, ipFamily)
			framework.ExpectNoError(err)
			for _, n := range routes[ip].NextHops {
				for _, ip := range nodeIps {
					if n.String() == ip {
						return true
					}
				}
			}
			return false
		}, time.Minute, time.Second).Should(BeFalse())
	}
}

func checkCommunitiesOnlyOnNodes(cs clientset.Interface, svc *corev1.Service, community string, expectedNodes []corev1.Node, ipFamily ipfamily.Family) {
	if len(expectedNodes) == 0 {
		return
	}
	ip := svc.Status.LoadBalancer.Ingress[0].IP

	for _, c := range FRRContainers {
		nodeIps, err := k8s.NodeIPsForFamily(expectedNodes, ipFamily, c.RouterConfig.VRF)
		framework.ExpectNoError(err)

		Eventually(func() error {
			routes, err := frr.RoutesForCommunity(c, community, ipFamily)
			if len(routes[ip].NextHops) != len(nodeIps) {
				return fmt.Errorf("%s: invalid number of routes for %s: expecting %d got %d", c.Name, ip, len(nodeIps), len(routes[ip].NextHops))
			}

		OUTER:
			for _, n := range routes[ip].NextHops {
				for _, ip := range nodeIps {
					if n.String() == ip {
						continue OUTER
					}
				}
				return fmt.Errorf("UnexpectedIP found %s, nodes %s in container %s for service %s", n.String(), nodeIps, c.Name, ip)
			}
			return err
		}, 10*time.Minute, time.Second).Should(Not(HaveOccurred()))
	}
}

func nodesForSelection(nodes []corev1.Node, selected []int) []corev1.Node {
	selectedNodes := []corev1.Node{}
	for _, i := range selected {
		if i >= len(nodes) {
			ginkgo.Skip("Not enough nodes")
		}
		selectedNodes = append(selectedNodes, nodes[i])
	}
	return selectedNodes
}

func nodesNotSelected(nodes []corev1.Node, selected []int) []corev1.Node {
	nonSelectedNodes := []corev1.Node{}
OUTER:
	for i, n := range nodes {
		for _, j := range selected {
			if i == j {
				continue OUTER
			}
		}
		nonSelectedNodes = append(nonSelectedNodes, n)
	}

	return nonSelectedNodes
}

func getIngressIPs(ingresses []corev1.LoadBalancerIngress) []string {
	var ips []string
	for _, ingress := range ingresses {
		ips = append(ips, ingress.IP)
	}
	return ips
}

func validateServiceNotAdvertised(svc *corev1.Service, frrContainers []*frrcontainer.FRR, advertised string, ipFamily ipfamily.Family) {
	for _, c := range frrContainers {
		if c.Name != advertised {
			for _, ip := range svc.Status.LoadBalancer.Ingress {
				ingressIP := e2eservice.GetIngressPoint(&ip)

				Eventually(func() bool {
					frrRoutesV4, frrRoutesV6, err := frr.Routes(c)
					if err != nil {
						framework.ExpectNoError(err)
					}

					_, ok := frrRoutesV4[ingressIP]
					if ipFamily == ipfamily.IPv6 {
						_, ok = frrRoutesV6[ingressIP]
					}

					return ok
				}, 4*time.Minute, 1*time.Second).Should(Equal(false))
			}
		}
	}
}

func validateServiceInRoutesForCommunity(c *frrcontainer.FRR, community string, family ipfamily.Family, svc *corev1.Service) {
	Eventually(func() error {
		routes, err := frr.RoutesForCommunity(c, community, family)
		if err != nil {
			return err
		}
		for _, ip := range svc.Status.LoadBalancer.Ingress {
			ingressIP := e2eservice.GetIngressPoint(&ip)
			if _, ok := routes[ingressIP]; !ok {
				return fmt.Errorf("service IP %s not in routes", ingressIP)
			}
		}
		return nil
	}, 4*time.Minute, 1*time.Second).Should(Not(HaveOccurred()))
}

func validateServiceNotInRoutesForCommunity(c *frrcontainer.FRR, community string, family ipfamily.Family, svc *corev1.Service) {
	Eventually(func() error {
		routes, err := frr.RoutesForCommunity(c, community, family)
		if err != nil {
			return err
		}
		for _, ip := range svc.Status.LoadBalancer.Ingress {
			ingressIP := e2eservice.GetIngressPoint(&ip)
			if _, ok := routes[ingressIP]; !ok {
				return fmt.Errorf("service IP %s not in routes", ingressIP)
			}
		}
		return nil
	}, 4*time.Minute, 1*time.Second).Should(MatchError(ContainSubstring("not in routes")))
}
