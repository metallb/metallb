// SPDX-License-Identifier:Apache-2.0

package config

import (
	"testing"

	"go.universe.tf/metallb/api/v1beta2"
)

func TestValidator(t *testing.T) {
	v := validator{DontValidate}

	bgpPeerList := v1beta2.BGPPeerList{
		Items: []v1beta2.BGPPeer{
			{
				Spec: v1beta2.BGPPeerSpec{
					MyASN:      42,
					ASN:        42,
					Address:    "1.2.3.4",
					BFDProfile: "default",
				},
			},
		},
	}
	err := v.Validate(&bgpPeerList)
	if err != nil {
		t.Error("The validator should not fail for non existing bfd profile")
	}
}
