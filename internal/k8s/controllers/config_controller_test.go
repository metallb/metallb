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
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/onsi/gomega"
	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	metallbcfg "go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
		var resources metallbcfg.ClusterResources
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

		cmpOpt := cmpopts.IgnoreUnexported(metallbcfg.Pool{})

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
			Scheme:         scheme,
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
		Scheme:         scheme,
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
		Data: map[string][]byte{"password": []byte([]byte("nopass"))}})
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

func TestNodeEvent(t *testing.T) {
	g := NewGomegaWithT(t)
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("../../..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		Scheme:                scheme,
	}
	cfg, err := testEnv.Start()
	g.Expect(err).ToNot(HaveOccurred())
	defer func() {
		err = testEnv.Stop()
		g.Expect(err).ToNot(HaveOccurred())
	}()
	err = v1beta1.AddToScheme(k8sscheme.Scheme)
	g.Expect(err).ToNot(HaveOccurred())
	err = v1beta2.AddToScheme(k8sscheme.Scheme)
	g.Expect(err).ToNot(HaveOccurred())
	m, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).ToNot(HaveOccurred())

	var configUpdate int
	var mutex sync.Mutex
	oldRequestHandler := requestHandler
	defer func() { requestHandler = oldRequestHandler }()

	requestHandler = func(r *ConfigReconciler, ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
		mutex.Lock()
		defer mutex.Unlock()
		configUpdate++
		return ctrl.Result{}, nil
	}

	r := &ConfigReconciler{
		Client:         m.GetClient(),
		Logger:         log.NewNopLogger(),
		Scheme:         scheme,
		Namespace:      testNamespace,
		ValidateConfig: config.DontValidate,
	}
	err = r.SetupWithManager(m)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()
	go func() {
		err = m.Start(ctx)
		g.Expect(err).ToNot(HaveOccurred())
	}()

	// count for update on namespace events
	var initialConfigUpdateCount int
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Second)
		mutex.Lock()
		initialConfigUpdateCount = configUpdate
		mutex.Unlock()
	}()
	wg.Wait()
	// test new node event.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
		Spec:       corev1.NodeSpec{},
	}
	node.Labels = make(map[string]string)
	node.Labels["test"] = "e2e"
	err = m.GetClient().Create(ctx, node)
	g.Expect(err).ToNot(HaveOccurred())
	g.Eventually(func() int {
		mutex.Lock()
		defer mutex.Unlock()
		return configUpdate
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(initialConfigUpdateCount + 1))

	// test update node event with no changes into node label.
	g.Eventually(func() error {
		err = m.GetClient().Get(ctx, types.NamespacedName{Name: "test-node"}, node)
		if err != nil {
			return err
		}
		node.Labels = make(map[string]string)
		node.Spec.PodCIDR = "192.168.10.0/24"
		node.Labels["test"] = "e2e"
		err = m.GetClient().Update(ctx, node)
		if err != nil {
			return err
		}
		return nil
	}, 5*time.Second, 200*time.Millisecond).Should(BeNil())
	g.Eventually(func() int {
		mutex.Lock()
		defer mutex.Unlock()
		return configUpdate
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(initialConfigUpdateCount + 1))

	// test update node event with changes into node label.
	g.Eventually(func() error {
		err = m.GetClient().Get(ctx, types.NamespacedName{Name: "test-node"}, node)
		if err != nil {
			return err
		}
		node.Labels = make(map[string]string)
		node.Labels["test"] = "e2e"
		node.Labels["test"] = "update"
		err = m.GetClient().Update(ctx, node)
		if err != nil {
			return err
		}
		return nil
	}, 5*time.Second, 200*time.Millisecond).Should(BeNil())
	g.Eventually(func() int {
		mutex.Lock()
		defer mutex.Unlock()
		return configUpdate
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(initialConfigUpdateCount + 2))
}

var (
	testNamespace                  = "test-controller"
	scheme                         = runtime.NewScheme()
	configControllerValidResources = metallbcfg.ClusterResources{
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
				Data: map[string][]byte{"password": []byte([]byte("nopass"))}},
		},
		LegacyAddressPools: []v1beta1.AddressPool{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "legacypool1",
					Namespace: testNamespace,
				},
				Spec: v1beta1.AddressPoolSpec{
					Addresses: []string{
						"10.21.0.0/16",
					},
					Protocol: "bgp",
				},
			},
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
	configControllerInvalidResources = metallbcfg.ClusterResources{
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
