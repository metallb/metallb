// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// reconcilerEnqueuer is a manager.Runnable that sends a
// GenericEvent to a channel to trigger a reconciliation at startup.
type reconcilerEnqueuer struct {
	sync.Once
	channel chan<- event.GenericEvent
	obj     client.Object
}

func (r *reconcilerEnqueuer) Start(ctx context.Context) error {
	r.Do(func() {
		r.channel <- event.GenericEvent{
			Object: r.obj,
		}
	})
	return nil
}

type ConfigurationStateReconciler struct {
	client.Client
	Logger          log.Logger
	Scheme          *runtime.Scheme
	ConfigStatusRef types.NamespacedName
}

func (r *ConfigurationStateReconciler) String() string {
	return "ConfigurationStateReconciler"
}

func (r *ConfigurationStateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", r, "start reconcile", req.String())
	defer level.Info(r.Logger).Log("controller", r, "end reconcile", req.String())

	var configStatus metallbv1beta1.ConfigurationState
	if err := r.Get(ctx, r.ConfigStatusRef, &configStatus); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		level.Info(r.Logger).Log("controller", r, "event", fmt.Sprintf("%s not found", r.ConfigStatusRef))
		configStatus = metallbv1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.ConfigStatusRef.Name,
				Namespace: r.ConfigStatusRef.Namespace,
			},
			Spec: metallbv1beta1.ConfigurationStateSpec{
				Type: "Controller",
			},
		}
		// TODO: Set OwnerReference to controller deployment for automatic cleanup
		if createErr := r.Create(ctx, &configStatus); createErr != nil {
			level.Error(r.Logger).Log("controller", r, "error", "failed to create ConfigurationState", "name", r.ConfigStatusRef, "error", createErr)
			return ctrl.Result{}, createErr
		}
		level.Info(r.Logger).Log("controller", r, "event", fmt.Sprintf("%s created", r.ConfigStatusRef))
	}

	if err := r.updateReadyCondition(ctx, &configStatus); err != nil {
		level.Error(r.Logger).Log("controller", r, "error", "failed to update Ready condition", "error", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ConfigurationStateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := make(chan event.GenericEvent)

	if err := mgr.Add(&reconcilerEnqueuer{
		channel: c,
		obj: &metallbv1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.ConfigStatusRef.Name,
				Namespace: r.ConfigStatusRef.Namespace,
			},
		},
	}); err != nil {
		level.Error(r.Logger).Log("controller", r, "error", "failed to add runnable to manager", "error", err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("ConfigurationStateController").
		For(&metallbv1beta1.ConfigurationState{}).
		WatchesRawSource(source.Channel(c, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

// ConditionReporter reports controller conditions to ConfigurationState using server-side apply.
// Implementations determine their owner ID ("controller/<name>" or "speaker-<node>/<name>") and
// map configErr (nil or error) and syncResult (SyncState) to condition Status/Reason/Message fields.
// The configErr parameter takes precedence: if non-nil, sets Status=False, Reason="ConfigError", Message=error text.
// Uses Status().Patch() with client.Apply and client.FieldOwner(owner) for conflict-free updates.
type ConditionReporter interface {
	reportCondition(ctx context.Context, configErr error, syncResult SyncState) error
}

// patchCondition patches a condition to ConfigurationState using server-side apply.
func patchCondition(ctx context.Context, cl client.Client, configStatusRef types.NamespacedName, owner string, condition metav1.Condition) error {
	configStatus := &metallbv1beta1.ConfigurationState{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "metallb.io/v1beta1",
			Kind:       "ConfigurationState",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configStatusRef.Name,
			Namespace: configStatusRef.Namespace,
		},
		Status: metallbv1beta1.ConfigurationStateStatus{
			Conditions: []metav1.Condition{condition},
		},
	}

	if err := cl.Status().Patch(ctx, configStatus, client.Apply, client.FieldOwner(owner), client.ForceOwnership); err != nil {
		return fmt.Errorf("patch %s/%s: %w", configStatusRef.Namespace, configStatusRef.Name, err)
	}

	return nil
}

// updateReadyCondition calculates and updates the aggregate Ready condition based on all controller conditions.
// Ready=True when all conditions are True, Ready=False when any condition is False (Reason shows all failed components),
// Ready=Unknown when no conditions exist yet.
func (r *ConfigurationStateReconciler) updateReadyCondition(ctx context.Context, configStatus *metallbv1beta1.ConfigurationState) error {
	const owner = "configurationStatusReconciler"

	condition := metav1.Condition{
		Type:               "Ready",
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: configStatus.Generation,
	}

	componentConditions := configStatus.Status.Conditions
	meta.RemoveStatusCondition(&componentConditions, "Ready")

	if len(componentConditions) == 0 {
		condition.Status = metav1.ConditionUnknown
		condition.Reason = "WaitingForConditions"
		condition.Message = "No controller conditions reported yet"
		return patchCondition(ctx, r.Client, r.ConfigStatusRef, owner, condition)
	}

	var failedComponents []string
	for _, cond := range componentConditions {
		if cond.Status == metav1.ConditionFalse {
			failedComponents = append(failedComponents, cond.Type)
		}
	}

	if len(failedComponents) > 0 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "ComponentsFailing"
		if len(failedComponents) == 1 {
			condition.Reason = "ComponentFailing"
		}
		condition.Message = fmt.Sprintf("Failed components: %s", failedComponents)
		return patchCondition(ctx, r.Client, r.ConfigStatusRef, owner, condition)
	}

	condition.Status = metav1.ConditionTrue
	condition.Reason = "AllComponentsReady"
	condition.Message = ""
	return patchCondition(ctx, r.Client, r.ConfigStatusRef, owner, condition)
}
