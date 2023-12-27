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

	corev1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestServiceController(t *testing.T) {
	var (
		calledForceReload      bool
		contextTimeOutDuration = time.Millisecond * 100
		testObjectName         = "testObject"
		testService            = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testObjectName,
				Namespace: testNamespace,
			},
		}
		testEndPointSlices = &discovery.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testObjectName,
				Namespace: testNamespace,
				Labels: map[string]string{
					discovery.LabelServiceName: testObjectName,
				},
			},
		}
	)
	tests := []struct {
		desc                    string
		handlerRes              SyncState
		needEndPoints           bool
		initObjects             []client.Object
		shouldReprocessAll      bool
		expectReconcileFails    bool
		expectForceReloadCalled bool
		initialLoadPerformed    bool
	}{
		{
			desc:                    "call reconcileService, handler returns SyncStateSuccess",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateSuccess",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           true,
			initObjects:             []client.Object{testService, testEndPointSlices},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateError",
			handlerRes:              SyncStateError,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateErrorNoRetry",
			handlerRes:              SyncStateErrorNoRetry,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateReprocessAll",
			handlerRes:              SyncStateReprocessAll,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: true,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reconcileService, initialLoadPerformed initiated to false",
			handlerRes:              SyncStateReprocessAll,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			initialLoadPerformed:    false,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateSuccess",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateSuccess",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           true,
			initObjects:             []client.Object{testService, testEndPointSlices},
			shouldReprocessAll:      true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateError",
			handlerRes:              SyncStateError,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateErrorNoRetry",
			handlerRes:              SyncStateErrorNoRetry,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateReprocessAll",
			handlerRes:              SyncStateReprocessAll,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
			initialLoadPerformed:    true,
		},
		{
			desc:                    "call reprocessAll, initialLoadPerformed initiated to false",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           false,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			initialLoadPerformed:    false,
		},
	}
	for _, test := range tests {
		fakeClient, err := newFakeClient(test.initObjects)
		if err != nil {
			t.Fatalf("test %s failed to create fake client: %v", test.desc, err)
		}

		mockHandler := func(l log.Logger, serviceName string, s *corev1.Service, eps []discovery.EndpointSlice) SyncState {
			if !reflect.DeepEqual(testService.ObjectMeta, s.ObjectMeta) {
				t.Errorf("test %s failed, handler called with the wrong service (-want +got)\n%s",
					test.desc, cmp.Diff(testService.ObjectMeta, s.ObjectMeta))
			}
			if test.needEndPoints &&
				!reflect.DeepEqual(testEndPointSlices.ObjectMeta, eps[0].ObjectMeta) {
				t.Errorf("test %s failed, handler called with the wrong endpointslices (-want +got)\n%s",
					test.desc, cmp.Diff(testEndPointSlices.ObjectMeta, eps[0].ObjectMeta))
			}
			return test.handlerRes
		}

		mockReload := make(chan event.GenericEvent, 1)

		r := &ServiceReconciler{
			Client:               fakeClient,
			Logger:               log.NewNopLogger(),
			Scheme:               scheme.Scheme,
			Namespace:            testNamespace,
			Handler:              mockHandler,
			Endpoints:            test.needEndPoints,
			Reload:               mockReload,
			initialLoadPerformed: false,
		}
		r.initialLoadPerformed = test.initialLoadPerformed
		var req reconcile.Request
		if test.shouldReprocessAll {
			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "metallbreload",
					Name:      "reload",
				},
			}
		} else {
			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: testNamespace,
					Name:      testObjectName,
				},
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), contextTimeOutDuration)
		defer cancel()

		_, err = r.Reconcile(ctx, req)
		failedReconcile := err != nil

		if test.expectReconcileFails != failedReconcile {
			t.Errorf("test %s failed: fail reconcile expected: %v, got: %v. err: %v",
				test.desc, test.expectReconcileFails, failedReconcile, err)
		}

		select {
		case <-ctx.Done():
			calledForceReload = false
		case <-mockReload:
			calledForceReload = true
		}
		if test.expectForceReloadCalled != calledForceReload {
			t.Errorf("test %s failed: call force reload expected: %v, got: %v",
				test.desc, test.expectForceReloadCalled, calledForceReload)
		}
		if test.shouldReprocessAll && !r.initialLoadPerformed {
			t.Errorf("test %s failed: reconciler's initialLoadPerformed flag didn't change to true", test.desc)
		}
	}
}

func TestLBClass(t *testing.T) {
	tests := []struct {
		desc           string
		serviceLBClass *string
		metallLBClass  string
		shouldFilter   bool
	}{
		{
			desc:           "Empty serviceclass, metallb default",
			serviceLBClass: nil,
			metallLBClass:  "",
			shouldFilter:   false,
		},
		{
			desc:           "Empty serviceclass, metallb specific",
			serviceLBClass: nil,
			metallLBClass:  "foo",
			shouldFilter:   true,
		},
		{
			desc:           "Set serviceclass, metallb default",
			serviceLBClass: ptr.To[string]("foo"),
			metallLBClass:  "",
			shouldFilter:   true,
		},
		{
			desc:           "Set serviceclass, metallb different",
			serviceLBClass: ptr.To[string]("foo"),
			metallLBClass:  "bar",
			shouldFilter:   true,
		},
		{
			desc:           "Set serviceclass, metallb same",
			serviceLBClass: ptr.To[string]("foo"),
			metallLBClass:  "foo",
			shouldFilter:   false,
		},
	}
	for _, test := range tests {
		svc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				LoadBalancerClass: test.serviceLBClass,
			},
		}
		filters := filterByLoadBalancerClass(svc, test.metallLBClass)
		if filters != test.shouldFilter {
			t.Errorf("test %s failed: expected filter: %v, got: %v",
				test.desc, test.shouldFilter, filters)
		}
	}
}
