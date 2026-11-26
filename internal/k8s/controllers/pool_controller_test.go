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
	metallbcfg "go.universe.tf/metallb/internal/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestPoolController(t *testing.T) {
	tests := []struct {
		desc                    string
		handlerRes              SyncState
		validResources          bool
		expectReconcileFails    bool
		expectForceReloadCalled bool
		wantConditions          []v1.Condition
	}{
		{
			desc:                    "handler returns SyncStateSuccess, valid resources",
			handlerRes:              SyncStateSuccess,
			validResources:          true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			wantConditions: []v1.Condition{
				{
					Type:   ConditionTypePoolReconcilerValid,
					Status: v1.ConditionTrue,
					Reason: ErrorTypeNone,
				},
			},
		},
		{
			desc:                    "handler returns SyncStateError, valid resources",
			handlerRes:              SyncStateError,
			validResources:          true,
			expectReconcileFails:    true,
			expectForceReloadCalled: false,
			wantConditions: []v1.Condition{
				{
					Type:    ConditionTypePoolReconcilerValid,
					Status:  v1.ConditionFalse,
					Reason:  ErrorTypeConfiguration,
					Message: "configuration error: general handler sync state error",
				},
			},
		},
		{
			desc:                    "handler returns SyncStateErrorNoRetry, valid resources",
			handlerRes:              SyncStateErrorNoRetry,
			validResources:          true,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			wantConditions: []v1.Condition{
				{
					Type:    ConditionTypePoolReconcilerValid,
					Status:  v1.ConditionFalse,
					Reason:  ErrorTypeConfiguration,
					Message: "configuration error: general handler sync state error",
				},
			},
		},
		{
			desc:                    "handler returns SyncStateReprocessAll, valid resources",
			handlerRes:              SyncStateReprocessAll,
			validResources:          true,
			expectReconcileFails:    false,
			expectForceReloadCalled: true,
			wantConditions: []v1.Condition{
				{
					Type:    ConditionTypePoolReconcilerValid,
					Status:  v1.ConditionTrue,
					Reason:  ErrorTypeNone,
					Message: "",
				},
			},
		},
		{
			desc:                    "handler returns SyncStateSuccess, invalid resources",
			handlerRes:              SyncStateSuccess,
			validResources:          false,
			expectReconcileFails:    false,
			expectForceReloadCalled: false,
			wantConditions: []v1.Condition{
				{
					Type:    ConditionTypePoolReconcilerValid,
					Status:  v1.ConditionFalse,
					Reason:  ErrorTypeConfiguration,
					Message: "configuration error: parsing address pool pool1: pool has no prefixes defined",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var resources metallbcfg.ClusterResources
			if test.validResources {
				resources = poolControllerValidResources
			} else {
				resources = poolControllerInvalidResources
			}

			initObjects := objectsFromResources(resources)

			configStateRef := &v1beta1.ConfigurationState{
				ObjectMeta: v1.ObjectMeta{
					Name:      "controller",
					Namespace: testNamespace,
				},
			}
			initObjects = append(initObjects, configStateRef)

			fakeClient, err := newFakeClient(initObjects)
			if err != nil {
				t.Fatalf("test %s failed to create fake client: %v", test.desc, err)
			}

			expectedCfg, err := metallbcfg.For(resources, metallbcfg.DontValidate, metallbcfg.ForOptions{})
			if err != nil && test.validResources {
				t.Fatalf("test %s failed to create config, got unexpected error: %v", test.desc, err)
			}

			cmpOpt := cmpopts.IgnoreUnexported(metallbcfg.Pool{})

			mockHandler := func(l log.Logger, pools *metallbcfg.Pools) SyncState {
				if !cmp.Equal(expectedCfg.Pools, pools, cmpOpt) {
					t.Errorf("test %s failed, handler called with unexpected config: %s", test.desc, cmp.Diff(expectedCfg.Pools, pools, cmpOpt))
				}
				return test.handlerRes
			}

			calledForceReload := false
			mockForceReload := func() { calledForceReload = true }

			r := &PoolReconciler{
				Client:          fakeClient,
				Logger:          log.NewNopLogger(),
				Scheme:          scheme.Scheme,
				Namespace:       testNamespace,
				ValidateConfig:  metallbcfg.DontValidate,
				Handler:         mockHandler,
				ForceReload:     mockForceReload,
				ConfigStateName: configStateRef.Name,
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

			var gotConfigState v1beta1.ConfigurationState
			if err := fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      configStateRef.Name,
				Namespace: configStateRef.Namespace,
			}, &gotConfigState); err != nil {
				t.Fatalf("%s: failed to get ConfigurationState: %v", test.desc, err)
			}

			opts := cmpopts.IgnoreFields(v1.Condition{}, "LastTransitionTime")
			if diff := cmp.Diff(test.wantConditions, gotConfigState.Status.Conditions, opts); diff != "" {
				t.Errorf("%s: conditions mismatch (-want +got):\n%s", test.desc, diff)
			}
		})
	}
}

