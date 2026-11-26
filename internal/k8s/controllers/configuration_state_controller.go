// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ConfigurationStateReconciler struct {
	client.Client
	Logger            log.Logger
	Scheme            *runtime.Scheme
	Namespace         string
	ConfigStateName   string
	ConfigStateLabels map[string]string
	OwnerPod          *v1.Pod
}

func (r *ConfigurationStateReconciler) String() string {
	return "ConfigurationStateReconciler"
}

func (r *ConfigurationStateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", r, "start reconcile", req.String())
	defer level.Info(r.Logger).Log("controller", r, "end reconcile", req.String())

	configState := &metallbv1beta1.ConfigurationState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.ConfigStateName,
			Namespace: r.Namespace,
		},
	}

	opResult, err := controllerutil.CreateOrPatch(ctx, r.Client, configState, func() error {
		configState.Labels = r.ConfigStateLabels
		if err := controllerutil.SetOwnerReference(r.OwnerPod, configState, r.Scheme); err != nil {
			return err
		}

		result := metallbv1beta1.ConfigurationResultUnknown
		if len(configState.Status.Conditions) > 0 {
			result = metallbv1beta1.ConfigurationResultValid
		}

		var errorMessages []string
		for _, cond := range configState.Status.Conditions {
			if cond.Status == metav1.ConditionFalse && cond.Reason == ErrorTypeConfiguration {
				result = metallbv1beta1.ConfigurationResultInvalid
				errorMessages = append(errorMessages, cond.Message)
			}
		}

		configState.Status.Result = result
		configState.Status.ErrorSummary = strings.Join(errorMessages, "\n")
		return nil
	})
	if err != nil {
		level.Error(r.Logger).Log("controller", r, "error", "failed to create or patch ConfigurationState", "error", err)
		return ctrl.Result{}, err
	}
	level.Info(r.Logger).Log("controller", r, "event", fmt.Sprintf("ConfigurationState %s", opResult), "name", r.ConfigStateName)

	return ctrl.Result{}, nil
}

// NewConfigurationStateReconcilerPredicate returns a predicate that filters events
// for a specific ConfigurationState CR by namespace and name.
func NewConfigurationStateReconcilerPredicate(namespace, name string) predicate.Predicate {
	matchesTarget := func(obj client.Object) bool {
		return obj.GetNamespace() == namespace && obj.GetName() == name
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return matchesTarget(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return matchesTarget(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return matchesTarget(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return matchesTarget(e.Object)
		},
	}
}

// NewConfigStateConditionReporterPredicate returns a predicate for controllers that write
// the condition conditionType to a ConfigurationState CR. It fires on Create events for
// the target CR, so that the condition is reported again after the resource is recreated,
// and on Update events that change the condition: reportConfigStateCondition doesn't
// write a condition the cache already holds, and the cache may not reflect the last
// write yet.
func NewConfigStateConditionReporterPredicate(namespace, name, conditionType string) predicate.Predicate {
	matchesTarget := func(obj client.Object) bool {
		return obj.GetNamespace() == namespace && obj.GetName() == name
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return matchesTarget(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldState, ok := e.ObjectOld.(*metallbv1beta1.ConfigurationState)
			if !ok {
				return false
			}
			newState, ok := e.ObjectNew.(*metallbv1beta1.ConfigurationState)
			if !ok || !matchesTarget(newState) {
				return false
			}
			return !equality.Semantic.DeepEqual(
				meta.FindStatusCondition(oldState.Status.Conditions, conditionType),
				meta.FindStatusCondition(newState.Status.Conditions, conditionType))
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func (r *ConfigurationStateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := NewConfigurationStateReconcilerPredicate(r.Namespace, r.ConfigStateName)

	reconcileChan := make(chan event.GenericEvent, 1)

	go func() {
		configStateRef := &metallbv1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.ConfigStateName,
				Namespace: r.Namespace,
				Labels:    r.ConfigStateLabels,
			},
		}
		reconcileChan <- event.GenericEvent{
			Object: configStateRef,
		}
	}()

	return ctrl.NewControllerManagedBy(mgr).
		Named("ConfigurationStateController").
		For(&metallbv1beta1.ConfigurationState{}).
		WatchesRawSource(source.Channel(reconcileChan, &handler.EnqueueRequestForObject{})).
		WithEventFilter(p).
		Complete(r)
}

// reportConfigStateCondition sets condition in the status of the ConfigurationState
// key. Most reconciles don't change the condition, and every write is delivered to
// all the watchers of the resource, so it writes only when the condition differs
// from the one in the resource; LastTransitionTime moves only when the status
// changes. The resource may come from a cache that doesn't reflect the last write
// yet: then a write fails on the stale resourceVersion, and a skipped write is
// done once the cache catches up, see NewConfigStateConditionReporterPredicate.
func reportConfigStateCondition(ctx context.Context, cl client.Client, key client.ObjectKey, fieldOwner string, condition metav1.Condition) error {
	configState := &metallbv1beta1.ConfigurationState{}
	if err := cl.Get(ctx, key, configState); err != nil {
		if apierrors.IsNotFound(err) {
			// The condition is reported when the resource is created, see
			// NewConfigStateConditionReporterPredicate.
			return nil
		}
		return fmt.Errorf("get %s: %w", key, err)
	}

	if !meta.SetStatusCondition(&configState.Status.Conditions, condition) {
		return nil
	}

	if err := cl.Status().Update(ctx, configState, client.FieldOwner(fieldOwner)); err != nil {
		return fmt.Errorf("update %s status: %w", key, err)
	}
	return nil
}
