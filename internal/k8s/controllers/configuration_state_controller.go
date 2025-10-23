// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
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
	Logger            log.Logger
	Scheme            *runtime.Scheme
	Namespace         string
	ConfigStateName   string
	ConfigStateLabels map[string]string
}

func (r *ConfigurationStateReconciler) String() string {
	return "ConfigurationStateReconciler"
}

func (r *ConfigurationStateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", r, "start reconcile", req.String())
	defer level.Info(r.Logger).Log("controller", r, "end reconcile", req.String())

	var configStatus metallbv1beta1.ConfigurationState
	err := r.Get(ctx, req.NamespacedName, &configStatus)
	if apierrors.IsNotFound(err) {
		configState := &metallbv1beta1.ConfigurationState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.ConfigStateName,
				Namespace: r.Namespace,
				Labels:    r.ConfigStateLabels,
			},
		}
		if err := r.Create(ctx, configState); err != nil {
			level.Error(r.Logger).Log("controller", r, "error", "failed to create ConfigurationState", "error", err)
			return ctrl.Result{}, err
		}

		level.Info(r.Logger).Log("controller", r, "error", "ConfigurationState created", "name", r.ConfigStateName)
		return ctrl.Result{}, nil
	}
	if err != nil {
		level.Error(r.Logger).Log("controller", r, "error", "failed to get ConfigurationState", "error", err)
		return ctrl.Result{}, err
	}

	result := metallbv1beta1.ConfigurationResultUnknown
	if len(configStatus.Status.Conditions) > 0 {
		result = metallbv1beta1.ConfigurationResultValid
	}

	var errorMessages []string
	for _, cond := range configStatus.Status.Conditions {
		if cond.Status == metav1.ConditionFalse && cond.Reason == ErrorTypeConfiguration {
			result = metallbv1beta1.ConfigurationResultInvalid
			errorMessages = append(errorMessages, cond.Message)
		}
	}

	configStatus.Status.Result = result
	configStatus.Status.ErrorSummary = strings.Join(errorMessages, "\n")
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
