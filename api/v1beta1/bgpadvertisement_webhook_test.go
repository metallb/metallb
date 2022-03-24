// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestValidateBGPAdvertisement(t *testing.T) {
	autoAssign := false
	ipPool := IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ipPool",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: IPPoolSpec{
			Addresses: []string{
				"10.20.30.40/24",
				"1.2.3.0/28",
			},
			AutoAssign: &autoAssign,
		},
	}
	ipPoolList := &IPPoolList{}
	ipPoolList.Items = append(ipPoolList.Items, ipPool)

	bgpAdv := BGPAdvertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bgpAdv",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: BGPAdvertisementSpec{
			AggregationLength: pointer.Int32Ptr(32),
			LocalPref:         uint32(100),
			Communities:       []string{"1234:2345"},
			IPPools:           []string{"test-ipPool"},
		},
	}
	bgpAdvList := &BGPAdvertisementList{}
	bgpAdvList.Items = append(bgpAdvList.Items, bgpAdv)

	tests := []struct {
		desc          string
		bgpAdv        *BGPAdvertisement
		isNewBGPAdv   bool
		expectedError string
	}{
		{
			desc: "No name",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPAdvertisementSpec{
					AggregationLength: pointer.Int32Ptr(32),
					LocalPref:         uint32(100),
					Communities:       []string{"1234:2345"},
					IPPools:           []string{"test-ipPool"},
				},
			},
			isNewBGPAdv:   true,
			expectedError: "Missing BGPAdvertisement name",
		},
		{
			desc: "Duplicate communities in BGPAdvertisement",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpAdv",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPAdvertisementSpec{
					AggregationLength: pointer.Int32Ptr(32),
					LocalPref:         uint32(100),
					Communities:       []string{"1234:2345", "1234:2345"},
					IPPools:           []string{"test-ipPool"},
				},
			},
			isNewBGPAdv:   true,
			expectedError: "duplicate definition of communities",
		},
		{
			desc: "Bad IPv4 aggregation length in bgp advertisment (too long)",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpAdv",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPAdvertisementSpec{
					AggregationLength: pointer.Int32Ptr(33),
					LocalPref:         uint32(100),
					Communities:       []string{"1234:2345"},
					IPPools:           []string{"test-ipPool"},
				},
			},
			isNewBGPAdv:   true,
			expectedError: "invalid aggregation length 33 for IPv4",
		},
		{
			desc: "Bad IPv6 aggregation length in bgp advertisment (too long)",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpAdv",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPAdvertisementSpec{
					AggregationLengthV6: pointer.Int32Ptr(129),
					LocalPref:           uint32(100),
					Communities:         []string{"1234:2345"},
					IPPools:             []string{"test-ipPool"},
				},
			},
			isNewBGPAdv:   true,
			expectedError: "invalid aggregation length 129 for IPv6",
		},
		{
			desc: "Bad IPv4 aggregation length in bgp advertisment (incompatible with addresses)",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpAdv",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPAdvertisementSpec{
					AggregationLength: pointer.Int32Ptr(26),
					IPPools:           []string{"test-ipPool"},
				},
			},
			isNewBGPAdv:   true,
			expectedError: "invalid aggregation length 26: prefix 28 in this pool is more specific than the aggregation length for addresses 1.2.3.0/28",
		},
		{
			desc: "Bad community literal in bgp advertisment (wrong format)",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpAdv",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPAdvertisementSpec{
					AggregationLength: pointer.Int32Ptr(32),
					LocalPref:         uint32(100),
					Communities:       []string{"65535"},
					IPPools:           []string{"test-ipPool"},
				},
			},
			isNewBGPAdv:   true,
			expectedError: "invalid community string \"65535\"",
		},
		{
			desc: "Bad community literal in bgp advertisment (asn part doesn't fit)",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpAdv",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPAdvertisementSpec{
					AggregationLength: pointer.Int32Ptr(32),
					LocalPref:         uint32(100),
					Communities:       []string{"99999999:1"},
					IPPools:           []string{"test-ipPool"},
				},
			},
			isNewBGPAdv:   true,
			expectedError: "invalid first section of community \"99999999\"",
		},
		{
			desc: "Bad community literal in bgp advertisment (community# part doesn't fit)",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpAdv",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPAdvertisementSpec{
					AggregationLength: pointer.Int32Ptr(32),
					LocalPref:         uint32(100),
					Communities:       []string{"1:99999999"},
					IPPools:           []string{"test-ipPool"},
				},
			},
			isNewBGPAdv:   true,
			expectedError: "invalid second section of community \"99999999\"",
		},
	}

	for _, test := range tests {
		err := test.bgpAdv.ValidateBGPAdv(test.isNewBGPAdv, bgpAdvList.Items, ipPoolList.Items)
		if err == nil {
			t.Errorf("%s: ValidateBGPAdv failed, no error found while expected: \"%s\"", test.desc, test.expectedError)
		} else {
			if !strings.Contains(fmt.Sprint(err), test.expectedError) {
				t.Errorf("%s: ValidateBGPAdv failed, expected error: \"%s\" to contain: \"%s\"", test.desc, err, test.expectedError)
			}
		}
	}
}
