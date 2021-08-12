package e2e

import (
	"context"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// Proto holds the protocol we are speaking.
type Proto string

// MetalLB supported protocols.
const (
	BGP    Proto = "bgp"
	Layer2 Proto = "layer2"
)

// configFile is the configuration as parsed out of the ConfigMap,
// without validation or useful high level types.
type configFile struct {
	Peers          []peer            `yaml:"peers,omitempty"`
	BGPCommunities map[string]string `yaml:"bgp-communities,omitempty"`
	Pools          []addressPool     `yaml:"address-pools,omitempty"`
}

type peer struct {
	MyASN         uint32         `yaml:"my-asn,omitempty"`
	ASN           uint32         `yaml:"peer-asn,omitempty"`
	Addr          string         `yaml:"peer-address,omitempty"`
	SrcAddr       string         `yaml:"source-address,omitempty"`
	Port          uint16         `yaml:"peer-port,omitempty"`
	HoldTime      string         `yaml:"hold-time,omitempty"`
	RouterID      string         `yaml:"router-id,omitempty"`
	NodeSelectors []nodeSelector `yaml:"node-selectors,omitempty"`
	Password      string         `yaml:"password,omitempty"`
}

type nodeSelector struct {
	MatchLabels      map[string]string      `yaml:"match-labels,omitempty"`
	MatchExpressions []selectorRequirements `yaml:"match-expressions,omitempty"`
}

type selectorRequirements struct {
	Key      string   `yaml:"key,omitempty"`
	Operator string   `yaml:"operator,omitempty"`
	Values   []string `yaml:"values,omitempty"`
}

type addressPool struct {
	Protocol          Proto              `yaml:"protocol,omitempty"`
	Name              string             `yaml:"name,omitempty"`
	Addresses         []string           `yaml:"addresses,omitempty"`
	AvoidBuggyIPs     bool               `yaml:"avoid-buggy-ips,omitempty"`
	AutoAssign        *bool              `yaml:"auto-assign,omitempty"`
	BGPAdvertisements []bgpAdvertisement `yaml:"bgp-advertisements,omitempty"`
}

type bgpAdvertisement struct {
	AggregationLength *int     `yaml:"aggregation-length,omitempty"`
	LocalPref         *uint32  `yaml:"localpref,omitempty"`
	Communities       []string `yaml:"communities,omitempty"`
}

func updateConfigMap(cs clientset.Interface, data configFile) error {
	resData, err := yaml.Marshal(data)
	if err != nil {
		return errors.Wrapf(err, "Failed to marshal MetalLB ConfigMap data")
	}

	_, err = cs.CoreV1().ConfigMaps(testNameSpace).Update(context.TODO(), &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: testNameSpace,
		},
		Data: map[string]string{"config": string(resData)},
	}, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "Failed to update MetalLB ConfigMap")
	}

	return nil
}
