// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"testing"

	"go.universe.tf/metallb/api/v1beta2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateBFDProfile(t *testing.T) {
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

	desc := "Delete bfdprofile used by bgppeer"
	bfdProfile := &BFDProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bfdprofile",
			Namespace: MetalLBTestNameSpace,
		},
	}

	err := bfdProfile.ValidateDelete()

	if err == nil {
		t.Fatalf("test %s failed, expecting error", desc)
	}
}
