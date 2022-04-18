// SPDX-License-Identifier:Apache-2.0

package v1alpha1

import (
	"reflect"
	"testing"

	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetalLBTestNameSpace = "metallb-test-namespace"
)

func TestValidateAddressPoolConvertTo(t *testing.T) {
	var err error
	var resAddressPool v1beta1.AddressPool

	convertAddressPool := AddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addresspool",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: AddressPoolSpec{
			Addresses: []string{
				"1.1.1.1-1.1.1.100",
			},

			Protocol:   "bgp",
			AutoAssign: pointer.BoolPtr(false),
			BGPAdvertisements: []BgpAdvertisement{
				{
					AggregationLength:   pointer.Int32Ptr(32),
					AggregationLengthV6: pointer.Int32Ptr(128),
					LocalPref:           uint32(100),
					Communities:         []string{"1234"},
				},
			},
		},
	}

	expectedAddressPool := v1beta1.AddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addresspool",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: v1beta1.AddressPoolSpec{
			Addresses: []string{
				"1.1.1.1-1.1.1.100",
			},

			Protocol:   "bgp",
			AutoAssign: pointer.BoolPtr(false),
			BGPAdvertisements: []v1beta1.LegacyBgpAdvertisement{
				{
					AggregationLength:   pointer.Int32Ptr(32),
					AggregationLengthV6: pointer.Int32Ptr(128),
					LocalPref:           uint32(100),
					Communities:         []string{"1234"},
				},
			},
		},
	}

	err = convertAddressPool.ConvertTo(&resAddressPool)
	if err != nil {
		t.Fatalf("failed converting AddressPool to v1beta1 version: %s", err)
	}

	if !reflect.DeepEqual(resAddressPool, expectedAddressPool) {
		t.Fatalf("expected AddressPool different than converted: %s", err)
	}
}

func TestValidateAddressPoolConvertFrom(t *testing.T) {
	var err error
	var resAddressPool AddressPool

	convertAddressPool := v1beta1.AddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addresspool",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: v1beta1.AddressPoolSpec{
			Addresses: []string{
				"1.1.1.1-1.1.1.100",
			},

			Protocol:   "bgp",
			AutoAssign: pointer.BoolPtr(false),
			BGPAdvertisements: []v1beta1.LegacyBgpAdvertisement{
				{
					AggregationLength:   pointer.Int32Ptr(32),
					AggregationLengthV6: pointer.Int32Ptr(128),
					LocalPref:           uint32(100),
					Communities:         []string{"1234"},
				},
			},
		},
	}

	expectedAddressPool := AddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addresspool",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: AddressPoolSpec{
			Addresses: []string{
				"1.1.1.1-1.1.1.100",
			},

			Protocol:   "bgp",
			AutoAssign: pointer.BoolPtr(false),
			BGPAdvertisements: []BgpAdvertisement{
				{
					AggregationLength:   pointer.Int32Ptr(32),
					AggregationLengthV6: pointer.Int32Ptr(128),
					LocalPref:           uint32(100),
					Communities:         []string{"1234"},
				},
			},
		},
	}

	err = resAddressPool.ConvertFrom(&convertAddressPool)
	if err != nil {
		t.Fatalf("failed converting v1beta1 AddressPool: %s", err)
	}

	if !reflect.DeepEqual(resAddressPool, expectedAddressPool) {
		t.Fatalf("expected AddressPool different than converted: %s", err)
	}
}
