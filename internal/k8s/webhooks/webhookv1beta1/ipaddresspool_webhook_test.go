// SPDX-License-Identifier:Apache-2.0

package webhookv1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"go.universe.tf/metallb/api/v1beta1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateIPAddressPool(t *testing.T) {
	MetalLBNamespace = MetalLBTestNameSpace
	ipAddressPool := v1beta1.IPAddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ippool",
			Namespace: MetalLBTestNameSpace,
		},
	}
	Logger = log.NewNopLogger()

	toRestoreIPAddressPools := getExistingIPAddressPools
	getExistingIPAddressPools = func() (*v1beta1.IPAddressPoolList, error) {
		return &v1beta1.IPAddressPoolList{
			Items: []v1beta1.IPAddressPool{
				ipAddressPool,
			},
		}, nil
	}
	toRestoreNodes := getExistingNodes
	getExistingNodes = func() (*v1core.NodeList, error) {
		return &v1core.NodeList{}, nil
	}

	defer func() {
		getExistingIPAddressPools = toRestoreIPAddressPools
		getExistingNodes = toRestoreNodes
	}()

	tests := []struct {
		desc          string
		ipAddressPool *v1beta1.IPAddressPool
		isNew         bool
		failValidate  bool
		expected      *v1beta1.IPAddressPoolList
	}{
		{
			desc: "Second IPAddressPool",
			ipAddressPool: &v1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool1",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: true,
			expected: &v1beta1.IPAddressPoolList{
				Items: []v1beta1.IPAddressPool{
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
			ipAddressPool: &v1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: false,
			expected: &v1beta1.IPAddressPoolList{
				Items: []v1beta1.IPAddressPool{
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
			ipAddressPool: &v1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: false,
			expected: &v1beta1.IPAddressPoolList{
				Items: []v1beta1.IPAddressPool{
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
		{
			desc: "Validation must fail if created in different namespace",
			ipAddressPool: &v1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool2",
					Namespace: "default",
				},
			},
			isNew:        true,
			expected:     nil,
			failValidate: true,
		},
	}

	for _, test := range tests {
		var err error
		mock := &mockValidator{}
		Validator = mock
		mock.forceError = test.failValidate

		if test.isNew {
			err = validateIPAddressPoolCreate(test.ipAddressPool)
		} else {
			err = validateIPAddressPoolUpdate(test.ipAddressPool, nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.ipAddressPools) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.ipAddressPools))
		}
	}
}
