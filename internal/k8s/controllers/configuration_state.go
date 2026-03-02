// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// newConfigurationStatePredicate returns a predicate for controllers that write
// a specific condition type to a ConfigurationState CR. It fires on Delete and Update
// events when the targeted ConfigurationState (matched by namespace/name) has been
// externally modified. Create events are ignored since the controller creates the resource.
// Update events only fire when the watched condition is removed or meaningfully changed
// (Status, Reason, or Message), ignoring LastTransitionTime and initial condition addition.
func newConfigurationStatePredicate(namespace, name, conditionType string) predicate.Predicate {
	matchesTarget := func(obj client.Object) bool {
		return obj.GetNamespace() == namespace && obj.GetName() == name
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Ignore CREATEs since this controller creates the ConfigurationState itself.
			// We only care about external modifications (UPDATE/DELETE).
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if matchesTarget(e.ObjectOld) || matchesTarget(e.ObjectNew) {
				changed := configurationStateConditionChanged(e.ObjectOld, e.ObjectNew, conditionType)
				return changed
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return matchesTarget(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			// Ignore Generic events - these are typically for manual triggers or resyncs
			// which the controller doesn't need to react to.
			return false
		},
	}
}

// configurationStateConditionChanged reports whether a specific condition type on
// two ConfigurationState objects differs in Status, Reason, or Message.
func configurationStateConditionChanged(oldObj, newObj client.Object, conditionType string) bool {
	oldCS, ok := oldObj.(*metallbv1beta1.ConfigurationState)
	if !ok {
		return false
	}
	newCS, ok := newObj.(*metallbv1beta1.ConfigurationState)
	if !ok {
		return false
	}

	oldCond := meta.FindStatusCondition(oldCS.Status.Conditions, conditionType)
	newCond := meta.FindStatusCondition(newCS.Status.Conditions, conditionType)

	if oldCond == nil && newCond == nil {
		return false
	}
	// If we're adding a condition (oldCond nil, newCond exists), don't trigger reconcile.
	// This happens when the controller sets the condition for the first time.
	if oldCond == nil {
		return false
	}
	// If the condition was removed (oldCond exists, newCond nil), trigger reconcile.
	// This indicates external modification.
	if newCond == nil {
		return true
	}
	// Otherwise, check if the condition's meaningful fields changed.
	return oldCond.Status != newCond.Status || oldCond.Reason != newCond.Reason || oldCond.Message != newCond.Message
}

// reportConfigurationStateCondition creates or updates a ConfigurationState resource with a condition.
// It uses Server-Side Apply to manage both the resource metadata and its status.
func reportConfigurationStateCondition(
	ctx context.Context,
	c client.Client,
	namespace string,
	name string,
	labels map[string]string,
	conditionType string,
	fieldOwner string,
	conditionErr error,
) error {
	if name == "" {
		return nil
	}

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		Reason:             ErrorTypeNone,
		Message:            "",
		LastTransitionTime: metav1.Now(),
	}
	result := metallbv1beta1.ConfigurationResultValid
	var errorMessages string

	switch {
	case errors.Is(conditionErr, ErrConfiguration):
		condition.Status = metav1.ConditionFalse
		condition.Reason = ErrorTypeConfiguration
		condition.Message = conditionErr.Error()
		result = metallbv1beta1.ConfigurationResultInvalid
		errorMessages = condition.Message
	case conditionErr != nil:
		condition.Status = metav1.ConditionFalse
		condition.Reason = ErrorTypeUnknown
		condition.Message = conditionErr.Error()
		result = metallbv1beta1.ConfigurationResultInvalid
		errorMessages = condition.Message
	}

	config := &metallbv1beta1.ConfigurationState{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "metallb.io/v1beta1",
			Kind:       "ConfigurationState",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
	if err := c.Patch(ctx, config, client.Apply, client.FieldOwner(fieldOwner), client.ForceOwnership); err != nil {
		return fmt.Errorf("patch %s/%s: %w", namespace, name, err)
	}

	configStatus := &metallbv1beta1.ConfigurationState{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "metallb.io/v1beta1",
			Kind:       "ConfigurationState",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: metallbv1beta1.ConfigurationStateStatus{
			Conditions:   []metav1.Condition{condition},
			Result:       result,
			ErrorSummary: errorMessages,
		},
	}
	if err := c.Status().Patch(ctx, configStatus, client.Apply, client.FieldOwner(fieldOwner), client.ForceOwnership); err != nil {
		return fmt.Errorf("patch status %s/%s: %w", namespace, name, err)
	}

	return nil
}
