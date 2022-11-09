// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"fmt"
	"os"
	"strings"

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
	multiHopNetwork      = "multi-hop-net"
	vrfNetwork           = "vrf-net"
	vrfName              = "red"
	metalLBASN           = 64512
	externalASN          = 4200000000
	nextHopContainerName = "ebgp-single-hop"
)

var (
	containersNetwork string
	hostIPv4          string
	hostIPv6          string
	multiHopRoutes    map[string]container.NetworkSettings
	FRRContainers     []*frrcontainer.FRR
	VRFFRRContainers  []*frrcontainer.FRR
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
func InfraSetup(ipv4Addresses, ipv6Addresses []string, externalContainers string, cs *clientset.Clientset) ([]*frrcontainer.FRR, []*frrcontainer.FRR, error) {
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

	ebgpSingleHopContainerVRFConfig := frrcontainer.Config{
		Name:    "ebgp-vrf-single-hop",
		Network: vrfNetwork,
		Neighbor: frrconfig.NeighborConfig{
			ASN:      metalLBASN,
			MultiHop: false,
		},
		Router: frrconfig.RouterConfig{
			ASN:     externalASN,
			BGPPort: 179,
			VRF:     vrfName,
		},
	}

	if externalContainers != "" {
		err := validateContainersNames(externalContainers)
		if err != nil {
			return nil, nil, err
		}

		configs, err := configsFor(externalContainers, ibgpSingleHopContainerConfig, ibgpMultiHopContainerConfig,
			ebgpMultiHopContainerConfig, ebgpSingleHopContainerConfig, ebgpSingleHopContainerVRFConfig)
		if err != nil {
			return nil, nil, err
		}

		res, err := frrcontainer.ConfigureExisting(configs...)
		if err != nil {
			return nil, nil, err
		}

		if containsMultiHop(res) {
			err = multiHopSetUp(res, cs)
			if err != nil {
				return nil, nil, err
			}
		}
		return res, []*frrcontainer.FRR{}, nil
	}
	if containersNetwork == "host" {
		res, err := frrcontainer.Create(ibgpSingleHopContainerConfig)
		if err != nil {
			return nil, nil, err
		}
		return res, []*frrcontainer.FRR{}, nil
	}

	Expect(len(ipv4Addresses)).Should(BeNumerically(">=", 2))
	Expect(len(ipv6Addresses)).Should(BeNumerically(">=", 2))

	ibgpSingleHopContainerConfig.IPv4Address = ipv4Addresses[0]
	ibgpSingleHopContainerConfig.IPv6Address = ipv6Addresses[0]
	ebgpSingleHopContainerConfig.IPv4Address = ipv4Addresses[1]
	ebgpSingleHopContainerConfig.IPv6Address = ipv6Addresses[1]

	var out string
	out, err := executor.Host.Exec(executor.ContainerRuntime, "network", "create", multiHopNetwork, "--ipv6",
		"--driver=bridge", "--subnet=172.30.0.0/16", "--subnet=fc00:f853:ccd:e798::/64")
	if err != nil && !strings.Contains(out, "already exists") {
		return nil, nil, errors.Wrapf(err, "failed to create %s: %s", multiHopNetwork, out)
	}

	out, err = executor.Host.Exec(executor.ContainerRuntime, "network", "create", vrfNetwork, "--ipv6",
		"--driver=bridge", "--subnet=172.31.0.0/16", "--subnet=fc00:f853:ccd:e799::/64")
	if err != nil && !strings.Contains(out, "already exists") {
		return nil, nil, errors.Wrapf(err, "failed to create %s: %s", vrfNetwork, out)
	}

	containers, err := frrcontainer.Create(ibgpSingleHopContainerConfig, ibgpMultiHopContainerConfig,
		ebgpMultiHopContainerConfig, ebgpSingleHopContainerConfig)
	if err != nil {
		return nil, nil, err
	}
	vrfContainers, err := frrcontainer.Create(ebgpSingleHopContainerVRFConfig)

	err = multiHopSetUp(containers, cs)
	if err != nil {
		return nil, nil, err
	}
	err = vrfSetup(cs)
	if err != nil {
		return nil, nil, err
	}
	return containers, vrfContainers, nil
}

// multiHopSetUp connects the ebgp-single-hop container to the multi-hop-net network,
// and creates the required static routes between the multi-hop containers and the speaker pods.
func multiHopSetUp(containers []*frrcontainer.FRR, cs *clientset.Clientset) error {
	out, err := executor.Host.Exec(executor.ContainerRuntime, "network", "connect",
		multiHopNetwork, nextHopContainerName)
	if err != nil {
		return errors.Wrapf(err, "Failed to connect %s to %s: %s", nextHopContainerName, multiHopNetwork, out)
	}

	multiHopRoutes, err = container.Networks(nextHopContainerName)
	if err != nil {
		return err
	}

	for _, c := range containers {
		if c.Network == multiHopNetwork {
			err = container.AddMultiHop(c, c.Network, containersNetwork, multiHopRoutes)
			if err != nil {
				return errors.Wrapf(err, "Failed to set up the multi-hop network for container %s", c.Name)
			}
		}
	}
	err = addMultiHopToNodes(cs)
	if err != nil {
		return errors.Wrapf(err, "Failed to set up the multi-hop network")
	}

	return nil
}

func vrfSetup(cs *clientset.Clientset) error {
	speakerPods, err := metallb.SpeakerPods(cs)
	if err != nil {
		return err
	}
	for _, pod := range speakerPods {
		err := addContainerToNetwork(pod.Spec.NodeName, vrfNetwork)
		if err != nil {
			return errors.Wrapf(err, "Failed to connect %s to %s", pod.Spec.NodeName, vrfNetwork)
		}

		err = container.SetupVRFForNetwork(pod.Spec.NodeName, vrfNetwork, vrfName)
		if err != nil {
			return err
		}
	}
	return nil
}

// InfraTearDown tears down the containers and the routes needed for bgp testing.
func InfraTearDown(cs *clientset.Clientset, containers []*frrcontainer.FRR) error {
	err := frrcontainer.Delete(containers)
	if err != nil {
		return err
	}

	err = multiHopTearDown(cs)
	if err != nil {
		return err
	}

	err = vrfTeardown(cs)
	if err != nil {
		return err
	}

	return nil
}

func multiHopTearDown(cs *clientset.Clientset) error {
	_, err := executor.Host.Exec(executor.ContainerRuntime, "network", "inspect", multiHopNetwork)
	if err != nil {
		// do nothing if the multi-hop network doesn't exist.
		return nil
	}

	out, err := executor.Host.Exec(executor.ContainerRuntime, "network", "rm", multiHopNetwork)
	if err != nil {
		return errors.Wrapf(err, "Failed to remove %s: %s", multiHopNetwork, out)
	}
	speakerPods, err := metallb.SpeakerPods(cs)
	if err != nil {
		return err
	}
	for _, pod := range speakerPods {
		nodeExec := executor.ForContainer(pod.Spec.NodeName)
		err = container.DeleteMultiHop(nodeExec, containersNetwork, multiHopNetwork, multiHopRoutes)
		if err != nil {
			return errors.Wrapf(err, "Failed to delete multihop routes for pod %s", pod.ObjectMeta.Name)
		}

	}

	return nil
}

func vrfTeardown(cs *clientset.Clientset) error {
	_, err := executor.Host.Exec(executor.ContainerRuntime, "network", "inspect", vrfNetwork)
	if err != nil {
		return nil
	}

	speakerPods, err := metallb.SpeakerPods(cs)
	if err != nil {
		return err
	}

	for _, pod := range speakerPods {
		err := removeContainerFromNetwork(pod.Spec.NodeName, vrfNetwork)
		if err != nil {
			return err
		}
	}

	out, err := executor.Host.Exec(executor.ContainerRuntime, "network", "rm", vrfNetwork)
	if err != nil {
		return errors.Wrapf(err, "Failed to remove %s: %s", multiHopNetwork, out)
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

// validateContainersNames validates that the given string is a comma separated list of containers names.
// The valid names are: ibgp-single-hop / ibgp-multi-hop / ebgp-single-hop / ebgp-multi-hop.
func validateContainersNames(containerNames string) error {
	if len(containerNames) == 0 {
		return fmt.Errorf("Failed to validate containers names: got empty string")
	}
	validNames := map[string]bool{
		"ibgp-single-hop": true,
		"ibgp-multi-hop":  true,
		"ebgp-single-hop": true,
		"ebgp-multi-hop":  true,
	}
	names := strings.Split(containerNames, ",")
	for _, n := range names {
		v, ok := validNames[n]
		if !ok {
			return fmt.Errorf("Failed to validate container name: %s invalid name", n)
		}
		if !v {
			return fmt.Errorf("Failed to validate container name: %s duplicate name", n)
		}
		validNames[n] = false
	}

	return nil
}

// configsFor returns the frr configs corresponding to the given comma separated list of containers names.
func configsFor(containerNames string, frrContainersConfigs ...frrcontainer.Config) ([]frrcontainer.Config, error) {
	var configs []frrcontainer.Config
	names := strings.Split(containerNames, ",")
	for _, n := range names {
		for _, containerCfg := range frrContainersConfigs {
			if n == containerCfg.Name {
				configs = append(configs, containerCfg)
				break
			}
		}
	}

	return configs, nil
}

// containsMultiHop returns true if the given containers list include a multi-hop container.
func containsMultiHop(frrContainers []*frrcontainer.FRR) bool {
	var multiHop = false
	for _, frr := range frrContainers {
		if strings.Contains(frr.Name, "multi-hop") {
			multiHop = true
		}
	}

	return multiHop
}

func addContainerToNetwork(containerName, network string) error {
	networks, err := container.Networks(containerName)
	if err != nil {
		return err
	}
	if _, ok := networks[network]; ok {
		return nil
	}

	out, err := executor.Host.Exec(executor.ContainerRuntime, "network", "connect",
		network, containerName)
	if err != nil {
		return errors.Wrapf(err, "Failed to connect %s to %s: %s", containerName, network, out)
	}
	return nil
}

func removeContainerFromNetwork(containerName, network string) error {
	networks, err := container.Networks(containerName)
	if err != nil {
		return err
	}
	if _, ok := networks[network]; !ok {
		return nil
	}

	out, err := executor.Host.Exec(executor.ContainerRuntime, "network", "disconnect",
		network, containerName)
	if err != nil {
		return errors.Wrapf(err, "Failed to disconnect %s from %s: %s", containerName, network, out)
	}
	return nil
}
