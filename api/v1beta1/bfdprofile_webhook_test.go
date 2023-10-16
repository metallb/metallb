// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/api/v1beta2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateBFDProfile(t *testing.T) {
	MetalLBNamespace = MetalLBTestNameSpace
	Logger = log.NewNopLogger()
	toRestoreBGPPeers := v1beta2.GetExistingBGPPeers
	v1beta2.GetExistingBGPPeers = func() (*v1beta2.BGPPeerList, error) {
		return &v1beta2.BGPPeerList{
			Items: []v1beta2.BGPPeer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-bgppeer",
						Namespace: MetalLBTestNameSpace,
					},
					Spec: v1beta2.BGPPeerSpec{
						BFDProfile: "bfdprofile",
					},
				},
			},
		}, nil
	}
	defer func() {
		v1beta2.GetExistingBGPPeers = toRestoreBGPPeers
	}()

	const (
		isNew int = iota
		isDel
	)
	tests := []struct {
		desc         string
		bfdProfile   *BFDProfile
		validateType int
		failValidate bool
	}{
		{
			desc:         "Delete bfdprofile used by bgppeer",
			validateType: isDel,
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
			},
			failValidate: true,
		},
		{
			desc:         "Validation must fail if created in different namespace",
			validateType: isNew,
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bfdprofile1",
					Namespace: "default",
				},
			},
			failValidate: true,
		},
	}

	for _, test := range tests {
		var err error
		switch test.validateType {
		case isNew:
			_, err = test.bfdProfile.ValidateCreate()
		case isDel:
			_, err = test.bfdProfile.ValidateDelete()
		}

		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
	}
}
