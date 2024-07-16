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
	reconcileChan        chan event.GenericEvent
	configChangedChan    chan struct{}
	desiredConfiguration *frrv1beta1.FRRConfiguration
	sync.Mutex
}

func (r *FRRK8sReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "FRRK8sReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "FRRK8sReconciler", "end reconcile", req.NamespacedName.String())
	updates.Inc()

	r.Lock()
	defer r.Unlock()
	if r.desiredConfiguration == nil {
		config := &frrv1beta1.FRRConfiguration{ObjectMeta: metav1.ObjectMeta{Name: frrk8s.ConfigName(r.NodeName), Namespace: r.FRRK8sNamespace}}
		err := r.Delete(ctx, config)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	current := frrv1beta1.FRRConfiguration{}
	err := r.Get(ctx, client.ObjectKey{Name: r.desiredConfiguration.Name, Namespace: r.desiredConfiguration.Namespace}, &current)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if reflect.DeepEqual(current.Spec, r.desiredConfiguration.Spec) {
		level.Debug(r.Logger).Log("controller", "FRRK8sReconciler", "event", "not reconciling because of no change")
		return ctrl.Result{}, nil
	}

	toApply := &frrv1beta1.FRRConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.desiredConfiguration.Name,
			Namespace: r.desiredConfiguration.Namespace},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, toApply, func() error {
		r.desiredConfiguration.Spec.DeepCopyInto(&toApply.Spec)
		return nil
	})
	if err != nil {
		level.Info(r.Logger).Log("controller", "FRRConfiguration", "event", "failed to create frr8s configuration")
		return ctrl.Result{}, err
	}

	if r.LogLevel == logging.LevelDebug {
		toDump, err := frrk8s.ConfigToDump(*r.desiredConfiguration)
		if err != nil {
			level.Error(r.Logger).Log("controller", "FRRConfiguration", "event", "failed to dump frr8s configuration", "error", err)
		}
		level.Debug(r.Logger).Log("controller", "FRRK8sReconciler", "event", "applied new configuration", "config", toDump)
	}

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
