// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateIPPool(t *testing.T) {
	autoAssign := false
	ipPool := IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ipPool",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: IPPoolSpec{
			Addresses: []string{
				"1.1.1.1-1.1.1.100",
			},
			AutoAssign: &autoAssign,
		},
	}
	ipPoolList := &IPPoolList{}
	ipPoolList.Items = append(ipPoolList.Items, ipPool)

	tests := []struct {
		desc             string
		ipPool           *IPPool
		isNewAddressPool bool
		expectedError    string
	}{
		{
			desc: "Second IPPool, already defined name",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ipPool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: IPPoolSpec{
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
			desc: "Second IPPool, overlapping addresses defined by address range",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: IPPoolSpec{
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
			desc: "Second IPPool, overlapping addresses defined by network prefix",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: IPPoolSpec{
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
			desc: "Second IPPool, invalid CIDR, single address provided while expecting a range",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: IPPoolSpec{
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
			desc: "Second IPPool, invalid CIDR, first address of the range is after the second",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: IPPoolSpec{
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
			desc: "Second IPPool, invalid ipv6 CIDR, single address provided while expecting a range",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: IPPoolSpec{
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
			desc: "Second IPPool, invalid ipv6 CIDR, first address of the range is after the second",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: IPPoolSpec{
					Addresses: []string{
						"2000:::ffff-2000::",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid IP range",
		},
	}

	for _, test := range tests {
		err := test.ipPool.ValidateIPPool(test.isNewAddressPool, ipPoolList.Items, nil)
		if err == nil {
			t.Errorf("%s: ValidateIPPool failed, no error found while expected: \"%s\"", test.desc, test.expectedError)
		} else {
			if !strings.Contains(fmt.Sprint(err), test.expectedError) {
				t.Errorf("%s: ValidateIPPool failed, expected error: \"%s\" to contain: \"%s\"", test.desc, err, test.expectedError)
			}
		}
	}
}
