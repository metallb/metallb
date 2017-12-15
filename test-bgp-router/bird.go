package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"strings"
)

func writeBirdConfig() error {
	cfg := fmt.Sprintf(`
router id 10.96.0.100;
listen bgp port 1179;
log stderr all;
debug protocols all;
protocol device {
}
protocol static {
  route %s/32 via "eth0";
}
protocol bgp minikube {
  local 10.96.0.100 as 64512;
  neighbor %s as 64512;
  passive;
  error wait time 1, 2;
}
`, nodeIP(), nodeIP())
	if err := ioutil.WriteFile("/etc/bird.conf", []byte(cfg), 0644); err != nil {
		return err
	}
	return nil
}

func runBird() error {
	return runOrCrash("/usr/sbin/bird", "-d", "-c", "/etc/bird.conf")
}

func birdStatus() (*values, error) {
	proto, err := bird("show protocol all minikube")
	if err != nil {
		return nil, err
	}
	routes, err := bird("show route all protocol minikube")
	if err != nil {
		return nil, err
	}

	summary, err := bird("show route protocol minikube")
	if err != nil {
		return nil, err
	}

	var cidrs []*net.IPNet
	// Quick and dirty parser to extract the prefixes from the route
	// dump.
	for _, l := range strings.Split(summary, "\n") {
		fs := strings.Split(l, " ")
		if len(fs) < 1 {
			continue
		}
		_, n, err := net.ParseCIDR(fs[0])
		if err != nil {
			continue
		}
		cidrs = append(cidrs, n)
	}

	return &values{
		Name:           "BIRD",
		Connected:      strings.Contains(proto, "Established"),
		Prefixes:       cidrs,
		ProtocolStatus: proto,
		Routes:         routes,
	}, nil
}

func bird(cmd string) (string, error) {
	c := exec.Command("/usr/sbin/birdc", strings.Split(cmd, " ")...)
	bs, err := c.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
