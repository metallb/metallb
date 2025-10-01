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
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	frrk8s "go.universe.tf/metallb/internal/bgp/frrk8s"
	"go.universe.tf/metallb/internal/logging"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type frrk8sConfigEvent struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (evt *frrk8sConfigEvent) DeepCopyObject() runtime.Object {
	res := new(frrk8sConfigEvent)
	res.Name = evt.Name
	res.Namespace = evt.Namespace
	return res
}

func NewFRRK8sConfigEvent() event.GenericEvent {
	evt := frrk8sConfigEvent{}
	evt.Name = "reload"
	evt.Namespace = "frrk8sreload"
	return event.GenericEvent{Object: &evt}
}

type FRRK8sReconciler struct {
	client.Client
	Logger               log.Logger
	LogLevel             logging.Level
	Scheme               *runtime.Scheme
	NodeName             string
	FRRK8sNamespace      string
	ConfigStatusRef      types.NamespacedName
	reconcileChan        chan event.GenericEvent
	configChangedChan    chan struct{}
	desiredConfiguration *frrv1beta1.FRRConfiguration
	sync.Mutex
}

func (r *FRRK8sReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "FRRK8sReconciler", "start reconcile", req.String())
	defer level.Info(r.Logger).Log("controller", "FRRK8sReconciler", "end reconcile", req.String())

	var syncResult SyncState
	var syncError error

	defer func() {
		if err := r.reportCondition(ctx, syncError, syncResult); err != nil {
			level.Error(r.Logger).Log("controller", "FRRK8sReconciler", "error", err, "syncError", syncError)
		}
	}()

	updates.Inc()

	r.Lock()
	defer r.Unlock()
	if r.desiredConfiguration == nil {
		config := &frrv1beta1.FRRConfiguration{ObjectMeta: metav1.ObjectMeta{Name: frrk8s.ConfigName(r.NodeName), Namespace: r.FRRK8sNamespace}}
		if err := r.Delete(ctx, config); err != nil {
			err = client.IgnoreNotFound(err)
			if err != nil {
				syncResult = SyncStateError
				syncError = fmt.Errorf("failed to delete frr configuration: %w", err)
				return ctrl.Result{}, err
			}
		}
		syncResult = SyncStateSuccess
		return ctrl.Result{}, nil
	}

	current := frrv1beta1.FRRConfiguration{}
	if err := r.Get(ctx, client.ObjectKey{Name: r.desiredConfiguration.Name, Namespace: r.desiredConfiguration.Namespace}, &current); err != nil && !apierrors.IsNotFound(err) {
		syncResult = SyncStateError
		syncError = fmt.Errorf("failed to get frr configuration: %w", err)
		return ctrl.Result{}, err
	}

	if reflect.DeepEqual(current.Spec, r.desiredConfiguration.Spec) {
		level.Debug(r.Logger).Log("controller", "FRRK8sReconciler", "event", "not reconciling because of no change")
		syncResult = SyncStateSuccess
		return ctrl.Result{}, nil
	}

	toApply := &frrv1beta1.FRRConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.desiredConfiguration.Name,
			Namespace: r.desiredConfiguration.Namespace},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, toApply, func() error {
		r.desiredConfiguration.Spec.DeepCopyInto(&toApply.Spec)
		return nil
	}); err != nil {
		level.Info(r.Logger).Log("controller", "FRRConfiguration", "event", "failed to create frr8s configuration")
		syncResult = SyncStateError
		syncError = fmt.Errorf("failed to create or update frr configuration: %w", err)
		return ctrl.Result{}, err
	}

	if r.LogLevel == logging.LevelDebug {
		toDump, err := frrk8s.ConfigToDump(*r.desiredConfiguration)
		if err != nil {
			level.Error(r.Logger).Log("controller", "FRRConfiguration", "event", "failed to dump frr8s configuration", "error", err)
		}
		level.Debug(r.Logger).Log("controller", "FRRK8sReconciler", "event", "applied new configuration", "config", toDump)
	}

	syncResult = SyncStateSuccess
	return ctrl.Result{}, nil
}

func (r *FRRK8sReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.configChangedChan = make(chan struct{})
	r.reconcileChan = make(chan event.GenericEvent)
	debouncer(r.configChangedChan, r.reconcileChan, 3*time.Second)

	configName := frrk8s.ConfigName(r.NodeName)
	p := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		config, ok := obj.(*frrv1beta1.FRRConfiguration)
		if !ok {
			return true
		}
		if config.Name != configName {
			return false
		}
		if config.Namespace != r.FRRK8sNamespace {
			return false
		}
		return true
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&frrv1beta1.FRRConfiguration{}).
		WatchesRawSource(source.Channel(r.reconcileChan, &handler.EnqueueRequestForObject{})).
		WithEventFilter(p).
		Complete(r)
}

func (r *FRRK8sReconciler) UpdateConfig(config interface{}) {
	r.Lock()
	defer r.Unlock()
	desired, ok := config.(frrv1beta1.FRRConfiguration)
	if !ok {
		panic("received an event that is not frr configuration")
	}
	r.desiredConfiguration = desired.DeepCopy()
	r.configChangedChan <- struct{}{}
}

// reportCondition implements ConditionReporter interface.
func (r *FRRK8sReconciler) reportCondition(ctx context.Context, configErr error, syncResult SyncState) error {
	owner := "frrk8sReconciler"
	if r.NodeName != "" {
		owner = fmt.Sprintf("speaker-%s/frrk8sReconciler", r.NodeName)
	}

	condition := metav1.Condition{
		Type:               owner + "Valid",
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

func debouncer(
	in <-chan struct{},
	out chan<- event.GenericEvent,
	reloadInterval time.Duration) {
	go func() {
		var timeOut <-chan time.Time
		timerSet := false
		for {
			select {
			case _, ok := <-in:
				if !ok { // the channel was closed
					return
				}
				if !timerSet {
					timeOut = time.After(reloadInterval)
					timerSet = true
				}
			case <-timeOut:
				timerSet = false
				out <- NewReloadEvent()
			}
		}
	}()
}
