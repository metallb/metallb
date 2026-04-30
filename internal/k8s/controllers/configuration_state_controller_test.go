// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"testing"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestConfigurationStateReconciler(t *testing.T) {
	const (
		configStateName      = "speaker-node1"
		configStateNamespace = "metallb-system"
	)

	ownerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb-speaker-abc123",
			Namespace: configStateNamespace,
			UID:       "pod-uid-12345",
		},
	}

	configStateObjectMeta := metav1.ObjectMeta{
		Name:      configStateName,
		Namespace: configStateNamespace,
	}

	tests := map[string]struct {
		before *metallbv1beta1.ConfigurationState
		want   *metallbv1beta1.ConfigurationState
	}{
		"create resource if not found": {
			before: nil,
			want: &metallbv1beta1.ConfigurationState{
				ObjectMeta: configStateObjectMeta,
				Status: metallbv1beta1.ConfigurationStateStatus{
					Result:       metallbv1beta1.ConfigurationResultUnknown,
					ErrorSummary: "",
				},
			},
		},
		"no conditions reported": {
			before: &metallbv1beta1.ConfigurationState{
				ObjectMeta: configStateObjectMeta,
			},
			want: &metallbv1beta1.ConfigurationState{
				ObjectMeta: configStateObjectMeta,
				Status: metallbv1beta1.ConfigurationStateStatus{
					Result:       metallbv1beta1.ConfigurationResultUnknown,
					ErrorSummary: "",
				},
			},
		},
		"all conditions true": {
			before: &metallbv1beta1.ConfigurationState{
				ObjectMeta: configStateObjectMeta,
				Status: metallbv1beta1.ConfigurationStateStatus{
					Conditions: []metav1.Condition{
						{
							Type:    ConditionTypeConfigReconcilerValid,
							Status:  metav1.ConditionTrue,
							Reason:  "Valid",
							Message: "",
						},
						{
							Type:    "testCondition",
							Status:  metav1.ConditionTrue,
							Reason:  "Valid",
							Message: "",
						},
					},
				},
			},
			want: &metallbv1beta1.ConfigurationState{
				ObjectMeta: configStateObjectMeta,
				Status: metallbv1beta1.ConfigurationStateStatus{
					Result:       metallbv1beta1.ConfigurationResultValid,
					ErrorSummary: "",
				},
			},
		},
		"one condition false": {
			before: &metallbv1beta1.ConfigurationState{
				ObjectMeta: configStateObjectMeta,
				Status: metallbv1beta1.ConfigurationStateStatus{
					Conditions: []metav1.Condition{
						{
							Type:    ConditionTypeConfigReconcilerValid,
							Status:  metav1.ConditionFalse,
							Reason:  ErrorTypeConfiguration,
							Message: "peer peer1 referencing non existing bfd profile",
						},
						{
							Type:    "testCondition",
							Status:  metav1.ConditionTrue,
							Reason:  "Valid",
							Message: "",
						},
					},
				},
			},
			want: &metallbv1beta1.ConfigurationState{
				ObjectMeta: configStateObjectMeta,
				Status: metallbv1beta1.ConfigurationStateStatus{
					Result:       metallbv1beta1.ConfigurationResultInvalid,
					ErrorSummary: "peer peer1 referencing non existing bfd profile",
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := metallbv1beta1.AddToScheme(scheme); err != nil {
				t.Fatalf("failed to add scheme: %v", err)
			}
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatalf("failed to add scheme: %v", err)
			}

			builder := fake.NewClientBuilder().WithScheme(scheme)
			if test.before != nil {
				builder = builder.WithObjects(test.before).WithStatusSubresource(test.before)
			}
			fakeClient := builder.Build()

			reconciler := &ConfigurationStateReconciler{
				Client:          fakeClient,
				Logger:          log.NewNopLogger(),
				Scheme:          scheme,
				Namespace:       configStateNamespace,
				ConfigStateName: configStateName,
				OwnerPod:        ownerPod,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      configStateName,
					Namespace: configStateNamespace,
				},
			}

			gotResult, gotErr := reconciler.Reconcile(context.Background(), req)
			if gotErr != nil {
				t.Fatalf("expected no error, got: %v", gotErr)
			}

			wantResult := reconcile.Result{}
			if diff := cmp.Diff(wantResult, gotResult); diff != "" {
				t.Fatalf("Reconcile result mismatch (-want +got):\n%s", diff)
			}

			var got metallbv1beta1.ConfigurationState
			if err := fakeClient.Get(context.Background(), req.NamespacedName, &got); err != nil {
				t.Fatalf("failed to get ConfigurationState: %v", err)
			}

			opts := []cmp.Option{
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields", "OwnerReferences"),
				cmpopts.IgnoreFields(metallbv1beta1.ConfigurationStateStatus{}, "Conditions"),
			}

			if diff := cmp.Diff(test.want, &got, opts...); diff != "" {
				t.Errorf("ConfigurationState mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConfigurationStateReconcilerOwnerReference(t *testing.T) {
	const (
		configStateName      = "speaker-node1"
		configStateNamespace = "metallb-system"
	)

	ownerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb-speaker-abc123",
			Namespace: configStateNamespace,
			UID:       "pod-uid-12345",
		},
	}

	scheme := runtime.NewScheme()
	if err := metallbv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      configStateName,
			Namespace: configStateNamespace,
		},
	}

	assertOwnerRef := func(t *testing.T, got *metallbv1beta1.ConfigurationState) {
		t.Helper()
		if len(got.OwnerReferences) != 1 {
			t.Fatalf("expect 1 owner reference, but got %d", len(got.OwnerReferences))
		}
		if got.OwnerReferences[0].UID != ownerPod.UID {
			t.Errorf("owner reference is not speaker pod, expect owner reference uid %s, but got %s", ownerPod.UID, got.OwnerReferences[0].UID)
		}
	}

	t.Run("set on create", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		reconciler := &ConfigurationStateReconciler{
			Client:          fakeClient,
			Logger:          log.NewNopLogger(),
			Scheme:          scheme,
			Namespace:       configStateNamespace,
			ConfigStateName: configStateName,
			OwnerPod:        ownerPod,
		}

		_, err := reconciler.Reconcile(context.Background(), req)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		var got metallbv1beta1.ConfigurationState
		if err := fakeClient.Get(context.Background(), req.NamespacedName, &got); err != nil {
			t.Fatalf("failed to get ConfigurationState: %v", err)
		}
		assertOwnerRef(t, &got)
	})

	t.Run("adopted on update", func(t *testing.T) {
		existing := &metallbv1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configStateName,
				Namespace: configStateNamespace,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(existing).WithStatusSubresource(existing).Build()

		reconciler := &ConfigurationStateReconciler{
			Client:          fakeClient,
			Logger:          log.NewNopLogger(),
			Scheme:          scheme,
			Namespace:       configStateNamespace,
			ConfigStateName: configStateName,
			OwnerPod:        ownerPod,
		}

		_, err := reconciler.Reconcile(context.Background(), req)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		var got metallbv1beta1.ConfigurationState
		if err := fakeClient.Get(context.Background(), req.NamespacedName, &got); err != nil {
			t.Fatalf("failed to get ConfigurationState: %v", err)
		}
		assertOwnerRef(t, &got)
	})
}

