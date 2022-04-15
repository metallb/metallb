// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateCommunity(t *testing.T) {
	Logger = log.NewNopLogger()

	toRestoreCommunities := getExistingCommunities
	getExistingCommunities = func() (*CommunityList, error) {
		return &CommunityList{
			Items: []Community{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-commuinty1",
						Namespace: MetalLBTestNameSpace,
					},
				},
			},
		}, nil
	}
	defer func() {
		getExistingCommunities = toRestoreCommunities
	}()

	tests := []struct {
		desc           string
		commuinty      *Community
		isNewCommunity bool
		failValidate   bool
		expected       *CommunityList
	}{
		{
			desc: "Second Community",
			commuinty: &Community{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-community2",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewCommunity: true,
			expected: &CommunityList{
				Items: []Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-commuinty1",
							Namespace: MetalLBTestNameSpace,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-community2",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Same Community, update",
			commuinty: &Community{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-commuinty1",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewCommunity: false,
			expected: &CommunityList{
				Items: []Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-commuinty1",
							Namespace: MetalLBTestNameSpace,
						},
					},
				},
			},
		},
		{
			desc: "Same community, new",
			commuinty: &Community{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-commuinty1",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewCommunity: true,
			expected: &CommunityList{
				Items: []Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-commuinty1",
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

		if test.isNewCommunity {
			err = test.commuinty.ValidateCreate()
		} else {
			err = test.commuinty.ValidateUpdate(nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.communities) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.pools))
		}
	}
}
