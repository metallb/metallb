// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"os"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/container"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	frrconfig "go.universe.tf/metallb/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	multiHopNetwork = "multi-hop-net"
	metalLBASN      = 64512
	externalASN     = 64513
)

var (
	containersNetwork string
	hostIPv4          string
	hostIPv6          string
	multiHopRoutes    map[string]container.NetworkSettings
	FRRContainers     []*frrcontainer.FRR
)

func init() {
	if _, res := os.LookupEnv("RUN_FRR_CONTAINER_ON_HOST_NETWORK"); res {
		containersNetwork = "host"
	} else {
		containersNetwork = "kind"
	}

	if ip := os.Getenv("PROVISIONING_HOST_EXTERNAL_IPV4"); len(ip) != 0 {
		hostIPv4 = ip
	}
	if ip := os.Getenv("PROVISIONING_HOST_EXTERNAL_IPV6"); len(ip) != 0 {
		hostIPv6 = ip
	}
}

// InfraSetup brings up the external container mimicking external routers, and set up the routing needed for
// testing.
func InfraSetup(ipv4Addresses, ipv6Addresses []string, cs *clientset.Clientset) ([]*frrcontainer.FRR, error) {
	/*
		We have 2 ways in which we setup the containers for the tests:
		1 - The user requested the containers to use the 'host' network
		so we spin up only one ibgp container.
		2 - The user specified (or didn't at all) a container network that
		is not 'host'. In that case he needs to supply 2 IPs for the containers.
		Then we spin up a total of 4 containers:
		  * ibgp container that uses the first IP, a single-hop away from our speakers (1st).
		  * ebgp container that uses the second IP, a single-hop away from our speakers,
		    and is connected to another containers network "multi-hop-net" (2nd).
		  * two ibgp/ebgp containers connected to the "multi-hop-net", multi-hops away
		    from our speakers (3rd,4th).
		We then wire these networks by adding static routes to both the speaker nodes
		containers (we're using kind) and the ibgp/ebgp containers connected to multi-hop-net,
		using the 2nd container as a gateway.

		See `e2etest/README.md` for more details.
	*/

	ibgpSingleHopContainerConfig := frrcontainer.Config{
		Name: "ibgp-single-hop",
		Neighbor: frrconfig.NeighborConfig{
			ASN:      metalLBASN,
			Password: "ibgp-test",
			MultiHop: false,
		},
		Router: frrconfig.RouterConfig{
			ASN:      metalLBASN,
			BGPPort:  179,
			Password: "ibgp-test",
		},
		Network:  containersNetwork,
		HostIPv4: hostIPv4,
		HostIPv6: hostIPv6,
	}
	ibgpMultiHopContainerConfig := frrcontainer.Config{
		Name: "ibgp-multi-hop",
		Neighbor: frrconfig.NeighborConfig{
			ASN:      metalLBASN,
			Password: "ibgp-test",
			MultiHop: true,
		},
		Router: frrconfig.RouterConfig{
			ASN:      metalLBASN,
			BGPPort:  180,
			Password: "ibgp-test",
		},
		Network:  multiHopNetwork,
		HostIPv4: hostIPv4,
		HostIPv6: hostIPv6,
	}
	ebgpMultiHopContainerConfig := frrcontainer.Config{
		Name: "ebgp-multi-hop",
		Neighbor: frrconfig.NeighborConfig{
			ASN:      metalLBASN,
			Password: "ebgp-test",
			MultiHop: true,
		},
		Router: frrconfig.RouterConfig{
			ASN:      externalASN,
			BGPPort:  180,
			Password: "ebgp-test",
		},
		Network:  multiHopNetwork,
		HostIPv4: hostIPv4,
		HostIPv6: hostIPv6,
	}
	ebgpSingleHopContainerConfig := frrcontainer.Config{
		Name:    "ebgp-single-hop",
		Network: containersNetwork,
		Neighbor: frrconfig.NeighborConfig{
			ASN:      metalLBASN,
			MultiHop: false,
		},
		Router: frrconfig.RouterConfig{
			ASN:     externalASN,
			BGPPort: 179,
		},
	}

	var res []*frrcontainer.FRR
	var err error
	if containersNetwork == "host" {
		res, err = frrcontainer.Create(ibgpSingleHopContainerConfig)
	} else {
		Expect(len(ipv4Addresses)).Should(BeNumerically(">=", 2))
		Expect(len(ipv6Addresses)).Should(BeNumerically(">=", 2))

		ibgpSingleHopContainerConfig.IPv4Address = ipv4Addresses[0]
		ibgpSingleHopContainerConfig.IPv6Address = ipv6Addresses[0]
		ebgpSingleHopContainerConfig.IPv4Address = ipv4Addresses[1]
		ebgpSingleHopContainerConfig.IPv6Address = ipv6Addresses[1]

		var out string
		out, err = executor.Host.Exec(executor.ContainerRuntime, "network", "create", multiHopNetwork, "--ipv6",
			"--driver=bridge", "--subnet=172.30.0.0/16", "--subnet=fc00:f853:ccd:e798::/64")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create %s: %s", multiHopNetwork, out)
		}

		res, err = frrcontainer.Create(ibgpSingleHopContainerConfig, ibgpMultiHopContainerConfig,
			ebgpMultiHopContainerConfig, ebgpSingleHopContainerConfig)
		if err != nil {
			return nil, err
		}

		out, err = executor.Host.Exec(executor.ContainerRuntime, "network", "connect",
			multiHopNetwork, ebgpSingleHopContainerConfig.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to connect %s to %s: %s", ebgpSingleHopContainerConfig.Name, multiHopNetwork, out)
		}

		multiHopRoutes, err = container.Networks(ebgpSingleHopContainerConfig.Name)
		if err != nil {
			return nil, err
		}

		for _, c := range res {
			if c.Network == multiHopNetwork {
				err = container.AddMultiHop(c, c.Network, containersNetwork, multiHopRoutes)
				if err != nil {
					return res, err
				}
			}
		}
		err = addMultiHopToNodes(cs)
		if err != nil {
			return nil, err
		}

	}
	return res, err
}

