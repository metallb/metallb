// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateL2Advertisement(t *testing.T) {
	MetalLBNamespace = MetalLBTestNameSpace
	l2Adv := L2Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-l2adv",
			Namespace: MetalLBTestNameSpace,
		},
	}

	Logger = log.NewNopLogger()

	toRestore := getExistingL2Advs
	getExistingL2Advs = func() (*L2AdvertisementList, error) {
		return &L2AdvertisementList{
			Items: []L2Advertisement{
				l2Adv,
			},
		}, nil
	}
	toRestoreAddresspools := getExistingAddressPools
	getExistingAddressPools = func() (*AddressPoolList, error) {
		return &AddressPoolList{}, nil
	}
	toRestoreIPAddressPools := getExistingIPAddressPools
	getExistingIPAddressPools = func() (*IPAddressPoolList, error) {
		return &IPAddressPoolList{}, nil
	}

	defer func() {
		getExistingL2Advs = toRestore
		getExistingAddressPools = toRestoreAddresspools
		getExistingIPAddressPools = toRestoreIPAddressPools
	}()

	tests := []struct {
		desc         string
		l2Adv        *L2Advertisement
		isNew        bool
		failValidate bool
		expected     *L2AdvertisementList
	}{
		{
			desc: "Second Adv",
			l2Adv: &L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: true,
			expected: &L2AdvertisementList{
				Items: []L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-l2adv",
							Namespace: MetalLBTestNameSpace,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Same, update",
			l2Adv: &L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-l2adv",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: false,
			expected: &L2AdvertisementList{
				Items: []L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-l2adv",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Same, new",
			l2Adv: &L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-l2adv",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: true,
			expected: &L2AdvertisementList{
				Items: []L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-l2adv",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
			failValidate: true,
		},
		{
			desc: "Validation must fail if created in different namespace",
			l2Adv: &L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-l2adv1",
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
			_, err = test.l2Adv.ValidateCreate()
		} else {
			_, err = test.l2Adv.ValidateUpdate(nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.l2Advs) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.l2Advs))
		}
	}
}
