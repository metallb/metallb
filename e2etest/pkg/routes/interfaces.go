// SPDX-License-Identifier:Apache-2.0

package routes

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"go.universe.tf/metallb/e2etest/pkg/executor"
)

type InterfaceAddress struct {
	Ifname   string `json:"ifname"`
	AddrInfo []struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		Prefixlen int    `json:"prefixlen"`
		Scope     string `json:"scope"`
	} `json:"addr_info"`
}

func InterfaceForAddress(exec executor.Executor, ipv4Address, ipv6Address string) (string, error) {
	jsonAddr, err := exec.Exec("ip", "-j", "a")
	if err != nil {
		return "", err
	}
	res, err := findInterfaceWithAddresses(jsonAddr, ipv4Address, ipv6Address)
	if err != nil {
		return "", err
	}
	return res, nil
}

type InterfaceLink struct {
	Ifname string `json:"ifname"`
	Master string `json:"master"`
}

type InterfaceNotFoundErr struct {
	interfaceName string
}

func (e *InterfaceNotFoundErr) Error() string {
	return fmt.Sprintf("interface %s not found", e.interfaceName)
}

func InterfaceExists(exec executor.Executor, intf string) error {
	interfaces, err := ipLink(exec)
	if err != nil {
		return err
	}
	for _, i := range interfaces {
		if i.Ifname == intf {
			return nil
		}
	}
	return &InterfaceNotFoundErr{interfaceName: intf}
}

func CreateVRF(exec executor.Executor, vrfName string) error {
	err := InterfaceExists(exec, vrfName)
	// The interface is already there, not doing anything
	if err == nil {
		return nil
	}
	var notFound *InterfaceNotFoundErr
	if err != nil && !errors.As(err, &notFound) {
		return err
	}

	out, err := exec.Exec("ip", "link", "add", vrfName, "type", "vrf", "table", "2")
	if err != nil {
		return errors.Wrapf(err, "failed to create vrf %s : %s", vrfName, out)
	}
	out, err = exec.Exec("ip", "link", "set", "dev", vrfName, "up")
	if err != nil {
		return errors.Wrapf(err, "failed to set vrf %s up : %s", vrfName, out)
	}
	return nil
}

func AddInterfaceToVRF(exec executor.Executor, intf, vrf, ipv6Address string) error {
	links, err := ipLink(exec)
	if err != nil {
		return err
	}
	for _, l := range links {
		if l.Ifname != intf {
			continue
		}
		if l.Master != "" && l.Master != vrf {
			return fmt.Errorf("interface %s already has a master %s different from %s", intf, l.Master, vrf)
		}
		if l.Master == vrf { // Already set
			return nil
		}
	}
	out, err := exec.Exec("ip", "link", "set", "dev", intf, "master", vrf)
	if err != nil {
		return errors.Wrapf(err, "failed to set master %s to %s : %s", vrf, intf, out)
	}
	// we need this because moving the interface to the vrf removes the v6 IP
	out, err = exec.Exec("ip", "-6", "addr", "add", ipv6Address+"/64", "dev", intf)
	if err != nil {
		return errors.Wrapf(err, "failed to add %s to %s : %s", ipv6Address, intf, out)
	}
	return nil
}

func findInterfaceWithAddresses(jsonIPOutput string, ipv4Address, ipv6Address string) (string, error) {
	var interfaces []InterfaceAddress
	err := json.Unmarshal([]byte(jsonIPOutput), &interfaces)
	if err != nil {
		return "", err
	}

	foundV4, foundV6 := false, false
	for _, i := range interfaces {
		for _, info := range i.AddrInfo {
			if info.Family == "inet" && info.Local == ipv4Address {
				foundV4 = true
			}
			if info.Family == "inet6" && info.Scope == "global" && info.Local == ipv6Address {
				foundV6 = true
			}
		}
		if foundV4 && !foundV6 {
			return "", fmt.Errorf("interface %s has v4 address %s but not v6 address %s", i.Ifname, ipv4Address, ipv6Address)
		}
		if !foundV4 && foundV6 {
			return "", fmt.Errorf("interface %s does not have v4 address %s but has v6 address %s", i.Ifname, ipv4Address, ipv6Address)
		}
		if foundV4 && foundV6 {
			return i.Ifname, nil
		}
	}
	return "", fmt.Errorf("could not find interface on with ipv4 address %s and ipv6 address %s", ipv4Address, ipv6Address)
}

func ipLink(exec executor.Executor) ([]InterfaceLink, error) {
	links, err := exec.Exec("ip", "-j", "l")
	if err != nil {
		return nil, err
	}
	var interfaces []InterfaceLink
	err = json.Unmarshal([]byte(links), &interfaces)
	if err != nil {
		return nil, err
	}
	return interfaces, nil
}
