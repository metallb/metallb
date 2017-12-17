package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
)

func hasGoBGP() bool {
	_, err := os.Stat("/gobgpd")
	return err == nil
}

func writeGoBGPConfig() error {
	cfg := fmt.Sprintf(`
[global.config]
  as = 64512
  router-id = "10.96.0.102"
  port = 2179

[[neighbors]]
  [neighbors.config]
    neighbor-address = "%s"
    peer-as = 64512
`, nodeIP())
	if err := ioutil.WriteFile("/gobgp.conf", []byte(cfg), 0644); err != nil {
		return err
	}
	return nil
}

func runGoBGP() error {
	return runOrCrash("/gobgpd", "-f", "/gobgp.conf")
}

func goBGPStatus() (*values, error) {
	proto, err := gobgp(fmt.Sprintf("neighbor %s", nodeIP()))
	if err != nil {
		return nil, err
	}
	routes, err := gobgp("global rib")
	if err != nil {
		return nil, err
	}

	var cidrs []*net.IPNet
	// Quick and dirty parser to extract the prefixes from the route
	// dump.
	for _, l := range strings.Split(routes, "\n") {
		fs := strings.Fields(l)
		if len(fs) < 2 {
			continue
		}
		_, n, err := net.ParseCIDR(fs[1])
		if err != nil {
			continue
		}
		cidrs = append(cidrs, n)
	}

	return &values{
		Name:           "GoBGP",
		Connected:      strings.Contains(proto, "established"),
		Prefixes:       cidrs,
		ProtocolStatus: proto,
		Routes:         routes,
	}, nil
}

func gobgp(cmd string) (string, error) {
	c := exec.Command("/gobgp", strings.Split(cmd, " ")...)
	bs, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %q: %s", cmd, err)
	}
	return string(bs), nil
}
