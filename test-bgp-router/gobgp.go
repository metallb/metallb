package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/server"
)

func runGoBGP() error {
	s := server.NewBgpServer()
	go s.Serve()
	g := api.NewGrpcServer(s, ":50051")
	go g.Serve()
	global := &config.Global{
		Config: config.GlobalConfig{
			As:       64512,
			RouterId: "10.96.0.102",
			Port:     2179,
		},
	}

	if err := s.Start(global); err != nil {
		return err
	}

	// neighbor configuration
	n := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: nodeIP(),
			PeerAs:          64512,
		},
	}

	if err := s.AddNeighbor(n); err != nil {
		return err
	}

	return nil
}

func goBGPStatus() (*values, error) {
	proto, err := gobgp("neighbor", nodeIP())
	if err != nil {
		return nil, err
	}
	routes, err := gobgp("global", "rib")
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

func gobgp(args ...string) (string, error) {
	c := exec.Command(os.Args[0], append([]string{"gobgp"}, args...)...)
	bs, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %q: %s\n%s", strings.Join(args, " "), err, string(bs))
	}
	return string(bs), nil
}
