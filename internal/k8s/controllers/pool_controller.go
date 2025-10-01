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
	"fmt"
	"reflect"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type PoolReconciler struct {
	client.Client
	Logger          log.Logger
	Scheme          *runtime.Scheme
	Namespace       string
	ConfigStatusRef types.NamespacedName
	Handler         func(log.Logger, *config.Pools) SyncState
	ValidateConfig  config.Validate
	ForceReload     func()
	currentConfig   *config.Config
}

func (r *PoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "PoolReconciler", "start reconcile", req.String())
	defer level.Info(r.Logger).Log("controller", "PoolReconciler", "end reconcile", req.String())

	var syncResult SyncState
	var syncError error

	defer func() {
		if err := r.reportCondition(ctx, syncError, syncResult); err != nil {
			level.Error(r.Logger).Log("controller", "PoolReconciler", "error", err, "syncError", syncError)
		}
	}()

	updates.Inc()

	var ipAddressPools metallbv1beta1.IPAddressPoolList
	if err := r.List(ctx, &ipAddressPools, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "PoolReconciler", "message", "failed to get ipaddresspools", "error", err)
		syncError = fmt.Errorf("failed to list ipaddresspools: %w", err)
		return ctrl.Result{}, err
	}

	var communities metallbv1beta1.CommunityList
	if err := r.List(ctx, &communities, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "PoolReconciler", "message", "failed to get communities", "error", err)
		syncError = fmt.Errorf("failed to list communities: %w", err)
		return ctrl.Result{}, err
	}

	var namespaces corev1.NamespaceList
	if err := r.List(ctx, &namespaces); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get namespaces", "error", err)
		syncError = fmt.Errorf("failed to list namespaces: %w", err)
		return ctrl.Result{}, err
	}

	resources := config.ClusterResources{
		Pools:       ipAddressPools.Items,
		Communities: communities.Items,
		Namespaces:  namespaces.Items,
	}

	level.Debug(r.Logger).Log("controller", "PoolReconciler", "metallb CRs", dumpClusterResources(&resources))

	cfg, err := toConfig(resources, r.ValidateConfig)
	if err != nil {
		configStale.Set(1)
		syncResult = SyncStateError
		level.Error(r.Logger).Log("controller", "PoolReconciler", "error", "failed to parse the configuration", "error", err)
		syncError = fmt.Errorf("failed to parse configuration: %w", err)
		return ctrl.Result{}, nil
	}

	level.Debug(r.Logger).Log("controller", "PoolReconciler", "rendered config", dumpConfig(cfg))
	if reflect.DeepEqual(r.currentConfig, cfg) {
		level.Debug(r.Logger).Log("controller", "PoolReconciler", "event", "configuration did not change, ignoring")
		syncResult = SyncStateSuccess
		return ctrl.Result{}, nil
	}

	syncResult = r.Handler(r.Logger, cfg.Pools)
	switch syncResult {
	case SyncStateError:
		updateErrors.Inc()
		configStale.Set(1)
		level.Error(r.Logger).Log("controller", "PoolReconciler", "metallb CRs and Secrets", dumpClusterResources(&resources), "event", "reload failed, retry")
		syncError = fmt.Errorf("handler failed to apply pool configuration: %w", errRetry)
		return ctrl.Result{}, errRetry
	case SyncStateReprocessAll:
		level.Info(r.Logger).Log("controller", "PoolReconciler", "event", "force service reload")
		r.ForceReload()
	case SyncStateErrorNoRetry:
		updateErrors.Inc()
		configStale.Set(1)
		level.Error(r.Logger).Log("controller", "PoolReconciler", "metallb CRs and Secrets", dumpClusterResources(&resources), "event", "reload failed, no retry")
		return ctrl.Result{}, nil
	}

	r.currentConfig = cfg

	configLoaded.Set(1)
	configStale.Set(0)
	level.Info(r.Logger).Log("controller", "PoolReconciler", "event", "config reloaded")
	return ctrl.Result{}, nil
}

func (r *PoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return filterNodeEvent(e) && filterNamespaceEvent(e) && filterPoolStatusEvent(e)
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1beta1.IPAddressPool{}).
		Watches(&metallbv1beta1.Community{}, &handler.EnqueueRequestForObject{}).
		Watches(&corev1.Namespace{}, &handler.EnqueueRequestForObject{}).
		WithEventFilter(p).
		Complete(r)
}

func filterPoolStatusEvent(e event.UpdateEvent) bool {
	_, ok := e.ObjectOld.(*metallbv1beta1.IPAddressPool)
	if !ok {
		return true
	}

	_, ok = e.ObjectNew.(*metallbv1beta1.IPAddressPool)
	if !ok {
		return true
	}

	return predicate.GenerationChangedPredicate{}.Update(e)
}

func (r *PoolReconciler) reportCondition(ctx context.Context, configErr error, syncResult SyncState) error {
	const owner = "poolReconciler"

	condition := metav1.Condition{
		Type:               "poolReconcilerValid",
		Status:             metav1.ConditionTrue,
		Reason:             syncResult.String(),
		LastTransitionTime: metav1.Now(),
	}

	if configErr != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "ConfigError"
		condition.Message = configErr.Error()
	}

	if syncResult != SyncStateSuccess && syncResult != SyncStateReprocessAll {
		condition.Status = metav1.ConditionFalse
	}

	if err := patchCondition(ctx, r.Client, r.ConfigStatusRef, owner, condition); err != nil {
		return fmt.Errorf("failed to patch condition for %s: %w", owner, err)
	}
	return nil
}
