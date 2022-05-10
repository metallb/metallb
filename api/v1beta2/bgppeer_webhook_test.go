// SPDX-License-Identifier:Apache-2.0

package v1beta2

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testNamespace = "namespace"

func TestValidateBGPPeer(t *testing.T) {
	bgpPeer := BGPPeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-peer",
			Namespace: testNamespace,
		},
	}

	Logger = log.NewNopLogger()

	toRestore := GetExistingBGPPeers
	GetExistingBGPPeers = func() (*BGPPeerList, error) {
		return &BGPPeerList{
			Items: []BGPPeer{
				bgpPeer,
			},
		}, nil
	}

	defer func() {
		GetExistingBGPPeers = toRestore
	}()

	tests := []struct {
		desc         string
		bgpPeer      *BGPPeer
		isNew        bool
		failValidate bool
		expected     *BGPPeerList
	}{
		{
			desc: "Second Peer",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: testNamespace,
				},
			},
			isNew: true,
			expected: &BGPPeerList{
				Items: []BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-peer",
							Namespace: testNamespace,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: testNamespace,
						},
					},
				},
			},
		},
		{
			desc: "Same, update",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-peer",
					Namespace: testNamespace,
				},
			},
			isNew: false,
			expected: &BGPPeerList{
				Items: []BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-peer",
							Namespace: testNamespace,
						},
					},
				},
			},
		},
		{
			desc: "Validation failed",
			bgpPeer: &BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-peer",
					Namespace: testNamespace,
				},
			},
			isNew: false,
			expected: &BGPPeerList{
				Items: []BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-peer",
							Namespace: testNamespace,
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
			err = test.bgpPeer.ValidateCreate()
		} else {
			err = test.bgpPeer.ValidateUpdate(nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.bgpPeers) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.bgpPeers))
		}
	}
}
