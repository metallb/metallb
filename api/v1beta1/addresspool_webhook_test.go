// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	MetalLBTestNameSpace = "metallb-test-namespace"
)

func TestValidateAddressPool(t *testing.T) {
	autoAssign := false
	addressPool := AddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addresspool",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: AddressPoolSpec{
			Protocol: "layer2",
			Addresses: []string{
				"1.1.1.1-1.1.1.100",
			},
			AutoAssign: &autoAssign,
		},
	}
	addressPoolList := &AddressPoolList{}
	addressPoolList.Items = append(addressPoolList.Items, addressPool)

	tests := []struct {
		desc             string
		addressPool      *AddressPool
		isNewAddressPool bool
		expectedError    string
	}{
		{
			desc: "No pool name",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.101-1.1.1.200",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "Missing AddressPool name",
		},
		{
			desc: "AddressPool with no address",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
				},
			},
			isNewAddressPool: true,
			expectedError:    "AddressPool has no prefixes defined",
		},
		{
			desc: "AddressPool with no protocol",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Addresses: []string{
						"1.1.1.101-1.1.1.200",
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "AddressPool is missing the protocol field",
		},
		{
			desc: "AddressPool with invalid protocol",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "babel",
					Addresses: []string{
						"1.1.1.101-1.1.1.200",
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "AddressPool has unknown protocol \"babel\"",
		},
		{
			desc: "Second AddressPool, already defined name",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.101-1.1.1.200",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "duplicate definition of pool",
		},
		{
			desc: "Second AddressPool, overlapping addresses defined by address range",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.15-1.1.1.20",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "overlaps with already defined CIDR",
		},
		{
			desc: "Second AddressPool, overlapping addresses defined by network prefix",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.0/24",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "overlaps with already defined CIDR",
		},
		{
			desc: "Second AddressPool, invalid CIDR, single address provided while expecting a range",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.15",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid CIDR",
		},
		{
			desc: "Second AddressPool, invalid CIDR, first address of the range is after the second",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.200-1.1.1.101",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid IP range",
		},
		{
			desc: "Second AddressPool, invalid ipv6 CIDR, single address provided while expecting a range",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2000::",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid CIDR",
		},
		{
			desc: "Second AddressPool, invalid ipv6 CIDR, first address of the range is after the second",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2000:::ffff-2000::",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid IP range",
		},
		{
			desc: "Invalid protocol used while using bgp advertisments",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2.2.2.2-2.2.2.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(24),
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "bgpadvertisement config not valid",
		},
		{
			desc: "Duplicate bgp advertisment in AddressPool",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
						{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "duplicate definition of bgpadvertisement",
		},
		{
			desc: "Duplicate communities in AddressPool",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"65535:65282",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "duplicate definition of communities",
		},
		{
			desc: "Bad IPv4 aggregation length in bgp advertisment (too long)",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(33),
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid aggregation length 33 for IPv4",
		},
		{
			desc: "Bad IPv6 aggregation length in bgp advertisment (too long)",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1000::/127",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLengthV6: pointer.Int32Ptr(129),
							LocalPref:           100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid aggregation length 129 for IPv6",
		},
		{
			desc: "Bad IPv4 aggregation length in bgp advertisment (incompatible with CIDR)",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"10.20.30.40/24",
						"1.2.3.0/28",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(26),
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid aggregation length 26: prefix 28 in this pool is more specific than the aggregation length for addresses 1.2.3.0/28",
		},
		{
			desc: "Bad IPv4 aggregation length by range in bgp advertisment (too wide)",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"3.3.3.2-3.3.3.254",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(24),
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid aggregation length 24: prefix 26 in this pool is more specific than the aggregation length for addresses 3.3.3.2-3.3.3.254",
		},
		{
			desc: "Bad community literal in bgp advertisment (wrong format)",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         100,
							Communities: []string{
								"65535",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid community string \"65535\"",
		},
		{
			desc: "Bad community literal in bgp advertisment (asn part doesn't fit)",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         100,
							Communities: []string{
								"99999999:1",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid first section of community \"99999999\"",
		},
		{
			desc: "Bad community literal in bgp advertisment (community# part doesn't fit)",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         100,
							Communities: []string{
								"1:99999999",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid second section of community \"99999999\"",
		},
	}

	for _, test := range tests {
		err := test.addressPool.ValidateAddressPool(test.isNewAddressPool, addressPoolList.Items, nil)
		if err == nil {
			t.Errorf("%s: ValidateAddressPool failed, no error found while expected: \"%s\"", test.desc, test.expectedError)
		} else {
			if !strings.Contains(fmt.Sprint(err), test.expectedError) {
				t.Errorf("%s: ValidateAddressPool failed, expected error: \"%s\" to contain: \"%s\"", test.desc, err, test.expectedError)
			}
		}
	}
}
