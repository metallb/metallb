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
	MetalLBNamespace = MetalLBTestNameSpace
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
	toRestoreIPAddressPools := getExistingIPAddressPools
	getExistingIPAddressPools = func() (*IPAddressPoolList, error) {
		return &IPAddressPoolList{}, nil
	}

	defer func() {
		getExistingAddressPools = toRestoreAddresspools
		getExistingIPAddressPools = toRestoreIPAddressPools
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
		{
			desc: "Validation must fail if created in different namespace",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: "default",
				},
			},
			isNewAddressPool: true,
			expected:         nil,
			failValidate:     true,
		},
	}

	for _, test := range tests {
		var err error
		mock := &mockValidator{}
		Validator = mock
		mock.forceError = test.failValidate

		if test.isNewAddressPool {
			_, err = test.addressPool.ValidateCreate()
		} else {
			_, err = test.addressPool.ValidateUpdate(nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.pools) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.pools))
		}
	}
}
