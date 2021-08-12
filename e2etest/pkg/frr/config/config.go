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
router bgp {{.ASN}}
{{range .Neighbors }}
  neighbor {{.Addr}} remote-as {{.ASN}}
  neighbor {{.Addr}} next-hop-self
{{- end }}
`

type routerConfig struct {
	ASN       uint32
	Neighbors []*neighborConfig
}

type neighborConfig struct {
	ASN  uint32
	Addr string
}

// Set the IP of each node in the cluster in the BGP router configuration.
// Each node will peer with the BGP router.
func BGPPeersForAllNodes(cs clientset.Interface) (string, error) {
	router := &routerConfig{
		ASN: 64512,
	}

	router.Neighbors = make([]*neighborConfig, 0)

	nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "Failed to get cluster nodes")
	}

	// TODO: The IP address handling will need updates to add support for IPv6.
	for _, node := range nodes.Items {
		for i := range node.Status.Addresses {
			if node.Status.Addresses[i].Type == "InternalIP" {
				neighbor := &neighborConfig{
					ASN:  64512,
					Addr: node.Status.Addresses[i].Address,
				}
				router.Neighbors = append(router.Neighbors, neighbor)
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
		BGPConfig string
	}

	info := Template{
		BGPConfig: config,
	}

	err = tpl.Execute(f, info)
	if err != nil {
		return errors.Wrapf(err, "Failed to update %s", path)
	}

	return nil
}
