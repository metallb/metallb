// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"testing"
	"time"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestNewConfigurationStateWriterPredicate(t *testing.T) {
	const (
		targetNamespace = "metallb-system"
		targetName      = "controller"
		watchedType     = ConditionTypeConfigReconcilerValid
	)

	now := metav1.Now()
	later := metav1.NewTime(now.Add(time.Second))

	p := newConfigurationStatePredicate(targetNamespace, targetName, watchedType)

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
			want: false, // Ignore CREATEs - controller creates this resource itself
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
		"Update matching CR, watched condition status changed": {
			event: event.UpdateEvent{
				ObjectOld: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionTrue, Reason: "Reconciled"},
						},
					},
				},
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionFalse, Reason: "ConfigurationError"},
						},
					},
				},
			},
			want: true,
		},
		"Update matching CR, watched condition added": {
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
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionTrue, Reason: "Reconciled"},
						},
					},
				},
			},
			want: false, // Ignore adding conditions - controller sets them for the first time
		},
		"Update matching CR, watched condition removed": {
			event: event.UpdateEvent{
				ObjectOld: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionTrue, Reason: "Reconciled"},
						},
					},
				},
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
				},
			},
			want: true,
		},
		"Update matching CR, only LastTransitionTime changed": {
			event: event.UpdateEvent{
				ObjectOld: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionTrue, Reason: "Reconciled", LastTransitionTime: now},
						},
					},
				},
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionTrue, Reason: "Reconciled", LastTransitionTime: later},
						},
					},
				},
			},
			want: false,
		},
		"Update matching CR, watched condition message changed": {
			event: event.UpdateEvent{
				ObjectOld: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionFalse, Reason: "ConfigurationError", Message: "old error"},
						},
					},
				},
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionFalse, Reason: "ConfigurationError", Message: "new error"},
						},
					},
				},
			},
			want: true,
		},
		"Update matching CR, different condition type changed": {
			event: event.UpdateEvent{
				ObjectOld: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionTrue, Reason: "Reconciled"},
							{Type: ConditionTypePoolReconcilerValid, Status: metav1.ConditionTrue, Reason: "Reconciled"},
						},
					},
				},
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetName,
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionTrue, Reason: "Reconciled"},
							{Type: ConditionTypePoolReconcilerValid, Status: metav1.ConditionFalse, Reason: "ConfigurationError"},
						},
					},
				},
			},
			want: false,
		},
		"Update different name": {
			event: event.UpdateEvent{
				ObjectOld: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other",
						Namespace: targetNamespace,
					},
				},
				ObjectNew: &metallbv1beta1.ConfigurationState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other",
						Namespace: targetNamespace,
					},
					Status: metallbv1beta1.ConfigurationStateStatus{
						Conditions: []metav1.Condition{
							{Type: watchedType, Status: metav1.ConditionTrue, Reason: "Reconciled"},
						},
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
			want: true, // React to deletes - controller should recreate the resource
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
