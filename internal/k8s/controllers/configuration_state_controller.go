// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ConfigurationStateReconciler struct {
	client.Client
	Logger         log.Logger
	Scheme         *runtime.Scheme
	ConfigStateRef *metallbv1beta1.ConfigurationState
}

func (r *ConfigurationStateReconciler) String() string {
	return "ConfigurationStateReconciler"
}

func (r *ConfigurationStateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", r, "start reconcile", req.String())
	defer level.Info(r.Logger).Log("controller", r, "end reconcile", req.String())

	var configStatus metallbv1beta1.ConfigurationState
	if err := r.Get(ctx, req.NamespacedName, &configStatus); err != nil {
		if !apierrors.IsNotFound(err) {
			level.Error(r.Logger).Log("controller", r, "error", "failed to get ConfigurationState", "error", err)
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, r.ConfigStateRef); err != nil {
			level.Error(r.Logger).Log("controller", r, "error", "failed to create ConfigurationState", "error", err)
			return ctrl.Result{}, err
		}

		level.Info(r.Logger).Log("controller", r, "error", "ConfigurationState created", "name", r.ConfigStateRef.Name)
		return ctrl.Result{}, nil
	}

	result := metallbv1beta1.ConfigurationStateResultUnknown
	lastError := ""

	// Only aggregate if there are conditions reported
	if len(configStatus.Status.Conditions) > 0 {
		result = metallbv1beta1.ConfigurationStateResultValid

		var msgs []string
		for _, cond := range configStatus.Status.Conditions {
			if cond.Status == metav1.ConditionFalse {
				msgs = append(msgs, fmt.Sprintf("%s: %s", cond.Type, cond.Message))
			}
		}

		if len(msgs) > 0 {
			result = metallbv1beta1.ConfigurationStateResultInvalid
			lastError = strings.Join(msgs, "\n")
		}
	}

	configStatus.Status.Result = result
	configStatus.Status.LastError = lastError
	if err := r.Client.Status().Update(ctx, &configStatus); err != nil {
		level.Error(r.Logger).Log("controller", r, "error", "failed to update status", "result", result, "error", err)
		return ctrl.Result{}, err
	}

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

func (r *ConfigurationStateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := NewConfigurationStateReconcilerPredicate(r.ConfigStateRef.Namespace, r.ConfigStateRef.Name)

	reconcileChan := make(chan event.GenericEvent, 1)

	go func() {
		reconcileChan <- event.GenericEvent{
			Object: r.ConfigStateRef,
		}
	}()

	return ctrl.NewControllerManagedBy(mgr).
		Named("ConfigurationStateController").
		For(&metallbv1beta1.ConfigurationState{}).
		WatchesRawSource(source.Channel(reconcileChan, &handler.EnqueueRequestForObject{})).
		WithEventFilter(p).
		Complete(r)
}
