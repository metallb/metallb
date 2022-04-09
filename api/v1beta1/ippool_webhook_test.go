// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateIPPool(t *testing.T) {
	ipPool := IPPool{
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
	toRestoreIPpools := getExistingIPPools
	getExistingIPPools = func() (*IPPoolList, error) {
		return &IPPoolList{
			Items: []IPPool{
				ipPool,
			},
		}, nil
	}

	defer func() {
		getExistingAddressPools = toRestoreAddresspools
		getExistingIPPools = toRestoreIPpools
	}()

	tests := []struct {
		desc         string
		ipPool       *IPPool
		isNew        bool
		failValidate bool
		expected     *IPPoolList
	}{
		{
			desc: "Second IPPool",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool1",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: true,
			expected: &IPPoolList{
				Items: []IPPool{
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
			desc: "Same IPPool, update",
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: false,
			expected: &IPPoolList{
				Items: []IPPool{
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
			ipPool: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: false,
			expected: &IPPoolList{
				Items: []IPPool{
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
			err = test.ipPool.ValidateCreate()
		} else {
			err = test.ipPool.ValidateUpdate(nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.ipPools) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.ipPools))
		}
	}
}
