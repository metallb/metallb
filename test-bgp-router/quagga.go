package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"strings"
)

const zebraConfig = `
hostname router
password zebra
enable password zebra
log stdout
`

func writeQuaggaConfig() error {
	if err := ioutil.WriteFile("/etc/quagga/zebra.conf", []byte(zebraConfig), 0644); err != nil {
		return err
	}

	bgpdConfig := fmt.Sprintf(`
router bgp 64512
 bgp router-id 10.0.0.100
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
	if err := runOrCrash("/usr/sbin/bgpd", "-p", "1179", "-A", "127.0.0.1", "-f", "/etc/quagga/bgpd.conf"); err != nil {
		return err
	}
	return nil
}

func quaggaStatus(w http.ResponseWriter, r *http.Request) {
	proto, err := quagga("show ip bgp neighbors")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	routes, err := quagga("show ip route bgp")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summary, err := quagga("show ip route bgp")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	renderStatus(w, &values{
		Connected:      strings.Contains(proto, "Established"),
		Prefixes:       cidrs,
		ProtocolStatus: proto,
		Routes:         routes,
	})
}

func quagga(cmd string) (string, error) {
	c := exec.Command("/usr/bin/vtysh", "-c", cmd)
	bs, err := c.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
