// SPDX-License-Identifier:Apache-2.0

package webhookv1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetalLBTestNameSpace = "metallb-test-namespace"
)

func TestValidateAddressPool(t *testing.T) {
	MetalLBNamespace = MetalLBTestNameSpace
	addressPool := v1beta1.AddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addresspool",
			Namespace: MetalLBTestNameSpace,
		},
	}
	Logger = log.NewNopLogger()

	toRestoreAddresspools := getExistingAddressPools
	getExistingAddressPools = func() (*v1beta1.AddressPoolList, error) {
		return &v1beta1.AddressPoolList{
			Items: []v1beta1.AddressPool{
				addressPool,
			},
		}, nil
	}
	toRestoreIPAddressPools := getExistingIPAddressPools
	getExistingIPAddressPools = func() (*v1beta1.IPAddressPoolList, error) {
		return &v1beta1.IPAddressPoolList{}, nil
	}

	defer func() {
		getExistingAddressPools = toRestoreAddresspools
		getExistingIPAddressPools = toRestoreIPAddressPools
	}()

	tests := []struct {
		desc             string
		addressPool      *v1beta1.AddressPool
		isNewAddressPool bool
		failValidate     bool
		expected         *v1beta1.AddressPoolList
	}{
		{
			desc: "Second AddressPool",
			addressPool: &v1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool1",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewAddressPool: true,
			expected: &v1beta1.AddressPoolList{
				Items: []v1beta1.AddressPool{
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
			addressPool: &v1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewAddressPool: false,
			expected: &v1beta1.AddressPoolList{
				Items: []v1beta1.AddressPool{
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
			addressPool: &v1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewAddressPool: false,
			expected: &v1beta1.AddressPoolList{
				Items: []v1beta1.AddressPool{
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
			addressPool: &v1beta1.AddressPool{
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
			err = validateAddressPoolCreate(test.addressPool)
		} else {
			err = validateAddressPoolUpdate(test.addressPool, nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.pools) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.pools))
		}
	}
}
