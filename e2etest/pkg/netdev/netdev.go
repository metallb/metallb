// SPDX-License-Identifier:Apache-2.0

package netdev

import (
	"encoding/json"
	"fmt"

	"errors"

	"go.universe.tf/e2etest/pkg/executor"
)

type interfaceAddress struct {
	Ifname   string `json:"ifname"`
	AddrInfo []struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		Prefixlen int    `json:"prefixlen"`
		Scope     string `json:"scope"`
	} `json:"addr_info"`
}

func ForAddress(exec executor.Executor, ipv4Address, ipv6Address string) (string, error) {
	jsonAddr, err := exec.Exec("ip", "-j", "a")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve addresses %w :%s", err, jsonAddr)
	}
	res, err := findInterfaceWithAddresses(jsonAddr, ipv4Address, ipv6Address)
	if err != nil {
		return "", err
	}
	return res, nil
}

type Link struct {
	Ifname string `json:"ifname"`
	Master string `json:"master"`
}

type NotFoundErr struct {
	interfaceName string
}

func (e *NotFoundErr) Error() string {
	return fmt.Sprintf("interface %s not found", e.interfaceName)
}

func Exists(exec executor.Executor, intf string) error {
	interfaces, err := ipLink(exec)
	if err != nil {
		return err
	}
	for _, i := range interfaces {
		if i.Ifname == intf {
			return nil
		}
	}
	return &NotFoundErr{interfaceName: intf}
}

func CreateVRF(exec executor.Executor, vrfName, routingTable string) error {
	err := Exists(exec, vrfName)
	// The interface is already there, not doing anything
	if err == nil {
		return nil
	}
	var notFound *NotFoundErr
	if err != nil && !errors.As(err, &notFound) {
		return err
	}

	out, err := exec.Exec("ip", "link", "add", vrfName, "type", "vrf", "table", routingTable)
	if err != nil {
		return errors.Join(err, fmt.Errorf("failed to create vrf %s : %s", vrfName, out))
	}
	out, err = exec.Exec("ip", "link", "set", "dev", vrfName, "up")
	if err != nil {
		return errors.Join(err, fmt.Errorf("failed to set vrf %s up : %s", vrfName, out))
	}
	return nil
}

func AddToVRF(exec executor.Executor, intf, vrf, ipv6Address string) error {
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
		return errors.Join(err, fmt.Errorf("failed to set master %s to %s : %s", vrf, intf, out))
	}
	// we need this because moving the interface to the vrf removes the v6 IP
	out, err = exec.Exec("ip", "-6", "addr", "add", ipv6Address+"/64", "dev", intf)
	if err != nil {
		return errors.Join(err, fmt.Errorf("failed to add %s to %s : %s", ipv6Address, intf, out))
	}
	return nil
}

// This assumes a custom network layout where a single interface is added
// inside a given VRF. The function returns error if more than one interface
// is found.
func WithMaster(exec executor.Executor, master string) (string, error) {
	links, err := ipLink(exec)
	if err != nil {
		return "", err
	}
	res := ""
	for _, l := range links {
		if l.Master == master && res != "" {
			return "", fmt.Errorf("multiple interfaces with master %s found: %s, %s", master, l.Master, res)
		}
		if l.Master == master {
			res = l.Ifname
		}
	}
	if res == "" {
		return "", &NotFoundErr{interfaceName: res}
	}

	return res, nil
}

type Addresses struct {
	InterfaceName string
	IPV4Address   string
	IPV6Address   string
}

func AddressesForDevice(exec executor.Executor, dev string) (*Addresses, error) {
	jsonIPOutput, err := exec.Exec("ip", "-j", "addr", "show", "dev", dev)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve interface address %s, %w :%s", dev, err, jsonIPOutput)
	}

	var intf []interfaceAddress
	err = json.Unmarshal([]byte(jsonIPOutput), &intf)
	if err != nil {
		return nil, err
	}
	if len(intf) != 1 {
		return nil, fmt.Errorf("expected one single interface for %s, got %d", dev, len(intf))
	}
	res := Addresses{InterfaceName: dev}
	for _, addr := range intf[0].AddrInfo {
		if addr.Family == "inet6" && addr.Scope == "global" {
			res.IPV6Address = addr.Local
		}
		if addr.Family == "inet" && addr.Scope == "global" {
			res.IPV4Address = addr.Local
		}
	}

	return &res, nil
}

func LinkLocalAddressForDevice(exec executor.Executor, dev string) (string, error) {
	jsonIPOutput, err := exec.Exec("ip", "-j", "-6", "addr", "show", "dev", dev, "scope", "link")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve interface address %s, %w :%s", dev, err, jsonIPOutput)
	}

	var intf []interfaceAddress
	err = json.Unmarshal([]byte(jsonIPOutput), &intf)
	if err != nil {
		return "", err
	}
	if len(intf) != 1 {
		return "", fmt.Errorf("expected one single interface for %s, got %d", dev, len(intf))
	}
	for _, addr := range intf[0].AddrInfo {
		return addr.Local, nil
	}

	return "", nil
}

func findInterfaceWithAddresses(jsonIPOutput string, ipv4Address, ipv6Address string) (string, error) {
	var interfaces []interfaceAddress
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

func ipLink(exec executor.Executor) ([]Link, error) {
	links, err := exec.Exec("ip", "-j", "l")
	if err != nil {
		return nil, fmt.Errorf("failed to find links %w :%s", err, links)
	}
	var interfaces []Link
	err = json.Unmarshal([]byte(links), &interfaces)
	if err != nil {
		return nil, err
	}
	return interfaces, nil
}
