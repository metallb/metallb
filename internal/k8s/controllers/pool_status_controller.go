// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"reflect"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/allocator"
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

type PoolCountersFetcher func(string) allocator.PoolCounters

type poolStatusEvent struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (evt *poolStatusEvent) DeepCopyObject() runtime.Object {
	res := new(poolStatusEvent)
	res.Name = evt.Name
	res.Namespace = evt.Namespace
	return res
}

func NewPoolStatusEvent(namespace, name string) event.GenericEvent {
	evt := poolStatusEvent{}
	evt.Name = name
	evt.Namespace = namespace
	return event.GenericEvent{Object: &evt}
}

type PoolStatusReconciler struct {
	client.Client
	Logger          log.Logger
	CountersFetcher PoolCountersFetcher
	ReconcileChan   <-chan event.GenericEvent
}

func (r *PoolStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "PoolStatusReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "PoolStatusReconciler", "end reconcile", req.NamespacedName.String())

	var pool v1beta1.IPAddressPool
	err := r.Get(ctx, req.NamespacedName, &pool)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	c := r.CountersFetcher(pool.Name)

	newStatus := v1beta1.IPAddressPoolStatus{
		AssignedIPv4:  c.AssignedIPv4,
		AssignedIPv6:  c.AssignedIPv6,
		AvailableIPv4: c.AvailableIPv4,
		AvailableIPv6: c.AvailableIPv6,
	}

	if reflect.DeepEqual(pool.Status, newStatus) {
		return ctrl.Result{}, nil
	}

	pool.Status = newStatus
	err = r.Client.Status().Update(ctx, &pool)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PoolStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !filterPoolStatusEvent(e)
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("PoolStatusController").
		For(&v1beta1.IPAddressPool{}).
		WithEventFilter(p).
		WatchesRawSource(source.Channel(r.ReconcileChan, &handler.EnqueueRequestForObject{})).
		Complete(r)
}
