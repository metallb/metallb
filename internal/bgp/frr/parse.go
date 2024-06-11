// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"

	"errors"
)

type Neighbor struct {
	IP             net.IP
	VRF            string
	Connected      bool
	LocalAS        string
	RemoteAS       string
	PrefixSent     int
	Port           int
	RemoteRouterID string
	MsgStats       MessageStats
}

type Route struct {
	Destination *net.IPNet
	NextHops    []net.IP
	LocalPref   uint32
	Origin      string
}

const bgpConnected = "Established"

type FRRNeighbor struct {
	RemoteAs          int          `json:"remoteAs"`
	LocalAs           int          `json:"localAs"`
	RemoteRouterID    string       `json:"remoteRouterId"`
	BgpVersion        int          `json:"bgpVersion"`
	BgpState          string       `json:"bgpState"`
	PortForeign       int          `json:"portForeign"`
	MsgStats          MessageStats `json:"messageStats"`
	VRFName           string       `json:"vrf"`
	AddressFamilyInfo map[string]struct {
		SentPrefixCounter int `json:"sentPrefixCounter"`
	} `json:"addressFamilyInfo"`
}

type MessageStats struct {
	OpensSent          int `json:"opensSent"`
	OpensReceived      int `json:"opensRecv"`
	NotificationsSent  int `json:"notificationsSent"`
	UpdatesSent        int `json:"updatesSent"`
	UpdatesReceived    int `json:"updatesRecv"`
	KeepalivesSent     int `json:"keepalivesSent"`
	KeepalivesReceived int `json:"keepalivesRecv"`
	RouteRefreshSent   int `json:"routeRefreshSent"`
	TotalSent          int `json:"totalSent"`
	TotalReceived      int `json:"totalRecv"`
}

type IPInfo struct {
	Routes map[string][]FRRRoute `json:"routes"`
}

type FRRRoute struct {
	Valid     bool   `json:"valid"`
	PeerID    string `json:"peerId"`
	LocalPref uint32 `json:"locPrf"`
	Origin    string `json:"origin"`
	Nexthops  []struct {
		IP    string `json:"ip"`
		Scope string `json:"scope"`
	} `json:"nexthops"`
}

type BFDPeer struct {
	Multihop                  bool   `json:"multihop"`
	Peer                      string `json:"peer"`
	Local                     string `json:"local"`
	Vrf                       string `json:"vrf"`
	Interface                 string `json:"interface"`
	ID                        int    `json:"id"`
	RemoteID                  int64  `json:"remote-id"`
	PassiveMode               bool   `json:"passive-mode"`
	Status                    string `json:"status"`
	Uptime                    int    `json:"uptime"`
	Diagnostic                string `json:"diagnostic"`
	RemoteDiagnostic          string `json:"remote-diagnostic"`
	ReceiveInterval           int    `json:"receive-interval"`
	TransmitInterval          int    `json:"transmit-interval"`
	EchoReceiveInterval       int    `json:"echo-receive-interval"`
	EchoTransmitInterval      int    `json:"echo-transmit-interval"`
	DetectMultiplier          int    `json:"detect-multiplier"`
	RemoteReceiveInterval     int    `json:"remote-receive-interval"`
	RemoteTransmitInterval    int    `json:"remote-transmit-interval"`
	RemoteEchoInterval        int    `json:"remote-echo-interval"`
	RemoteEchoReceiveInterval int    `json:"remote-echo-receive-interval"`
	RemoteDetectMultiplier    int    `json:"remote-detect-multiplier"`
}

// parseNeighbour takes the result of a show bgp neighbor x.y.w.z
// and parses the informations related to the neighbour.
func ParseNeighbour(vtyshRes string) (*Neighbor, error) {
	res := map[string]FRRNeighbor{}
	err := json.Unmarshal([]byte(vtyshRes), &res)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to parse vtysh response"))
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
		prefixSent := 0
		for _, s := range n.AddressFamilyInfo {
			prefixSent += s.SentPrefixCounter
		}
		return &Neighbor{
			IP:             ip,
			Connected:      connected,
			LocalAS:        strconv.Itoa(n.LocalAs),
			RemoteAS:       strconv.Itoa(n.RemoteAs),
			PrefixSent:     prefixSent,
			Port:           n.PortForeign,
			RemoteRouterID: n.RemoteRouterID,
			MsgStats:       n.MsgStats,
		}, nil
	}
	return nil, errors.New("no peers were returned")
}

// parseNeighbour takes the result of a show bgp neighbor
// and parses the informations related to all the neighbours.
func ParseNeighbours(vtyshRes string) ([]*Neighbor, error) {
	toParse := map[string]FRRNeighbor{}
	err := json.Unmarshal([]byte(vtyshRes), &toParse)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to parse vtysh response"))
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
		prefixSent := 0
		for _, s := range n.AddressFamilyInfo {
			prefixSent += s.SentPrefixCounter
		}
		res = append(res, &Neighbor{
			IP:             ip,
			Connected:      connected,
			LocalAS:        strconv.Itoa(n.LocalAs),
			RemoteAS:       strconv.Itoa(n.RemoteAs),
			PrefixSent:     prefixSent,
			Port:           n.PortForeign,
			RemoteRouterID: n.RemoteRouterID,
			MsgStats:       n.MsgStats,
		})
	}
	return res, nil
}

// parseRoute takes the result of a show bgp neighbor
// and parses the informations related to all the neighbours.
func ParseRoutes(vtyshRes string) (map[string]Route, error) {
	toParse := IPInfo{}
	err := json.Unmarshal([]byte(vtyshRes), &toParse)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to parse vtysh response"))
	}

	res := make(map[string]Route)
	for k, frrRoutes := range toParse.Routes {
		destIP, dest, err := net.ParseCIDR(k)
		if err != nil {
			return nil, errors.Join(err, fmt.Errorf("failed to parse cidr for %s", k))
		}

		r := Route{
			Destination: dest,
			NextHops:    make([]net.IP, 0),
		}
		for _, n := range frrRoutes {
			r.LocalPref = n.LocalPref
			r.Origin = n.Origin
		out:
			for _, h := range n.Nexthops {
				ip := net.ParseIP(h.IP)
				if ip == nil {
					return nil, fmt.Errorf("failed to parse ip %s", h.IP)
				}
				if ip.To4() == nil && h.Scope == "link-local" {
					continue
				}
				for _, current := range r.NextHops {
					if ip.Equal(current) {
						continue out
					}
				}
				r.NextHops = append(r.NextHops, ip)
			}
		}
		res[destIP.String()] = r
	}
	return res, nil
}

func ParseBFDPeers(vtyshRes string) ([]BFDPeer, error) {
	parseRes := []BFDPeer{}
	err := json.Unmarshal([]byte(vtyshRes), &parseRes)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to parse vtysh response"))
	}
	return parseRes, nil
}

func ParseVRFs(vtyshRes string) ([]string, error) {
	vrfs := map[string]interface{}{}
	err := json.Unmarshal([]byte(vtyshRes), &vrfs)
	if err != nil {
		return nil, errors.Join(err, errors.New("parseVRFs: failed to parse vtysh response"))
	}
	res := make([]string, 0)
	for v := range vrfs {
		res = append(res, v)
	}
	sort.Strings(res)
	return res, nil
}
