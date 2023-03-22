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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1beta2 "go.universe.tf/metallb/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestNodeController(t *testing.T) {
	var testNodeName = "testNode"
	var testNode = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNodeName,
			Namespace: testNamespace,
		},
	}
	tests := []struct {
		desc                 string
		handlerRes           SyncState
		expectReconcileFails bool
		initObjects          []client.Object
	}{
		{
			desc:                 "handler returns SyncStateSuccess",
			handlerRes:           SyncStateSuccess,
			initObjects:          []client.Object{testNode},
			expectReconcileFails: false,
		},
		{
			desc:                 "handler returns SyncStateError",
			handlerRes:           SyncStateError,
			initObjects:          []client.Object{testNode},
			expectReconcileFails: true,
		},
		{
			desc:                 "handler returns SyncStateErrorNoRetry",
			handlerRes:           SyncStateErrorNoRetry,
			initObjects:          []client.Object{testNode},
			expectReconcileFails: false,
		},
		{
			desc:                 "handler returns SyncStateReprocessAll",
			handlerRes:           SyncStateReprocessAll,
			initObjects:          []client.Object{testNode},
			expectReconcileFails: false,
		},
	}
	for _, test := range tests {
		fakeClient, err := newFakeClient(test.initObjects)
		if err != nil {
			t.Fatalf("test %s failed to create fake client: %v", test.desc, err)
		}

		mockHandler := func(l log.Logger, n *corev1.Node) SyncState {
			if !reflect.DeepEqual(testNode.ObjectMeta, n.ObjectMeta) {
				t.Errorf("test %s failed, handler called with the wrong node (-want +got)\n%s",
					test.desc, cmp.Diff(testNode.ObjectMeta, n.ObjectMeta))
			}
			return test.handlerRes
		}

		r := &NodeReconciler{
			Client:    fakeClient,
			Logger:    log.NewNopLogger(),
			Scheme:    scheme,
			NodeName:  testNodeName,
			Namespace: testNamespace,
			Handler:   mockHandler,
		}
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: testNamespace,
				Name:      testNodeName,
			},
		}

		_, err = r.Reconcile(context.TODO(), req)
		failedReconcile := err != nil

		if test.expectReconcileFails != failedReconcile {
			t.Errorf("test %s failed: fail reconcile expected: %v, got: %v. err: %v",
				test.desc, test.expectReconcileFails, failedReconcile, err)
		}
	}
}

func TestNodeReconciler_SetupWithManager(t *testing.T) {
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
	mockHandler := func(l log.Logger, n *corev1.Node) SyncState {
		mutex.Lock()
		defer mutex.Unlock()
		configUpdate++
		return SyncStateSuccess
	}
	r := &NodeReconciler{
		Client:    m.GetClient(),
		Logger:    log.NewNopLogger(),
		Scheme:    scheme,
		Namespace: testNamespace,
		Handler:   mockHandler,
		NodeName:  "test-node",
	}
	err = r.SetupWithManager(m)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()
	go func() {
		err = m.Start(ctx)
		g.Expect(err).ToNot(HaveOccurred())
	}()

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
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(1))

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
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(1))

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
	}, 5*time.Second, 200*time.Millisecond).Should(Equal(2))
}
