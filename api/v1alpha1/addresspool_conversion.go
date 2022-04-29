// SPDX-License-Identifier:Apache-2.0

package v1alpha1

import (
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/pointer"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AddressPool to the Hub version (vbeta1).
func (src *AddressPool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.AddressPool)
	dst.Spec.Protocol = src.Spec.Protocol
	dst.Spec.Addresses = make([]string, len(src.Spec.Addresses))
	copy(dst.Spec.Addresses, src.Spec.Addresses)
	if src.Spec.AutoAssign != nil {
		dst.Spec.AutoAssign = pointer.BoolPtr(*src.Spec.AutoAssign)
	}
	dst.Spec.BGPAdvertisements = make([]v1beta1.LegacyBgpAdvertisement, len(src.Spec.BGPAdvertisements))
	for i, adv := range src.Spec.BGPAdvertisements {
		if adv.AggregationLength != nil {
			dst.Spec.BGPAdvertisements[i].AggregationLength = pointer.Int32Ptr(*adv.AggregationLength)
		}
		if adv.AggregationLengthV6 != nil {
			dst.Spec.BGPAdvertisements[i].AggregationLengthV6 = pointer.Int32Ptr(*adv.AggregationLengthV6)
		}
		dst.Spec.BGPAdvertisements[i].LocalPref = adv.LocalPref
		dst.Spec.BGPAdvertisements[i].Communities = make([]string, len(adv.Communities))
		copy(dst.Spec.BGPAdvertisements[i].Communities, adv.Communities)
	}
	dst.ObjectMeta = src.ObjectMeta
	return nil
}

// ConvertFrom converts from the Hub version (vbeta1) to this version.
func (dst *AddressPool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.AddressPool)
	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.Protocol = src.Spec.Protocol
	dst.Spec.Addresses = make([]string, len(src.Spec.Addresses))
	copy(dst.Spec.Addresses, src.Spec.Addresses)
	if src.Spec.AutoAssign != nil {
		dst.Spec.AutoAssign = pointer.BoolPtr(*src.Spec.AutoAssign)
	}
	dst.Spec.BGPAdvertisements = make([]BgpAdvertisement, len(src.Spec.BGPAdvertisements))
	for i, adv := range src.Spec.BGPAdvertisements {
		if adv.AggregationLength != nil {
			dst.Spec.BGPAdvertisements[i].AggregationLength = pointer.Int32Ptr(*adv.AggregationLength)
		}
		if adv.AggregationLengthV6 != nil {
			dst.Spec.BGPAdvertisements[i].AggregationLengthV6 = pointer.Int32Ptr(*adv.AggregationLengthV6)
		}
		dst.Spec.BGPAdvertisements[i].LocalPref = adv.LocalPref
		dst.Spec.BGPAdvertisements[i].Communities = make([]string, len(adv.Communities))
		copy(dst.Spec.BGPAdvertisements[i].Communities, adv.Communities)
	}
	return nil
}
