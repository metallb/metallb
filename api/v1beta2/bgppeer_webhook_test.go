// SPDX-License-Identifier:Apache-2.0

package v1beta2

import (
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetalLBTestNameSpace = "metallb-test-namespace"
)

func TestValidateBGPPeer(t *testing.T) {
	bgpPeer := BGPPeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bgppeer",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: BGPPeerSpec{
			Address:  "10.0.0.1",
			ASN:      64501,
			MyASN:    64500,
			RouterID: "10.10.10.10",
		},
	}
	bgpPeerList := &BGPPeerList{}
	bgpPeerList.Items = append(bgpPeerList.Items, bgpPeer)
	tests := []struct {
		desc          string
		bgpPeer       *BGPPeer
		expectedError string
	}{
		{
			desc: "Invalid RouterID",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:  "10.0.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "11.11.11.300",
				},
			},
			expectedError: "Invalid RouterID",
		},
		{
			desc: "BGPPeer with different RouterID",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:  "10.0.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "11.11.11.11",
				},
			},
			expectedError: "BGPPeers with different RouterID not supported ",
		},
		{
			desc: "Invalid BGP Peer IP address",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:  "10.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "10.10.10.10",
				},
			},
			expectedError: "Invalid BGPPeer address",
		},
		{
			desc: "Duplicate BGP Peer",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:  "10.0.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "10.10.10.10",
				},
			},
			expectedError: "Duplicate BGPPeer",
		},
		{
			desc: "Invalid BGP Peer source address",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:    "10.0.1.1",
					SrcAddress: "10:",
					ASN:        64501,
					MyASN:      64500,
					RouterID:   "10.10.10.10",
				},
			},
			expectedError: "Invalid BGPPeer source address",
		},
		{
			desc: "Different myASN configuration",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:  "10.0.1.1",
					ASN:      64501,
					MyASN:    64400,
					RouterID: "10.10.10.10",
				},
			},
			expectedError: "Multiple local ASN not supported in FRR mode",
		},
		{
			desc: "Invalid keepalive time configuration",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:       "10.0.0.2",
					ASN:           64502,
					MyASN:         64500,
					HoldTime:      metav1.Duration{Duration: 90 * time.Second},
					KeepaliveTime: metav1.Duration{Duration: 180 * time.Second},
					RouterID:      "10.10.10.10",
				},
			},
			expectedError: "Invalid keepalive time",
		},
		{
			desc: "Missing holdtime time configuration",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:       "10.0.0.2",
					ASN:           64502,
					MyASN:         64500,
					KeepaliveTime: metav1.Duration{Duration: 180 * time.Second},
					RouterID:      "10.10.10.10",
				},
			},
			expectedError: "Missing to configure HoldTime",
		},
		{
			desc: "Invalid EBGPMultiHop for IBGP peer",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:      "10.0.0.2",
					ASN:          64502,
					MyASN:        64502,
					RouterID:     "10.10.10.10",
					EBGPMultiHop: true,
				},
			},
			expectedError: "Invalid EBGPMultiHop parameter set for an ibgp peer",
		},
		{
			desc: "Invalid holdtime time configuration",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgppeer",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BGPPeerSpec{
					Address:  "10.0.0.2",
					ASN:      64502,
					MyASN:    64500,
					HoldTime: metav1.Duration{Duration: 2 * time.Second},
					RouterID: "10.10.10.10",
				},
			},
			expectedError: "Invalid hold time",
		},
	}

	for _, test := range tests {
		err := test.bgpPeer.ValidateBGPPeer(bgpPeerList.Items, true)
		if err == nil {
			t.Errorf("%s: ValidateBGPPeer failed, no error found while expected: \"%s\"", test.desc, test.expectedError)
		} else {
			if !strings.Contains(fmt.Sprint(err), test.expectedError) {
				t.Errorf("%s: ValidateBGPPeer failed, expected error: \"%s\" to contain: \"%s\"", test.desc, err, test.expectedError)
			}
		}
	}
}
