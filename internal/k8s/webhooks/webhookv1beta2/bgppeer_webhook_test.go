// SPDX-License-Identifier:Apache-2.0

package webhookv1beta2

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"go.universe.tf/metallb/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testNamespace = "namespace"

func TestValidateBGPPeer(t *testing.T) {
	MetalLBNamespace = testNamespace
	bgpPeer := v1beta2.BGPPeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-peer",
			Namespace: testNamespace,
		},
	}

	Logger = log.NewNopLogger()

	toRestorePeers := GetExistingBGPPeers
	GetExistingBGPPeers = func() (*v1beta2.BGPPeerList, error) {
		return &v1beta2.BGPPeerList{
			Items: []v1beta2.BGPPeer{
				bgpPeer,
			},
		}, nil
	}
	toRestoreNodes := getExistingNodes
	getExistingNodes = func() (*corev1.NodeList, error) {
		return &corev1.NodeList{}, nil
	}
	toRestoreValidator := Validator

	defer func() {
		GetExistingBGPPeers = toRestorePeers
		getExistingNodes = toRestoreNodes
		Validator = toRestoreValidator
	}()

	tests := []struct {
		desc         string
		bgpPeer      *v1beta2.BGPPeer
		isNew        bool
		failValidate bool
		expected     *v1beta2.BGPPeerList
	}{
		{
			desc: "Second Peer",
			bgpPeer: &v1beta2.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: testNamespace,
				},
			},
			isNew: true,
			expected: &v1beta2.BGPPeerList{
				Items: []v1beta2.BGPPeer{
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
			bgpPeer: &v1beta2.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-peer",
					Namespace: testNamespace,
				},
			},
			isNew: false,
			expected: &v1beta2.BGPPeerList{
				Items: []v1beta2.BGPPeer{
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
			bgpPeer: &v1beta2.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-peer",
					Namespace: testNamespace,
				},
			},
			isNew: false,
			expected: &v1beta2.BGPPeerList{
				Items: []v1beta2.BGPPeer{
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
		{
			desc: "Validation must fail if created in different namespace",
			bgpPeer: &v1beta2.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-peer1",
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
			_, err = validatePeerCreate(test.bgpPeer)
		} else {
			err = validatePeerUpdate(test.bgpPeer, nil)
		}
		if test.failValidate && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !cmp.Equal(test.expected, mock.bgpPeers) {
			t.Fatalf("test %s failed, %s", test.desc, cmp.Diff(test.expected, mock.bgpPeers))
		}
	}
}

// TestValidateBGPPeerCreateWithDisableMP verifies that the disableMP deprecation
// warning does not skip the namespace check nor the cluster validation.
func TestValidateBGPPeerCreateWithDisableMP(t *testing.T) {
	MetalLBNamespace = testNamespace
	Logger = log.NewNopLogger()

	toRestorePeers := GetExistingBGPPeers
	GetExistingBGPPeers = func() (*v1beta2.BGPPeerList, error) {
		return &v1beta2.BGPPeerList{}, nil
	}
	toRestoreNodes := getExistingNodes
	getExistingNodes = func() (*corev1.NodeList, error) {
		return &corev1.NodeList{}, nil
	}
	toRestoreValidator := Validator
	defer func() {
		GetExistingBGPPeers = toRestorePeers
		getExistingNodes = toRestoreNodes
		Validator = toRestoreValidator
	}()

	tests := []struct {
		desc         string
		namespace    string
		failValidate bool
		expectErr    bool
		expectWarn   bool
	}{
		{
			desc:       "valid peer is admitted with the deprecation warning",
			namespace:  testNamespace,
			expectWarn: true,
		},
		{
			desc:      "peer in a different namespace is rejected",
			namespace: "default",
			expectErr: true,
		},
		{
			desc:         "peer failing cluster validation is rejected",
			namespace:    testNamespace,
			failValidate: true,
			expectErr:    true,
		},
	}
	for _, test := range tests {
		mock := &mockValidator{forceError: test.failValidate}
		Validator = mock
		warning, err := validatePeerCreate(&v1beta2.BGPPeer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-peer",
				Namespace: test.namespace,
			},
			Spec: v1beta2.BGPPeerSpec{DisableMP: true},
		})
		if test.expectErr && err == nil {
			t.Fatalf("test %s failed, expecting error", test.desc)
		}
		if !test.expectErr && err != nil {
			t.Fatalf("test %s failed, unexpected error %v", test.desc, err)
		}
		if test.expectWarn && warning == "" {
			t.Fatalf("test %s failed, expecting the deprecation warning", test.desc)
		}
	}
}

// TestValidateBGPPeerPassesNodesToValidator verifies that validatePeerCreate and
// validatePeerUpdate forward the node list to the validator so that
// DiscardNativeOnly can perform nodeSelector overlap checks.
func TestValidateBGPPeerPassesNodesToValidator(t *testing.T) {
	MetalLBNamespace = testNamespace
	Logger = log.NewNopLogger()

	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"kubernetes.io/hostname": "node1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"kubernetes.io/hostname": "node2"}}},
		},
	}

	// The node list is always fetched and forwarded to the validator so that
	// DiscardNativeOnly can perform nodeSelector overlap checks.
	existingPeer := &v1beta2.BGPPeer{
		ObjectMeta: metav1.ObjectMeta{Name: "p-existing", Namespace: testNamespace},
		Spec:       v1beta2.BGPPeerSpec{Address: "192.0.2.1"},
	}
	peer := &v1beta2.BGPPeer{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: testNamespace},
		Spec:       v1beta2.BGPPeerSpec{Address: "192.0.2.1"},
	}

	toRestorePeers := GetExistingBGPPeers
	GetExistingBGPPeers = func() (*v1beta2.BGPPeerList, error) {
		return &v1beta2.BGPPeerList{Items: []v1beta2.BGPPeer{*existingPeer}}, nil
	}
	toRestoreNodes := getExistingNodes
	getExistingNodes = func() (*corev1.NodeList, error) { return nodes, nil }
	defer func() {
		GetExistingBGPPeers = toRestorePeers
		getExistingNodes = toRestoreNodes
	}()

	t.Run("create", func(t *testing.T) {
		prev := Validator
		t.Cleanup(func() { Validator = prev })
		mock := &mockValidator{}
		Validator = mock
		_, _ = validatePeerCreate(peer)
		if diff := cmp.Diff(nodes, mock.nodes); diff != "" {
			t.Fatalf("node list not forwarded to validator (-want +got):\n%s", diff)
		}
	})

	t.Run("update", func(t *testing.T) {
		prev := Validator
		t.Cleanup(func() { Validator = prev })
		mock := &mockValidator{}
		Validator = mock
		_ = validatePeerUpdate(peer, nil)
		if diff := cmp.Diff(nodes, mock.nodes); diff != "" {
			t.Fatalf("node list not forwarded to validator (-want +got):\n%s", diff)
		}
	})
}
