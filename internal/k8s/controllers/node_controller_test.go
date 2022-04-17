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
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
