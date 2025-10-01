// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ConfigurationStateNamespace = "metallb-system"
	ConfigurationStateName      = "config-status"
)

var testObjectMeta = metav1.ObjectMeta{
	Name:      ConfigurationStateName,
	Namespace: ConfigurationStateNamespace,
}

func TestConfigurationState(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := metallbv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	testCases := map[string]struct {
		existingStatus *metallbv1beta1.ConfigurationState
		wantStatus     *metallbv1beta1.ConfigurationState
	}{
		"reconciler creates resource when it does not exist": {
			existingStatus: nil,
			wantStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:    "Ready",
							Status:  metav1.ConditionUnknown,
							Reason:  "WaitingForConditions",
							Message: "No controller conditions reported yet",
						},
					},
				},
			},
		},
		"reconciler sets Ready to Unknown when no component conditions exist": {
			existingStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{},
				},
			},
			wantStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:    "Ready",
							Status:  metav1.ConditionUnknown,
							Reason:  "WaitingForConditions",
							Message: "No controller conditions reported yet",
						},
					},
				},
			},
		},
		"reconciler sets Ready to Unknown when only Ready condition exists": {
			existingStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
							Reason: "OldReason",
						},
					},
				},
			},
			wantStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:    "Ready",
							Status:  metav1.ConditionUnknown,
							Reason:  "WaitingForConditions",
							Message: "No controller conditions reported yet",
						},
					},
				},
			},
		},

		// Ready condition aggregation tests
		"reconciler sets Ready to True when all component conditions are True": {
			existingStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:   "controller/poolReconcilerValid",
							Status: metav1.ConditionTrue,
							Reason: "SyncStateSuccess",
						},
						{
							Type:   "speaker-node1/configReconcilerValid",
							Status: metav1.ConditionTrue,
							Reason: "SyncStateSuccess",
						},
					},
				},
			},
			wantStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:   "controller/poolReconcilerValid",
							Status: metav1.ConditionTrue,
							Reason: "SyncStateSuccess",
						},
						{
							Type:   "speaker-node1/configReconcilerValid",
							Status: metav1.ConditionTrue,
							Reason: "SyncStateSuccess",
						},
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
							Reason: "AllComponentsReady",
						},
					},
				},
			},
		},
		"reconciler sets Ready to False when one component condition is False": {
			existingStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:    "controller/poolReconcilerValid",
							Status:  metav1.ConditionFalse,
							Reason:  "ConfigError",
							Message: "pool overlap error",
						},
						{
							Type:   "speaker-node1/configReconcilerValid",
							Status: metav1.ConditionTrue,
							Reason: "SyncStateSuccess",
						},
					},
				},
			},
			wantStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:    "controller/poolReconcilerValid",
							Status:  metav1.ConditionFalse,
							Reason:  "ConfigError",
							Message: "pool overlap error",
						},
						{
							Type:   "speaker-node1/configReconcilerValid",
							Status: metav1.ConditionTrue,
							Reason: "SyncStateSuccess",
						},
						{
							Type:    "Ready",
							Status:  metav1.ConditionFalse,
							Reason:  "ComponentFailing",
							Message: "Failed components: [controller/poolReconcilerValid]",
						},
					},
				},
			},
		},
		"reconciler sets Ready to False when multiple component conditions are False": {
			existingStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:    "controller/poolReconcilerValid",
							Status:  metav1.ConditionFalse,
							Reason:  "ConfigError",
							Message: "pool error",
						},
						{
							Type:    "speaker-node1/configReconcilerValid",
							Status:  metav1.ConditionFalse,
							Reason:  "SyncStateError",
							Message: "sync error",
						},
					},
				},
			},
			wantStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:    "controller/poolReconcilerValid",
							Status:  metav1.ConditionFalse,
							Reason:  "ConfigError",
							Message: "pool error",
						},
						{
							Type:    "speaker-node1/configReconcilerValid",
							Status:  metav1.ConditionFalse,
							Reason:  "SyncStateError",
							Message: "sync error",
						},
						{
							Type:    "Ready",
							Status:  metav1.ConditionFalse,
							Reason:  "ComponentsFailing",
							Message: "Failed components: [controller/poolReconcilerValid speaker-node1/configReconcilerValid]",
						},
					},
				},
			},
		},
		"reconciler sets Ready to True from component conditions only": {
			existingStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
							Reason: "OldReason",
						},
						{
							Type:   "controller/poolReconcilerValid",
							Status: metav1.ConditionTrue,
							Reason: "SyncStateSuccess",
						},
					},
				},
			},
			wantStatus: &metallbv1beta1.ConfigurationState{
				ObjectMeta: testObjectMeta,
				Status: metallbv1beta1.MetalLBConfigurationState{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
							Reason: "AllComponentsReady",
						},
						{
							Type:   "controller/poolReconcilerValid",
							Status: metav1.ConditionTrue,
							Reason: "SyncStateSuccess",
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var objs []client.Object
			if tc.existingStatus != nil {
				objs = append(objs, tc.existingStatus)
			}

			fakeClient, err := newFakeClient(objs)
			if err != nil {
				t.Fatalf("Failed to create fake client: %v", err)
			}

			reconciler := &ConfigurationStateReconciler{
				Client:          fakeClient,
				Logger:          log.NewNopLogger(),
				Scheme:          scheme,
				ConfigStatusRef: types.NamespacedName{Name: ConfigurationStateName, Namespace: ConfigurationStateNamespace},
			}

			if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: reconciler.ConfigStatusRef,
			}); err != nil {
				t.Errorf("Reconcile() error = %v", err)
				return
			}

			var gotStatus metallbv1beta1.ConfigurationState
			if err := fakeClient.Get(context.Background(), types.NamespacedName{
				Name: ConfigurationStateName, Namespace: ConfigurationStateNamespace,
			}, &gotStatus); err != nil {
				t.Errorf("Failed to get ConfigurationState: %v", err)
				return
			}

			opts := cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime", "ObservedGeneration")
			if diff := cmp.Diff(tc.wantStatus.Status, gotStatus.Status, opts); diff != "" {
				t.Errorf("Status mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