func TestNewConfigStateConditionReporterPredicate(t *testing.T) {
	const (
		targetNamespace = "metallb-system"
		targetName      = "controller"
	)

	p := NewConfigStateConditionReporterPredicate(targetNamespace, targetName)

	tests := map[string]struct {
		event any
		want  bool
	}{
		"Create matching CR": {
			event: event.CreateEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: true,
		},
		"Create different name": {
			event: event.CreateEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other",
						Namespace: targetNamespace,
					},
				},
			},
			want: false,
		},
		"Create different namespace": {
			event: event.CreateEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: "other-namespace",
					},
				},
			},
			want: false,
		},
		"Update always returns false": {
			event: event.UpdateEvent{
				ObjectOld: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: false,
		},
		"Delete matching CR": {
			event: event.DeleteEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: false,
		},
		"Generic matching CR": {
			event: event.GenericEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var got bool
			switch e := test.event.(type) {
			case event.CreateEvent:
				got = p.Create(e)
			case event.UpdateEvent:
				got = p.Update(e)
			case event.DeleteEvent:
				got = p.Delete(e)
			case event.GenericEvent:
				got = p.Generic(e)
			default:
				t.Fatalf("unknown event type: %T", e)
			}

			if got != test.want {
				t.Errorf("predicate returned %v, want %v", got, test.want)
			}
		})
	}
}

func TestNewConfigurationStateReconcilerPredicate(t *testing.T) {
	const (
		targetNamespace = "metallb-system"
		targetName      = "controller"
	)

	predicate := NewConfigurationStateReconcilerPredicate(targetNamespace, targetName)

	tests := map[string]struct {
		event any
		want  bool
	}{
		"CreateFunc matching CR": {
			event: event.CreateEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: true,
		},
		"CreateFunc different name": {
			event: event.CreateEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other",
						Namespace: targetNamespace,
					},
				},
			},
			want: false,
		},
		"CreateFunc different namespace": {
			event: event.CreateEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: "other-namespace",
					},
				},
			},
			want: false,
		},
		"UpdateFunc matching CR": {
			event: event.UpdateEvent{
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: true,
		},
		"UpdateFunc different name": {
			event: event.UpdateEvent{
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other",
						Namespace: targetNamespace,
					},
				},
			},
			want: false,
		},
		"DeleteFunc matching CR": {
			event: event.DeleteEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: true,
		},
		"DeleteFunc different name": {
			event: event.DeleteEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other",
						Namespace: targetNamespace,
					},
				},
			},
			want: false,
		},
		"GenericFunc matching CR": {
			event: event.GenericEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: true,
		},
		"GenericFunc different namespace": {
			event: event.GenericEvent{
				Object: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: "other-namespace",
					},
				},
			},
			want: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var got bool
			switch e := test.event.(type) {
			case event.CreateEvent:
				got = predicate.Create(e)
			case event.UpdateEvent:
				got = predicate.Update(e)
			case event.DeleteEvent:
				got = predicate.Delete(e)
			case event.GenericEvent:
				got = predicate.Generic(e)
			default:
				t.Fatalf("unknown event type: %T", e)
			}

			if got != test.want {
				t.Errorf("predicate returned %v, want %v", got, test.want)
			}
		})
	}
}
