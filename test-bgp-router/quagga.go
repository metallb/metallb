package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
)

const zebraConfig = `
hostname router
password zebra
enable password zebra
log stdout
`

func hasQuagga() bool {
	_, err := os.Stat("/usr/sbin/bgpd")
	return err == nil
}

func writeQuaggaConfig() error {
	if err := ioutil.WriteFile("/etc/quagga/zebra.conf", []byte(zebraConfig), 0644); err != nil {
		return err
	}

	bgpdConfig := fmt.Sprintf(`
router bgp 64512
 bgp router-id 10.96.0.101
 neighbor %s remote-as 64512
 neighbor %s passive
!
`, nodeIP(), nodeIP())
	if err := ioutil.WriteFile("/etc/quagga/bgpd.conf", []byte(bgpdConfig), 0644); err != nil {
		return err
	}
	return nil
}

func runQuagga() error {
	if err := runOrCrash("/usr/sbin/zebra", "-A", "127.0.0.1", "-f", "/etc/quagga/zebra.conf"); err != nil {
		return err
	}
	if err := runOrCrash("/usr/sbin/bgpd", "-A", "127.0.0.1", "-f", "/etc/quagga/bgpd.conf"); err != nil {
		return err
	}
	return nil
}

func quaggaStatus() (*values, error) {
	proto, err := quagga("show ip bgp neighbors")
	if err != nil {
		return nil, err
	}
	routes, err := quagga("show ip route bgp")
	if err != nil {
		return nil, err
	}

	summary, err := quagga("show ip route bgp")
	if err != nil {
		return nil, err
	}

	var cidrs []*net.IPNet
	// Quick and dirty parser to extract the prefixes from the route
	// dump.
	for _, l := range strings.Split(summary, "\n") {
		fs := strings.Split(l, " ")
		if len(fs) < 3 {
			continue
		}
		_, n, err := net.ParseCIDR(fs[2])
		if err != nil {
			continue
		}
		cidrs = append(cidrs, n)
	}

	return &values{
		Name:           "Quagga",
		Connected:      strings.Contains(proto, "Established"),
		Prefixes:       cidrs,
		ProtocolStatus: proto,
		Routes:         routes,
	}, nil
}

func quagga(cmd string) (string, error) {
	c := exec.Command("/usr/bin/vtysh", "-c", cmd)
	bs, err := c.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