// InfraTearDown tears down the containers and the routes needed for bgp testing.
func InfraTearDown(containers []*frrcontainer.FRR, cs *clientset.Clientset) error {
	err := frrcontainer.Stop(containers)
	if err != nil {
		return err
	}

	if containersNetwork != "host" {
		out, err := executor.Host.Exec(executor.ContainerRuntime, "network", "rm", multiHopNetwork)
		if err != nil {
			return errors.Wrapf(err, "failed to remove %s: %s", multiHopNetwork, out)
		}
		speakerPods, err := metallb.SpeakerPods(cs)
		if err != nil {
			return err
		}
		for _, pod := range speakerPods {
			nodeExec := executor.ForContainer(pod.Spec.NodeName)
			err = container.DeleteMultiHop(nodeExec, containersNetwork, multiHopNetwork, multiHopRoutes)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

// Allow the speaker nodes to reach the multi-hop network containers.
func addMultiHopToNodes(cs *clientset.Clientset) error {
	/*
		When "host" network is not specified we assume that the tests
		run on a kind cluster, where all the nodes are actually containers
		on our pc. This allows us to create containerExecutors for the speakers
		nodes, and edit their routes without any added privileges.
	*/
	speakerPods, err := metallb.SpeakerPods(cs)
	if err != nil {
		return err
	}
	for _, pod := range speakerPods {
		nodeExec := executor.ForContainer(pod.Spec.NodeName)
		err := container.AddMultiHop(nodeExec, containersNetwork, multiHopNetwork, multiHopRoutes)
		if err != nil {
			return err
		}
	}
	return nil
}
