// SPDX-License-Identifier:Apache-2.0

package config

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"text/template"

	"github.com/pkg/errors"
	consts "go.universe.tf/metallb/e2etest/pkg/frr/consts"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/internal/ipfamily"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

const Empty = `password zebra
log stdout debugging
log file /tmp/frr.log debugging`

// BGP router config.
const bgpConfigTemplate = `
password zebra

debug bgp updates
debug bgp neighbor
debug zebra nht
debug bgp nht
debug bfd peer
ip nht resolve-via-default
ipv6 nht resolve-via-default

log file /tmp/frr.log debugging
log timestamp precision 3
route-map RMAP permit 10
set ipv6 next-hop prefer-global
{{$ROUTERASN:=.ASN}}
router bgp {{$ROUTERASN}}
  bgp router-id {{.RouterID}}
  no bgp network import-check
  no bgp ebgp-requires-policy
  no bgp default ipv4-unicast
{{range .Neighbors }}
  neighbor {{.Addr}} remote-as {{.ASN}}
  {{- if and (ne .ASN $ROUTERASN) (.MultiHop) }}
  neighbor {{.Addr}} ebgp-multihop
  {{- end }}
  {{ if .Password -}}
  neighbor {{.Addr}} password {{.Password}}
  {{- end }}
{{- if .BFDEnabled }} 
  neighbor {{.Addr}} bfd
{{- end -}}
{{- end }}
{{- if ne (len .V4Neighbors) 0}}
  address-family ipv4 unicast
{{range .V4Neighbors }}
    neighbor {{.Addr}} next-hop-self
    neighbor {{.Addr}} activate
    {{- if .ToAdvertise}}
    network {{.ToAdvertise}}
    {{- end }}
{{- end }}
  exit-address-family
{{- end }}
{{- if ne (len .V6Neighbors) 0}}
  address-family ipv6 unicast
{{range .V6Neighbors }}
    neighbor {{.Addr}} next-hop-self
    neighbor {{.Addr}} activate
    neighbor {{.Addr}} route-map RMAP in
    {{- if .ToAdvertise}}
    network {{.ToAdvertise}}
    {{- end }}
{{- end }}
exit-address-family
{{- end }}

`

type RouterConfig struct {
	RouterID    string
	ASN         uint32
	Neighbors   []*NeighborConfig
	V4Neighbors []*NeighborConfig
	V6Neighbors []*NeighborConfig
	BGPPort     uint16
	Password    string
}

type NeighborConfig struct {
	ASN         uint32
	Addr        string
	Password    string
	IPFamily    ipfamily.Family
	BFDEnabled  bool
	ToAdvertise string
	MultiHop    bool
}

// Set the IP of each node in the cluster in the BGP router configuration.
// Each node will peer with the BGP router.
func BGPPeersForAllNodes(cs clientset.Interface, nc NeighborConfig, rc RouterConfig, ipFamily ipfamily.Family) (string, error) {
	router := rc

	router.V4Neighbors = make([]*NeighborConfig, 0)
	router.V6Neighbors = make([]*NeighborConfig, 0)
	router.Neighbors = make([]*NeighborConfig, 0)

	nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "Failed to get cluster nodes")
	}

	ips := k8s.NodeIPsForFamily(nodes.Items, ipFamily)
	for _, ip := range ips {
		neighbor := nc
		neighbor.Addr = ip
		nc.IPFamily = ipfamily.ForAddress(net.ParseIP(ip))

		if nc.IPFamily == ipfamily.IPv4 {
			router.V4Neighbors = append(router.V4Neighbors, &neighbor)
		} else {
			router.V6Neighbors = append(router.V6Neighbors, &neighbor)
		}
		router.Neighbors = append(router.Neighbors, &neighbor)
	}

	t, err := template.New("bgp Config Template").Parse(bgpConfigTemplate)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to create bgp template")
	}

	var b bytes.Buffer
	err = t.Execute(&b, router)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to update bgp template")
	}

	return b.String(), nil
}

// Set BGP configuration file in the test directory.
func SetBGPConfig(testDirName string, config string) error {
	path := fmt.Sprintf("%s/%s", testDirName, consts.BGPConfigFile)
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "Failed to open file %s", path)
	}
	defer f.Close()

	_, err = f.WriteString(config)
	if err != nil {
		return errors.Wrapf(err, "Failed to write to file %s", path)
	}
	return nil
}

// Set daemons config file.
func SetDaemonsConfig(testDirName string, rc RouterConfig) error {
	path := fmt.Sprintf("%s/%s", testDirName, consts.DaemonsConfigFile)
	tpl, err := template.ParseFiles(path)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse %s", path)
	}

	f, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "Failed to open file %s", path)
	}

	defer f.Close()

	type Template struct {
		BGPPort uint16
	}

	info := Template{
		BGPPort: rc.BGPPort,
	}

	err = tpl.Execute(f, info)
	if err != nil {
		return errors.Wrapf(err, "Failed to update %s", path)
	}

	return nil
}
