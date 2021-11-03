// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/pkg/errors"
)

type Neighbor struct {
	ip        net.IP
	connected bool
	localAS   string
	remoteAS  string
}

type Route struct {
	Destination *net.IPNet
	NextHops    []net.IP
}

const bgpConnected = "Established"

type FRRNeighbor struct {
	RemoteAs   int    `json:"remoteAs"`
	LocalAs    int    `json:"localAs"`
	BgpVersion int    `json:"bgpVersion"`
	BgpState   string `json:"bgpState"`
}

type IPInfo struct {
	Routes map[string][]FRRRoute `json:"routes"`
}

type FRRRoute struct {
	Valid    bool   `json:"valid"`
	PeerID   string `json:"peerId"`
	Nexthops []struct {
		IP string `json:"ip"`
	} `json:"nexthops"`
}

// parseNeighbour takes the result of a show bgp neighbor x.y.w.z
// and parses the informations related to the neighbour.
func parseNeighbour(vtyshRes string) (*Neighbor, error) {
	res := map[string]FRRNeighbor{}
	err := json.Unmarshal([]byte(vtyshRes), &res)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse vtysh response")
	}
	if len(res) > 1 {
		return nil, errors.New("more than one peer were returned")
	}
	if len(res) == 0 {
		return nil, errors.New("no peers were returned")
	}
	for k, n := range res {
		ip := net.ParseIP(k)
		if ip == nil {
			return nil, fmt.Errorf("failed to parse %s as ip", ip)
		}
		connected := true
		if n.BgpState != bgpConnected {
			connected = false
		}
		return &Neighbor{
			ip:        ip,
			connected: connected,
			localAS:   strconv.Itoa(n.LocalAs),
			remoteAS:  strconv.Itoa(n.RemoteAs),
		}, nil
	}
	return nil, errors.New("no peers were returned")
}

// parseNeighbour takes the result of a show bgp neighbor
// and parses the informations related to all the neighbours.
func parseNeighbours(vtyshRes string) ([]*Neighbor, error) {
	toParse := map[string]FRRNeighbor{}
	err := json.Unmarshal([]byte(vtyshRes), &toParse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse vtysh response")
	}

	res := make([]*Neighbor, 0)
	for k, n := range toParse {
		ip := net.ParseIP(k)
		if ip == nil {
			return nil, fmt.Errorf("failed to parse %s as ip", ip)
		}
		connected := true
		if n.BgpState != bgpConnected {
			connected = false
		}
		res = append(res, &Neighbor{
			ip:        ip,
			connected: connected,
			localAS:   strconv.Itoa(n.LocalAs),
			remoteAS:  strconv.Itoa(n.RemoteAs),
		})
	}
	return res, nil
}

// parseRoute takes the result of a show bgp neighbor
// and parses the informations related to all the neighbours.
func parseRoutes(vtyshRes string) (map[string]Route, error) {
	toParse := IPInfo{}
	err := json.Unmarshal([]byte(vtyshRes), &toParse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse vtysh response")
	}

	res := make(map[string]Route)
	for k, frrRoutes := range toParse.Routes {
		destIP, dest, err := net.ParseCIDR(k)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse cidr for %s", k)
		}

		r := Route{
			Destination: dest,
			NextHops:    make([]net.IP, 0),
		}
		for _, n := range frrRoutes {
			for _, h := range n.Nexthops {
				ip := net.ParseIP(h.IP)
				if ip == nil {
					return nil, fmt.Errorf("failed to parse ip %s", h.IP)
				}
				r.NextHops = append(r.NextHops, ip)
			}
		}
		res[destIP.String()] = r
	}
	return res, nil
}

// NeighborConnected tells if the neighbor in the given
// json format is connected.
func NeighborConnected(neighborJson string) (bool, error) {
	n, err := parseNeighbour(neighborJson)
	if err != nil {
		return false, err
	}
	return n.connected, nil
}
