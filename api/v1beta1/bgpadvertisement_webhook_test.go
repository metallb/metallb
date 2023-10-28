// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateBGPAdvertisement(t *testing.T) {
	MetalLBNamespace = MetalLBTestNameSpace
	bgpAdv := BGPAdvertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bgpadv",
			Namespace: MetalLBTestNameSpace,
		},
	}

	Logger = log.NewNopLogger()

	toRestore := getExistingBGPAdvs
	getExistingBGPAdvs = func() (*BGPAdvertisementList, error) {
		return &BGPAdvertisementList{
			Items: []BGPAdvertisement{
				bgpAdv,
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
	toRestoreNodes := getExistingNodes
	getExistingNodes = func() (*v1core.NodeList, error) {
		return &v1core.NodeList{}, nil
	}

	defer func() {
		getExistingBGPAdvs = toRestore
		getExistingAddressPools = toRestoreAddresspools
		getExistingIPAddressPools = toRestoreIPAddressPools
		getExistingNodes = toRestoreNodes
	}()

	tests := []struct {
		desc         string
		bgpAdv       *BGPAdvertisement
		isNew        bool
		failValidate bool
		expected     *BGPAdvertisementList
	}{
		{
			desc: "Second Adv",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: true,
			expected: &BGPAdvertisementList{
				Items: []BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-bgpadv",
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
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpadv",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: false,
			expected: &BGPAdvertisementList{
				Items: []BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-bgpadv",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Same, new",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpadv",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNew: true,
			expected: &BGPAdvertisementList{
				Items: []BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-bgpadv",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
			failValidate: true,
		},
		{
			desc: "Validation must fail if created in different namespace",
			bgpAdv: &BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgpadv1",
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
			_, err = test.bgpAdv.ValidateCreate()
		} else {
			_, err = test.bgpAdv.ValidateUpdate(nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.bgpAdvs) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.bgpAdvs))
		}
	}
}
