// SPDX-License-Identifier:Apache-2.0

package config

import (
	"bytes"
	"fmt"
	"text/template"
)

const unnumberedPeerFRRConfigTempl = `
frr defaults datacenter
hostname {{.Hostname}}
log file /tmp/frr.log debugging
log timestamp precision 3
interface {{.Interface}}
 no ipv6 nd suppress-ra
 ipv6 nd ra-lifetime 0
exit
router bgp {{.ASNLocal}}
 bgp router-id {{.RouterID}}
 no bgp network import-check
 no bgp ebgp-requires-policy
 neighbor MTLB peer-group
 neighbor MTLB passive
 neighbor MTLB remote-as {{.ASNRemote}}
 neighbor MTLB description LEAF-MTLB
 neighbor MTLB bfd
 neighbor {{.Interface}} interface peer-group MTLB
 neighbor {{.Interface}} description k8s-node
 address-family ipv4 unicast
{{- range .ToAdvertiseV4 }}
  network {{.}}
{{- end }}
  neighbor MTLB activate
 exit-address-family
address-family ipv6 unicast
{{- range .ToAdvertiseV6 }}
  network {{.}}
{{- end }}
  neighbor MTLB activate
 exit-address-family
exit`

type RouterConfigUnnumbered struct {
	ASNLocal      uint32
	ASNRemote     uint32
	Hostname      string
	Interface     string
	RouterID      string
	ToAdvertiseV4 []string
	ToAdvertiseV6 []string
}

func (rc RouterConfigUnnumbered) Config() (string, error) {
	t, err := template.New("UnnumberedBGPTemplate").Parse(unnumberedPeerFRRConfigTempl)
	if err != nil {
		return "", fmt.Errorf("Failed to create template %w", err)
	}
	var b bytes.Buffer
	err = t.Execute(&b, rc)
	if err != nil {
		return "", fmt.Errorf("failed to update template %w", err)
	}

	return b.String(), nil
}
