// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateIPAddressPool(t *testing.T) {
	ipAddressPool := IPAddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ippool",
			Namespace: MetalLBTestNameSpace,
		},
	}
	Logger = log.NewNopLogger()

	toRestoreAddresspools := getExistingAddressPools
	getExistingAddressPools = func() (*AddressPoolList, error) {
		return &AddressPoolList{}, nil
	}
	toRestoreIPAddressPools := getExistingIPAddressPools
	getExistingIPAddressPools = func() (*IPAddressPoolList, error) {
		return &IPAddressPoolList{
			Items: []IPAddressPool{
				ipAddressPool,
			},
		}, nil
	}

	defer func() {
		getExistingAddressPools = toRestoreAddresspools
		getExistingIPAddressPools = toRestoreIPAddressPools
	}()

	tests := []struct {
		desc          string
		ipAddressPool *IPAddressPool
		isNew         bool
		failValidate  bool
		expected      *IPAddressPoolList
	}{
		{
			desc: "Second IPAddressPool",
			ipAddressPool: &IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool1",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: true,
			expected: &IPAddressPoolList{
				Items: []IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-ippool",
							Namespace: MetalLBTestNameSpace,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-ippool1",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Same IPAddressPool, update",
			ipAddressPool: &IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: false,
			expected: &IPAddressPoolList{
				Items: []IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-ippool",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Validation fails",
			ipAddressPool: &IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: false,
			expected: &IPAddressPoolList{
				Items: []IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-ippool",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
			failValidate: true,
		},
	}

	for _, test := range tests {
		var err error
		mock := &mockValidator{}
		Validator = mock
		mock.forceError = test.failValidate

		if test.isNew {
			err = test.ipAddressPool.ValidateCreate()
		} else {
			err = test.ipAddressPool.ValidateUpdate(nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.ipAddressPools) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.ipAddressPools))
		}
	}
}
