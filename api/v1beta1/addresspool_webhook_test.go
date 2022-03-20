// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetalLBTestNameSpace = "metallb-test-namespace"
)

func TestValidateAddressPool(t *testing.T) {
	addressPool := AddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addresspool",
			Namespace: MetalLBTestNameSpace,
		},
	}
	Logger = log.NewNopLogger()

	toRestoreAddresspools := getExistingAddressPools
	getExistingAddressPools = func() (*AddressPoolList, error) {
		return &AddressPoolList{
			Items: []AddressPool{
				addressPool,
			},
		}, nil
	}
	toRestoreIPpools := getExistingIPPools
	getExistingIPPools = func() (*IPPoolList, error) {
		return &IPPoolList{}, nil
	}

	defer func() {
		getExistingAddressPools = toRestoreAddresspools
		getExistingIPPools = toRestoreIPpools
	}()

	tests := []struct {
		desc             string
		addressPool      *AddressPool
		isNewAddressPool bool
		failValidate     bool
		expected         *AddressPoolList
	}{
		{
			desc: "Second AddressPool",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool1",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewAddressPool: true,
			expected: &AddressPoolList{
				Items: []AddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-addresspool",
							Namespace: MetalLBTestNameSpace,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-addresspool1",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Same AddressPool, update",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewAddressPool: false,
			expected: &AddressPoolList{
				Items: []AddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-addresspool",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Validation Fails",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewAddressPool: false,
			expected: &AddressPoolList{
				Items: []AddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-addresspool",
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

		if test.isNewAddressPool {
			err = test.addressPool.ValidateCreate()
		} else {
			err = test.addressPool.ValidateUpdate(nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.pools) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.pools))
		}
	}
}
