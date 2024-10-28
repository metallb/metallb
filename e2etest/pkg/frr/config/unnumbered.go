// SPDX-License-Identifier:Apache-2.0

package config

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"go.universe.tf/e2etest/pkg/executor"
	corev1 "k8s.io/api/core/v1"
)

const UnnumberedPeerFRRConfig = `
frr defaults datacenter
hostname tor
no ipv6 forwarding
log file /tmp/frr.log
!
interface eth00
		ipv6 nd ra-interval 10
		no ipv6 nd suppress-ra
exit
!
interface eth01
		ipv6 nd ra-interval 10
		no ipv6 nd suppress-ra
exit
!
interface eth02
		ipv6 nd ra-interval 10
		no ipv6 nd suppress-ra
exit
!
interface lo
		ip address 200.100.100.1/24
exit
!
router bgp 65004
		bgp router-id 11.11.11.254
		neighbor MTLB peer-group
		neighbor MTLB passive
		neighbor MTLB remote-as external
		neighbor MTLB description LEAF-MTLB
		neighbor eth00 interface peer-group MTLB
		neighbor eth00 description k8s-node
		!
		address-family ipv4 unicast
				redistribute connected
				neighbor MTLB activate
		exit-address-family
		!
		address-family ipv6 unicast
				redistribute connected
				neighbor MTLB activate
		exit-address-family
	exit`

func WirePeer(peerName string, n corev1.Node) error {
	fromNetNS, err := exec.Command(executor.ContainerRuntime,
		"inspect", "-f", "{{ .NetworkSettings.SandboxKey }}", peerName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s - %w", fromNetNS, err)
	}
	fromNetNS = bytes.TrimSpace(fromNetNS)

	peer := executor.ForContainer(peerName)
	toNetNs, err := exec.Command(executor.ContainerRuntime,
		"inspect", "-f", "{{ .NetworkSettings.SandboxKey }}", n.GetName()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s - %w", toNetNs, err)
	}
	toNetNs = bytes.TrimSpace(toNetNs)

	c := fmt.Sprintf("ip link add eth00 netns %s type veth peer name net0", fromNetNS)
	if out, err := executor.Host.Exec("sudo", strings.Split(c, " ")...); err != nil {
		return fmt.Errorf("%s - %w", out, err)
	}

	c = fmt.Sprintf("ip link set dev net0 netns %s address de:ad:be:ff:11:60", toNetNs)
	if out, err := executor.Host.Exec("sudo", strings.Split(c, " ")...); err != nil {
		return fmt.Errorf("%s - %w", out, err)
	}

	node := executor.ForContainer(n.GetName())
	// c = fmt.Sprintf("-6 addr add 2001:db8:85a3::10%d/64 dev net0", i)
	// out, err = node.Exec("ip", strings.Split(c, " ")...)
	// if err != nil {
	//	panic(out)
	// }

	if out, err := node.Exec("ip", "link", "set", "dev", "net0", "up"); err != nil {
		return fmt.Errorf("%s - %w", out, err)
	}

	if out, err := peer.Exec("ip", "link", "set", "dev", "eth00", "up"); err != nil {
		return fmt.Errorf("%s - %w", out, err)
	}

	// if out, err := peer.Exec("ip", "-6", "addr", "add", fmt.Sprintf("2001:db8:85a3::1%d/64", i), "dev", fmt.Sprintf("eth0%d", i)); err != nil {
	//return fmt.Errorf("%s - %w", out, err)
	// }
	return nil
}
