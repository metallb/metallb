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
	"time"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1beta1 "go.universe.tf/metallb/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
		desc                    string
		handlerRes              SyncState
		expectReconcileFails    bool
		initObjects             []client.Object
		expectForceReloadCalled bool
		wantCondition           metav1.Condition
	}{
		{
			desc:                    "handler returns SyncStateSuccess",
			handlerRes:              SyncStateSuccess,
			initObjects:             []client.Object{testNode},
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			wantCondition:           metav1.Condition{Status: metav1.ConditionTrue, Reason: "SyncStateSuccess"},
		},
		{
			desc:                    "handler returns SyncStateError",
			handlerRes:              SyncStateError,
			initObjects:             []client.Object{testNode},
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
			wantCondition:           metav1.Condition{Status: metav1.ConditionFalse, Reason: "ConfigError", Message: "handler failed for node test-controller/testNode: " + errRetry.Error()},
		},
		{
			desc:                    "handler returns SyncStateErrorNoRetry",
			handlerRes:              SyncStateErrorNoRetry,
			initObjects:             []client.Object{testNode},
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			wantCondition:           metav1.Condition{Status: metav1.ConditionFalse, Reason: "SyncStateErrorNoRetry"},
		},
		{
			desc:                    "handler returns SyncStateReprocessAll",
			handlerRes:              SyncStateReprocessAll,
			initObjects:             []client.Object{testNode},
			expectReconcileFails:    false,
			expectForceReloadCalled: true,
			wantCondition:           metav1.Condition{Status: metav1.ConditionTrue, Reason: "SyncStateReprocessAll"},
		},
	}
	for _, test := range tests {
		initObjects := make([]client.Object, len(test.initObjects))
		copy(initObjects, test.initObjects)
		initObjects = append(initObjects, &v1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "config-status",
				Namespace: testNamespace,
			},
		})
		fakeClient, err := newFakeClient(initObjects)
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

		calledForceReload := false
		mockForceReload := func() { calledForceReload = true }

		r := &NodeReconciler{
			Client:          fakeClient,
			Logger:          log.NewNopLogger(),
			Scheme:          scheme.Scheme,
			NodeName:        testNodeName,
			ConfigStatusRef: types.NamespacedName{Name: "config-status", Namespace: testNamespace},
			Handler:         mockHandler,
			ForceReload:     mockForceReload,
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

		if test.expectForceReloadCalled != calledForceReload {
			t.Errorf("test %s failed: call force reload expected: %v, got: %v", test.desc, test.expectForceReloadCalled, calledForceReload)
		}

		configStatus := &v1beta1.ConfigurationState{}
		if err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: "config-status", Namespace: testNamespace}, configStatus); err != nil {
			t.Errorf("test %s: failed to get ConfigurationState: %v", test.desc, err)
			continue
		}

		const conditionType = "speaker-testNode/nodeReconcilerValid"
		gotCondition := meta.FindStatusCondition(configStatus.Status.Conditions, conditionType)
		if gotCondition == nil {
			t.Errorf("test %s: condition %q not found in ConfigurationState", test.desc, conditionType)
			continue
		}

		opts := cmpopts.IgnoreFields(metav1.Condition{}, "Type", "LastTransitionTime", "ObservedGeneration")
		if diff := cmp.Diff(test.wantCondition, *gotCondition, opts); diff != "" {
			t.Errorf("test %s: condition mismatch (-want +got):\n%s", test.desc, diff)
		}
	}
}

func TestNodeReconcilerPredicate(t *testing.T) {
	t.Parallel()

	p := NodeReconcilerPredicate()

	t.Run("allow delete event pass", func(t *testing.T) {
		t.Parallel()
		if got, expected := p.Delete(event.DeleteEvent{}), true; got != expected {
			t.Fatalf("p.Create(event=%+v) returned %v; expected %v", "any", got, expected)
		}
	})

	t.Run("allow create event pass", func(t *testing.T) {
		t.Parallel()
		if got, expected := p.Create(event.CreateEvent{}), true; got != expected {
			t.Fatalf("p.Create(event=%+v) returned %v; expected %v", "any", got, expected)
		}
	})

	tests := map[string]struct {
		event    event.UpdateEvent
		expected bool
	}{
		"default": {
			event: event.UpdateEvent{
				ObjectOld: &corev1.Node{},
				ObjectNew: &corev1.Node{},
			},
			expected: false,
		},
		"wrong event object old": {
			event: event.UpdateEvent{
				ObjectOld: &corev1.Pod{},
				ObjectNew: &corev1.Node{},
			},
			expected: false,
		},
		"wrong event object new": {
			event: event.UpdateEvent{
				ObjectOld: &corev1.Node{},
				ObjectNew: &corev1.Pod{},
			},
			expected: false,
		},
		"label change": {
			event: event.UpdateEvent{
				ObjectOld: &corev1.Node{},
				ObjectNew: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"x": "y"}}}},
			expected: true,
		},
		"spec schedulable change": {
			event: event.UpdateEvent{
				ObjectOld: &corev1.Node{
					Spec: corev1.NodeSpec{Unschedulable: false},
				},
				ObjectNew: &corev1.Node{
					Spec: corev1.NodeSpec{Unschedulable: true},
				},
			},
			expected: false,
		},
		"condition NodeNetworkUnavailable status change": {
			event: event.UpdateEvent{
				ObjectOld: &corev1.Node{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse}},
					},
				},
				ObjectNew: &corev1.Node{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue}},
					},
				},
			},
			expected: true,
		},
		"condition other change": {
			event: event.UpdateEvent{
				ObjectOld: &corev1.Node{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable,
							LastHeartbeatTime: metav1.Time{Time: time.Now()}}},
					},
				},
				ObjectNew: &corev1.Node{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable,
							LastHeartbeatTime: metav1.Time{Time: time.Now().Add(10 * time.Second)}}},
					},
				},
			},
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			t.Log(name)
			if got, expected := p.Update(test.event), test.expected; got != expected {
				t.Fatalf("p.Update(event=%+v) returned %v; expected %v", name, got, expected)
			}
		})
	}
}