var (
	poolControllerValidResources = metallbcfg.ClusterResources{
		Pools: []v1beta1.IPAddressPool{
			{
				ObjectMeta: v1.ObjectMeta{
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
		Communities: []v1beta1.Community{
			{
				ObjectMeta: v1.ObjectMeta{
					Name:      "community",
					Namespace: testNamespace,
				},
				Spec: v1beta1.CommunitySpec{
					Communities: []v1beta1.CommunityAlias{
						{
							Name:  "bar",
							Value: "1234:4567",
						},
					},
				},
			},
		},
	}

	poolControllerInvalidResources = metallbcfg.ClusterResources{
		Pools: []v1beta1.IPAddressPool{
			{
				ObjectMeta: v1.ObjectMeta{
					Name:      "pool1",
					Namespace: testNamespace,
				},
			},
		},
	}
)

func TestPoolControllerReportsOnlyConditionChanges(t *testing.T) {
	configStateRef := &v1beta1.ConfigurationState{
		ObjectMeta: v1.ObjectMeta{
			Name:      "controller",
			Namespace: testNamespace,
		},
	}
	initObjects := append(objectsFromResources(poolControllerValidResources), configStateRef)
	fakeClient, err := newFakeClient(initObjects)
	if err != nil {
		t.Fatalf("failed to create fake client: %v", err)
	}
	writes := 0
	c := withStatusWriteCounter(fakeClient, &writes)

	r := &PoolReconciler{
		Client:          c,
		Logger:          log.NewNopLogger(),
		Scheme:          scheme.Scheme,
		Namespace:       testNamespace,
		ValidateConfig:  metallbcfg.DontValidate,
		Handler:         func(log.Logger, *metallbcfg.Pools) SyncState { return SyncStateSuccess },
		ForceReload:     func() {},
		ConfigStateName: configStateRef.Name,
	}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace}}

	setPoolAddresses := func(addresses []string) {
		t.Helper()
		var pool v1beta1.IPAddressPool
		if err := c.Get(context.TODO(), types.NamespacedName{Name: "pool1", Namespace: testNamespace}, &pool); err != nil {
			t.Fatalf("failed to get pool: %v", err)
		}
		pool.Spec.Addresses = addresses
		if err := c.Update(context.TODO(), &pool); err != nil {
			t.Fatalf("failed to update pool: %v", err)
		}
	}

	steps := []struct {
		desc       string
		update     func()
		wantStatus v1.ConditionStatus
		wantWrites int
	}{
		{desc: "first reconcile", wantStatus: v1.ConditionTrue, wantWrites: 1},
		{desc: "valid again", wantStatus: v1.ConditionTrue, wantWrites: 1},
		{desc: "pool becomes invalid", update: func() { setPoolAddresses(nil) }, wantStatus: v1.ConditionFalse, wantWrites: 2},
		{desc: "invalid again", wantStatus: v1.ConditionFalse, wantWrites: 2},
		{desc: "pool becomes valid", update: func() { setPoolAddresses([]string{"10.20.0.0/16"}) }, wantStatus: v1.ConditionTrue, wantWrites: 3},
		{desc: "valid again after recovery", wantStatus: v1.ConditionTrue, wantWrites: 3},
	}
	for _, step := range steps {
		if step.update != nil {
			step.update()
		}
		if _, err := r.Reconcile(context.TODO(), req); err != nil {
			t.Fatalf("%s: reconcile failed: %v", step.desc, err)
		}
		if writes != step.wantWrites {
			t.Fatalf("%s: expected %d status writes, got %d", step.desc, step.wantWrites, writes)
		}
		var got v1beta1.ConfigurationState
		if err := c.Get(context.TODO(), types.NamespacedName{Name: configStateRef.Name, Namespace: testNamespace}, &got); err != nil {
			t.Fatalf("%s: failed to get ConfigurationState: %v", step.desc, err)
		}
		if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Status != step.wantStatus {
			t.Fatalf("%s: expected one condition with status %s, got %+v", step.desc, step.wantStatus, got.Status.Conditions)
		}
	}
}
