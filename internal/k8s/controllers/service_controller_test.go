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
	"go.universe.tf/metallb/internal/k8s/epslices"
	corev1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
		testEndPoint = &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testObjectName,
				Namespace: testNamespace,
			},
		}
		testEndPointSlices = &discovery.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testObjectName,
				Namespace: testNamespace,
			},
		}
	)
	tests := []struct {
		desc                    string
		handlerRes              SyncState
		needEndPoints           NeedEndPoints
		initObjects             []client.Object
		shouldReprocessAll      bool
		expectReconcileFails    bool
		expectForceReloadCalled bool
	}{
		{
			desc:                    "call reconcileService, handler returns SyncStateSuccess",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           NoNeed,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateSuccess - with endpoints",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           Endpoints,
			initObjects:             []client.Object{testService, testEndPoint},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateSuccess - with endpointSlices",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           EndpointSlices,
			initObjects:             []client.Object{testService, testEndPointSlices},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateError",
			handlerRes:              SyncStateError,
			needEndPoints:           NoNeed,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateErrorNoRetry",
			handlerRes:              SyncStateErrorNoRetry,
			needEndPoints:           NoNeed,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reconcileService, handler returns SyncStateReprocessAll",
			handlerRes:              SyncStateReprocessAll,
			needEndPoints:           NoNeed,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      false,
			expectReconcileFails:    false,
			expectForceReloadCalled: true,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateSuccess",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           NoNeed,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateSuccess - with endpoints",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           Endpoints,
			initObjects:             []client.Object{testService, testEndPoint},
			shouldReprocessAll:      true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateSuccess - with endpointSlices",
			handlerRes:              SyncStateSuccess,
			needEndPoints:           EndpointSlices,
			initObjects:             []client.Object{testService, testEndPointSlices},
			shouldReprocessAll:      true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateError",
			handlerRes:              SyncStateError,
			needEndPoints:           NoNeed,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateErrorNoRetry",
			handlerRes:              SyncStateErrorNoRetry,
			needEndPoints:           NoNeed,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
		},
		{
			desc:                    "call reprocessAll, handler returns SyncStateReprocessAll",
			handlerRes:              SyncStateReprocessAll,
			needEndPoints:           NoNeed,
			initObjects:             []client.Object{testService},
			shouldReprocessAll:      true,
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
		},
	}
	for _, test := range tests {
		fakeClient, err := newFakeClient(test.initObjects)
		if err != nil {
			t.Fatalf("test %s failed to create fake client: %v", test.desc, err)
		}

		mockHandler := func(l log.Logger, serviceName string, s *corev1.Service, e epslices.EpsOrSlices) SyncState {
			if !reflect.DeepEqual(testService.ObjectMeta, s.ObjectMeta) {
				t.Errorf("test %s failed, handler called with the wrong service (-want +got)\n%s",
					test.desc, cmp.Diff(testService.ObjectMeta, s.ObjectMeta))
			}
			if test.needEndPoints == Endpoints &&
				!reflect.DeepEqual(testEndPoint.ObjectMeta, e.EpVal.ObjectMeta) {
				t.Errorf("test %s failed, handler called with the wrong endpoints (-want +got)\n%s",
					test.desc, cmp.Diff(testEndPoint.ObjectMeta, e.EpVal.ObjectMeta))
			}
			if test.needEndPoints == EndpointSlices &&
				!reflect.DeepEqual(testEndPointSlices.ObjectMeta, e.SlicesVal[0].ObjectMeta) {
				t.Errorf("test %s failed, handler called with the wrong endpointslices (-want +got)\n%s",
					test.desc, cmp.Diff(testEndPointSlices.ObjectMeta, e.SlicesVal[0].ObjectMeta))
			}
			return test.handlerRes
		}

		mockReload := make(chan event.GenericEvent, 1)

		r := &ServiceReconciler{
			Client:    fakeClient,
			Logger:    log.NewNopLogger(),
			Scheme:    scheme,
			Namespace: testNamespace,
			Handler:   mockHandler,
			Endpoints: test.needEndPoints,
			Reload:    mockReload,
		}

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
	}
}
