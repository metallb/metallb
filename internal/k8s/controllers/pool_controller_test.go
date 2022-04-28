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

	"github.com/go-kit/kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/config"
	metallbcfg "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/pointer"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestPoolController(t *testing.T) {
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
			resources = poolControllerValidResources
		} else {
			resources = poolControllerInvalidResources
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

		mockHandler := func(l log.Logger, pools map[string]*config.Pool) SyncState {
			if !cmp.Equal(expectedCfg.Pools, pools, cmpOpt) {
				t.Errorf("test %s failed, handler called with unexpected config: %s", test.desc, cmp.Diff(expectedCfg.Pools, pools, cmpOpt))
			}
			return test.handlerRes
		}

		calledForceReload := false
		mockForceReload := func() { calledForceReload = true }

		r := &PoolReconciler{
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
			t.Errorf("test %s failed: fail reconcile expected: %v, got: %v. err: %v", test.desc, test.expectReconcileFails, failedReconcile, err)
		}

		if test.expectForceReloadCalled != calledForceReload {
			t.Errorf("test %s failed: call force reload expected: %v, got: %v", test.desc, test.expectForceReloadCalled, calledForceReload)
		}
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
		LegacyAddressPools: []v1beta1.AddressPool{
			{
				ObjectMeta: v1.ObjectMeta{
					Name:      "legacypool1",
					Namespace: testNamespace,
				},
				Spec: v1beta1.AddressPoolSpec{
					Addresses: []string{
						"10.21.0.0/16",
					},
					Protocol: "bgp",
					BGPAdvertisements: []v1beta1.LegacyBgpAdvertisement{
						{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(100),
							Communities:       []string{"bar"},
						},
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
