// SPDX-License-Identifier:Apache-2.0

package config

import (
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/e2etest/pkg/pointer"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const BGP = "bgp"
const L2 = "layer2"

func BFDProfileWithDefaults(profile metallbv1beta1.BFDProfile, multiHop bool) metallbv1beta1.BFDProfile {
	res := metallbv1beta1.BFDProfile{}
	res.Name = profile.Name
	res.Spec.ReceiveInterval = valueWithDefault(profile.Spec.ReceiveInterval, 300)
	res.Spec.TransmitInterval = valueWithDefault(profile.Spec.TransmitInterval, 300)
	res.Spec.DetectMultiplier = valueWithDefault(profile.Spec.DetectMultiplier, 3)
	res.Spec.EchoInterval = valueWithDefault(profile.Spec.EchoInterval, 50)
	res.Spec.MinimumTTL = valueWithDefault(profile.Spec.MinimumTTL, 254)
	res.Spec.EchoMode = profile.Spec.EchoMode
	res.Spec.PassiveMode = profile.Spec.PassiveMode

	if multiHop {
		res.Spec.EchoMode = pointer.BoolPtr(false)
		res.Spec.EchoInterval = pointer.Uint32Ptr(50)
	}

	return res
}

// IPAddressPoolToLegacy converts the given IPAddressPool to the legacy addresspool.
func IPAddressPoolToLegacy(ipAddressPool metallbv1beta1.IPAddressPool, protocol string, bgpAdv []metallbv1beta1.BGPAdvertisement) metallbv1beta1.AddressPool {
	res := metallbv1beta1.AddressPool{
		ObjectMeta: v1.ObjectMeta{
			Name: ipAddressPool.Name,
		},
		Spec: metallbv1beta1.AddressPoolSpec{
			Protocol:          protocol,
			Addresses:         make([]string, 0),
			AutoAssign:        ipAddressPool.Spec.AutoAssign,
			BGPAdvertisements: make([]metallbv1beta1.LegacyBgpAdvertisement, 0),
		},
	}
	res.Spec.Addresses = append(res.Spec.Addresses, ipAddressPool.Spec.Addresses...)

	for _, adv := range bgpAdv {
		legacy := metallbv1beta1.LegacyBgpAdvertisement{
			AggregationLength:   adv.Spec.AggregationLength,
			AggregationLengthV6: adv.Spec.AggregationLengthV6,
			LocalPref:           adv.Spec.LocalPref,
			Communities:         make([]string, 0),
		}
		legacy.Communities = append(legacy.Communities, adv.Spec.Communities...)
		res.Spec.BGPAdvertisements = append(res.Spec.BGPAdvertisements, legacy)
	}
	return res
}

func valueWithDefault(v *uint32, def uint32) *uint32 {
	if v != nil {
		return v
	}
	return &def
}
