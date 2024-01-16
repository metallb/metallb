/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestConfigController(t *testing.T) {
	tests := []struct {
		desc                    string
		handlerRes              SyncState
		validResources          bool
		expectReconcileFails    bool
		expectForceReloadCalled bool
	}{
		{
			desc:                    "handler returns SyncStateSuccess, valid resources",
			handlerRes:              SyncStateSuccess,
			validResources:          true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "handler returns SyncStateError, valid resources",
			handlerRes:              SyncStateError,
			validResources:          true,
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "handler returns SyncStateErrorNoRetry, valid resources",
			handlerRes:              SyncStateErrorNoRetry,
			validResources:          true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "handler returns SyncStateReprocessAll, valid resources",
			handlerRes:              SyncStateReprocessAll,
			validResources:          true,
			expectReconcileFails:    false,
			expectForceReloadCalled: true,
		},
		{
			desc:                    "handler returns SyncStateSuccess, invalid resources",
			handlerRes:              SyncStateSuccess,
			validResources:          false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
	}
	for _, test := range tests {
		var resources config.ClusterResources
		if test.validResources {
			resources = configControllerValidResources
		} else {
			resources = configControllerInvalidResources
		}

		initObjects := objectsFromResources(resources)
		fakeClient, err := newFakeClient(initObjects)
		if err != nil {
			t.Fatalf("test %s failed to create fake client: %v", test.desc, err)
		}

		expectedCfg, err := config.For(resources, config.DontValidate)
		if err != nil && test.validResources {
			t.Fatalf("test %s failed to create config, got unexpected error: %v", test.desc, err)
		}

		cmpOpt := cmpopts.IgnoreUnexported(config.Pool{})

		mockHandler := func(l log.Logger, cfg *config.Config) SyncState {
			if !cmp.Equal(expectedCfg, cfg, cmpOpt) {
				t.Errorf("test %s failed, handler called with unexpected config: %s", test.desc, cmp.Diff(expectedCfg, cfg, cmpOpt))
			}
			return test.handlerRes
		}

		calledForceReload := false
		mockForceReload := func() { calledForceReload = true }

		r := &ConfigReconciler{
			Client:         fakeClient,
			Logger:         log.NewNopLogger(),
			Scheme:         scheme.Scheme,
			Namespace:      testNamespace,
			ValidateConfig: config.DontValidate,
			Handler:        mockHandler,
			ForceReload:    mockForceReload,
		}
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: testNamespace,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		failedReconcile := err != nil

		if test.expectReconcileFails != failedReconcile {
			t.Errorf("%s: fail reconcile expected: %v, got: %v. err: %v", test.desc, test.expectReconcileFails, failedReconcile, err)
		}

		if test.expectForceReloadCalled != calledForceReload {
			t.Errorf("%s: call force reload expected: %v, got: %v", test.desc, test.expectForceReloadCalled, calledForceReload)
		}
	}
}

func TestSecretShouldntTrigger(t *testing.T) {
	initObjects := objectsFromResources(configControllerValidResources)
	fakeClient, err := newFakeClient(initObjects)
	if err != nil {
		t.Fatalf("test failed to create fake client: %v", err)
	}

	handlerCalled := false
	mockHandler := func(l log.Logger, cfg *config.Config) SyncState {
		handlerCalled = true
		return SyncStateSuccess
	}

	r := &ConfigReconciler{
		Client:         fakeClient,
		Logger:         log.NewNopLogger(),
		Scheme:         scheme.Scheme,
		Namespace:      testNamespace,
		ValidateConfig: config.DontValidate,
		Handler:        mockHandler,
		ForceReload:    func() {},
	}
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: testNamespace,
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !handlerCalled {
		t.Fatalf("handler not called")
	}
	handlerCalled = false
	err = fakeClient.Create(context.TODO(), &v1beta2.BGPPeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "peer2",
			Namespace: testNamespace,
		},
		Spec: v1beta2.BGPPeerSpec{
			MyASN:      42,
			ASN:        142,
			Address:    "1.2.3.4",
			BFDProfile: "default",
		},
	})
	if err != nil {
		t.Fatalf("create failed on peer2: %v", err)
	}
	_, err = r.Reconcile(context.TODO(), req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !handlerCalled {
		t.Fatalf("handler not called")
	}

	handlerCalled = false
	err = fakeClient.Create(context.TODO(), &corev1.Secret{Type: corev1.SecretTypeBasicAuth, ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: testNamespace},
		Data: map[string][]byte{"password": []byte("nopass")}})
	if err != nil {
		t.Fatalf("create failed on secret foo: %v", err)
	}
	_, err = r.Reconcile(context.TODO(), req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if handlerCalled {
		t.Fatalf("handler called")
	}
}

var (
	testNamespace                  = "test-controller"
	configControllerValidResources = config.ClusterResources{
		Peers: []v1beta2.BGPPeer{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "peer1",
					Namespace: testNamespace,
				},
				Spec: v1beta2.BGPPeerSpec{
					MyASN:      42,
					ASN:        142,
					Address:    "1.2.3.4",
					BFDProfile: "default",
				},
			},
		},
		BFDProfiles: []v1beta1.BFDProfile{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: testNamespace,
				},
			},
		},
		Pools: []v1beta1.IPAddressPool{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pool1",
					Namespace: testNamespace,
				},
				Spec: v1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"10.20.0.0/16",
					},
				},
			},
		},
		BGPAdvs: []v1beta1.BGPAdvertisement{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "adv1",
					Namespace: testNamespace,
				},
				Spec: v1beta1.BGPAdvertisementSpec{
					Communities: []string{"bar"},
				},
			},
		},
		L2Advs: []v1beta1.L2Advertisement{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "l2adv1",
					Namespace: testNamespace,
				},
			},
		},
		PasswordSecrets: map[string]corev1.Secret{
			"bgpsecret": {Type: corev1.SecretTypeBasicAuth, ObjectMeta: metav1.ObjectMeta{Name: "bgpsecret", Namespace: testNamespace},
				Data: map[string][]byte{"password": []byte("nopass")}},
		},
		Communities: []v1beta1.Community{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "community",
					Namespace: testNamespace,
				},
				Spec: v1beta1.CommunitySpec{
					Communities: []v1beta1.CommunityAlias{
						{
							Name:  "bar",
							Value: "64512:1234",
						},
					},
				},
			},
		},
	}
	configControllerInvalidResources = config.ClusterResources{
		Peers: []v1beta2.BGPPeer{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "peer1",
					Namespace: testNamespace,
				},
			},
		},
	}
)
