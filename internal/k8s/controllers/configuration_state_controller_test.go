// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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

	wantObjectMeta := metav1.ObjectMeta{
		Name:      configStateName,
		Namespace: configStateNamespace,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       ownerPod.Name,
				UID:        ownerPod.UID,
			},
		},
	}

	tests := map[string]struct {
		before *metallbv1beta1.ConfigurationState
		want   *metallbv1beta1.ConfigurationState
	}{
		"create resource if not found": {
			before: nil,
			want: &metallbv1beta1.ConfigurationState{
				ObjectMeta: wantObjectMeta,
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
				ObjectMeta: wantObjectMeta,
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
				ObjectMeta: wantObjectMeta,
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
				ObjectMeta: wantObjectMeta,
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
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields"),
				cmpopts.IgnoreFields(metallbv1beta1.ConfigurationStateStatus{}, "Conditions"),
			}

			if diff := cmp.Diff(test.want, &got, opts...); diff != "" {
				t.Errorf("ConfigurationState mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewConfigStateConditionReporterPredicate(t *testing.T) {
	const (
		targetNamespace = "metallb-system"
		targetName      = "controller"
	)

	p := NewConfigStateConditionReporterPredicate(targetNamespace, targetName, ConditionTypePoolReconcilerValid)

	valid := metav1.Condition{Type: ConditionTypePoolReconcilerValid, Status: metav1.ConditionTrue, Reason: ErrorTypeNone}
	invalid := metav1.Condition{Type: ConditionTypePoolReconcilerValid, Status: metav1.ConditionFalse, Reason: ErrorTypeConfiguration, Message: "broken"}
	otherValid := metav1.Condition{Type: ConditionTypeConfigReconcilerValid, Status: metav1.ConditionTrue, Reason: ErrorTypeNone}
	otherInvalid := metav1.Condition{Type: ConditionTypeConfigReconcilerValid, Status: metav1.ConditionFalse, Reason: ErrorTypeConfiguration, Message: "broken"}
	configState := func(name string, conditions ...metav1.Condition) *metallbv1beta1.ConfigurationState {
		return &metallbv1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: targetNamespace},
			Status:     metallbv1beta1.ConfigurationStateStatus{Conditions: conditions},
		}
	}
	withResult := func(state *metallbv1beta1.ConfigurationState, result metallbv1beta1.ConfigurationResult) *metallbv1beta1.ConfigurationState {
		state.Status.Result = result
		return state
	}

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
		"Update changing the condition": {
			event: event.UpdateEvent{ObjectOld: configState(targetName, valid), ObjectNew: configState(targetName, invalid)},
			want:  true,
		},
		"Update adding the condition": {
			event: event.UpdateEvent{ObjectOld: configState(targetName), ObjectNew: configState(targetName, valid)},
			want:  true,
		},
		"Update removing the condition": {
			event: event.UpdateEvent{ObjectOld: configState(targetName, valid, otherValid), ObjectNew: configState(targetName, otherValid)},
			want:  true,
		},
		"Update keeping the condition": {
			event: event.UpdateEvent{
				ObjectOld: configState(targetName, valid),
				ObjectNew: withResult(configState(targetName, valid), metallbv1beta1.ConfigurationResultValid),
			},
			want: false,
		},
		"Update changing another condition": {
			event: event.UpdateEvent{ObjectOld: configState(targetName, valid, otherValid), ObjectNew: configState(targetName, valid, otherInvalid)},
			want:  false,
		},
		"Update changing the condition of a different CR": {
			event: event.UpdateEvent{ObjectOld: configState("other", valid), ObjectNew: configState("other", invalid)},
			want:  false,
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

func TestReportConfigStateCondition(t *testing.T) {
	key := types.NamespacedName{Name: "speaker-node1", Namespace: testNamespace}
	oneHourAgo := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))

	valid := metav1.Condition{
		Type:   ConditionTypeConfigReconcilerValid,
		Status: metav1.ConditionTrue,
		Reason: ErrorTypeNone,
	}
	invalid := metav1.Condition{
		Type:    ConditionTypeConfigReconcilerValid,
		Status:  metav1.ConditionFalse,
		Reason:  ErrorTypeConfiguration,
		Message: "configuration error: peer1 is broken",
	}
	invalidOtherMessage := invalid
	invalidOtherMessage.Message = "configuration error: peer2 is broken"
	invalidOtherReason := invalidOtherMessage
	invalidOtherReason.Reason = ErrorTypeUnknown

	newConfigState := func(conditions ...metav1.Condition) *metallbv1beta1.ConfigurationState {
		return &metallbv1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			Status:     metallbv1beta1.ConfigurationStateStatus{Conditions: conditions},
		}
	}

	// newClient returns a fake client holding the given objects, wrapped by
	// funcs, and a counter of the status writes sent through it.
	newClient := func(t *testing.T, funcs interceptor.Funcs, objects ...client.Object) (client.WithWatch, *int) {
		t.Helper()
		fakeClient, err := newFakeClient(objects)
		if err != nil {
			t.Fatalf("failed to create fake client: %v", err)
		}
		writes := 0
		return withStatusWriteCounter(interceptor.NewClient(fakeClient, funcs), &writes), &writes
	}

	report := func(t *testing.T, c client.Client, condition metav1.Condition) {
		t.Helper()
		if err := reportConfigStateCondition(context.Background(), c, key, "configReconciler", condition); err != nil {
			t.Fatalf("failed to report condition: %v", err)
		}
	}

	get := func(t *testing.T, c client.Client) *metallbv1beta1.ConfigurationState {
		t.Helper()
		configState := &metallbv1beta1.ConfigurationState{}
		if err := c.Get(context.Background(), key, configState); err != nil {
			t.Fatalf("failed to get ConfigurationState: %v", err)
		}
		return configState
	}

	getCondition := func(t *testing.T, c client.Client, conditionType string) metav1.Condition {
		t.Helper()
		condition := meta.FindStatusCondition(get(t, c).Status.Conditions, conditionType)
		if condition == nil {
			t.Fatalf("condition %s not found", conditionType)
		}
		return *condition
	}

	expectWrites := func(t *testing.T, got *int, want int) {
		t.Helper()
		if *got != want {
			t.Fatalf("expected %d status writes, got %d", want, *got)
		}
	}

	t.Run("unchanged condition is written once", func(t *testing.T) {
		c, writes := newClient(t, interceptor.Funcs{}, newConfigState())

		report(t, c, valid)
		expectWrites(t, writes, 1)
		before := get(t, c)

		report(t, c, valid)
		report(t, c, valid)
		expectWrites(t, writes, 1)

		after := get(t, c)
		if after.ResourceVersion != before.ResourceVersion {
			t.Fatalf("expected resourceVersion %s, got %s", before.ResourceVersion, after.ResourceVersion)
		}
		if diff := cmp.Diff(before.Status.Conditions, after.Status.Conditions); diff != "" {
			t.Fatalf("conditions changed (-before +after):\n%s", diff)
		}
	})

	t.Run("LastTransitionTime moves only when the status changes", func(t *testing.T) {
		stored := valid
		stored.LastTransitionTime = oneHourAgo
		c, writes := newClient(t, interceptor.Funcs{}, newConfigState(stored))

		report(t, c, valid)
		expectWrites(t, writes, 0)

		report(t, c, invalid)
		report(t, c, invalid)
		expectWrites(t, writes, 1)
		got := getCondition(t, c, invalid.Type)
		if !got.LastTransitionTime.After(oneHourAgo.Time) {
			t.Fatalf("expected LastTransitionTime after %v, got %v", oneHourAgo, got.LastTransitionTime)
		}
		if diff := cmp.Diff(invalid, got, cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime")); diff != "" {
			t.Fatalf("condition mismatch (-want +got):\n%s", diff)
		}

		report(t, c, valid)
		expectWrites(t, writes, 2)
		got = getCondition(t, c, valid.Type)
		if got.Status != metav1.ConditionTrue || got.LastTransitionTime.Equal(&oneHourAgo) {
			t.Fatalf("expected a new True condition, got %+v", got)
		}
	})

	t.Run("LastTransitionTime is kept when only the message or reason changes", func(t *testing.T) {
		stored := invalid
		stored.LastTransitionTime = oneHourAgo
		c, writes := newClient(t, interceptor.Funcs{}, newConfigState(stored))

		for i, condition := range []metav1.Condition{invalidOtherMessage, invalidOtherReason} {
			report(t, c, condition)
			expectWrites(t, writes, 1+i)
			got := getCondition(t, c, condition.Type)
			if diff := cmp.Diff(condition, got, cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime")); diff != "" {
				t.Fatalf("condition mismatch (-want +got):\n%s", diff)
			}
			if !got.LastTransitionTime.Equal(&oneHourAgo) {
				t.Fatalf("expected LastTransitionTime %v, got %v", oneHourAgo, got.LastTransitionTime)
			}
		}
	})

	t.Run("other conditions are kept", func(t *testing.T) {
		other := metav1.Condition{
			Type:               ConditionTypePoolReconcilerValid,
			Status:             metav1.ConditionTrue,
			Reason:             ErrorTypeNone,
			LastTransitionTime: oneHourAgo,
		}
		c, writes := newClient(t, interceptor.Funcs{}, newConfigState(other))

		report(t, c, invalid)
		expectWrites(t, writes, 1)
		if diff := cmp.Diff(other, getCondition(t, c, other.Type)); diff != "" {
			t.Fatalf("other condition changed (-want +got):\n%s", diff)
		}
	})

	t.Run("missing resource gets the condition once created", func(t *testing.T) {
		c, writes := newClient(t, interceptor.Funcs{})

		// The reporter runs before the resource is created: nothing to write,
		// the creation triggers a new report.
		report(t, c, valid)
		expectWrites(t, writes, 0)

		if err := c.Create(context.Background(), newConfigState()); err != nil {
			t.Fatalf("failed to create ConfigurationState: %v", err)
		}
		report(t, c, valid)
		expectWrites(t, writes, 1)
		getCondition(t, c, valid.Type)
	})

	t.Run("stale read does not overwrite the resource", func(t *testing.T) {
		var stale *metallbv1beta1.ConfigurationState
		// Mimics a cache that has not observed the last writes yet.
		c, writes := newClient(t, interceptor.Funcs{
			Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if configState, ok := obj.(*metallbv1beta1.ConfigurationState); ok && stale != nil {
					stale.DeepCopyInto(configState)
					return nil
				}
				return cl.Get(ctx, key, obj, opts...)
			},
		}, newConfigState())

		report(t, c, valid)
		validState := get(t, c)
		report(t, c, invalid)
		expectWrites(t, writes, 2)

		stale = validState
		err := reportConfigStateCondition(context.Background(), c, key, "configReconciler", invalidOtherMessage)
		if !apierrors.IsConflict(err) {
			t.Fatalf("expected a conflict, got %v", err)
		}
		stale = nil
		if diff := cmp.Diff(invalid, getCondition(t, c, invalid.Type), cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime")); diff != "" {
			t.Fatalf("condition mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("failed read is returned without writing", func(t *testing.T) {
		c, writes := newClient(t, interceptor.Funcs{
			Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return errors.New("injected get error")
			},
		}, newConfigState())

		if err := reportConfigStateCondition(context.Background(), c, key, "configReconciler", valid); err == nil {
			t.Fatalf("expected the get error to be returned")
		}
		expectWrites(t, writes, 0)
	})
}
