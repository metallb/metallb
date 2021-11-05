// SPDX-License-Identifier:Apache-2.0

package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/template"

	"github.com/pkg/errors"
	consts "go.universe.tf/metallb/e2etest/pkg/frr/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// BGP router config.
const bgpConfigTemplate = `
hostname bgpd
#password zebra

route-map RMAP permit 10
set ipv6 next-hop prefer-global
router bgp {{.ASN}}
  no bgp ebgp-requires-policy
  no bgp default ipv4-unicast
{{range .Neighbors }}
  neighbor {{.Addr}} remote-as {{.ASN}}
  {{ if .Password -}}
  neighbor {{.Addr}} password {{.Password}}
  {{- end }}
{{- end }}
  address-family {{.IPFamily}} unicast
{{range .Neighbors }}
    neighbor {{.Addr}} next-hop-self
    neighbor {{.Addr}} activate
    {{ if eq .IPFamily "ipv6" -}}
    neighbor {{.Addr}} route-map RMAP in
    {{- end }}
{{- end }}
  exit-address-family

log stdout debugging
`

type RouterConfig struct {
	ASN       uint32
	Neighbors []*NeighborConfig
	BGPPort   uint16
	Password  string
	IPFamily  string
}

type NeighborConfig struct {
	ASN      uint32
	Addr     string
	Password string
	IPFamily string
}

// Set the IP of each node in the cluster in the BGP router configuration.
// Each node will peer with the BGP router.
func BGPPeersForAllNodes(cs clientset.Interface, nc NeighborConfig, rc RouterConfig) (string, error) {
	router := rc

	router.Neighbors = make([]*NeighborConfig, 0)

	nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "Failed to get cluster nodes")
	}

	// TODO: The IP address handling will need updates to add support for dual stack
	for _, node := range nodes.Items {
		for i := range node.Status.Addresses {
			if node.Status.Addresses[i].Type == "InternalIP" {
				neighbor := nc
				neighbor.Addr = node.Status.Addresses[i].Address
				router.Neighbors = append(router.Neighbors, &neighbor)
			}
		}
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
