// SPDX-License-Identifier:Apache-2.0

package webhookv1beta1

import (
	"testing"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/k8s/webhooks/webhookv1beta2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetalLBTestNameSpace = "metallb-test-namespace"
)

func TestValidateBFDProfile(t *testing.T) {
	MetalLBNamespace = MetalLBTestNameSpace
	Logger = log.NewNopLogger()
	toRestoreBGPPeers := webhookv1beta2.GetExistingBGPPeers
	webhookv1beta2.GetExistingBGPPeers = func() (*v1beta2.BGPPeerList, error) {
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
		webhookv1beta2.GetExistingBGPPeers = toRestoreBGPPeers
	}()

	const (
		isNew int = iota
		isDel
	)
	tests := []struct {
		desc         string
		bfdProfile   *v1beta1.BFDProfile
		validateType int
		failValidate bool
	}{
		{
			desc:         "Delete bfdprofile used by bgppeer",
			validateType: isDel,
			bfdProfile: &v1beta1.BFDProfile{
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
			bfdProfile: &v1beta1.BFDProfile{
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
			err = validateBFDCreate(test.bfdProfile)
		case isDel:
			err = validateBFDDelete(test.bfdProfile)
		}

		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
	}
}
