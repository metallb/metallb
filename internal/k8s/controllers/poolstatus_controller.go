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

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"go.universe.tf/metallb/api/v1beta1"
)

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

func NewPoolStatusEvent(name, namespace string) event.GenericEvent {
	return event.GenericEvent{Object: &poolStatusEvent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}}
}

type PoolStatusReconciler struct {
	client.Client
	Logger            log.Logger
	GetStatusForPool  func(string) (v1beta1.IPAddressPoolStatus, error)
	PoolStatusChannel chan event.GenericEvent
}

func (r *PoolStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "PoolStatusReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "PoolStatusReconciler", "end reconcile", req.NamespacedName.String())

	var pool v1beta1.IPAddressPool
	err := r.Get(ctx, req.NamespacedName, &pool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		level.Error(r.Logger).Log("controller", "PoolStatusReconciler", "message", "failed to get pool", "ipaddresspool", req.NamespacedName, "error", err)
		return ctrl.Result{}, nil
	}

	status, err := r.GetStatusForPool(pool.Name)
	if err != nil {
		level.Error(r.Logger).Log("controller", "PoolStatusReconciler", "message", "failed to obtain pool stats", "ipaddresspool", req.NamespacedName, "error", err)
		return ctrl.Result{}, errRetry
	}

	if reflect.DeepEqual(pool.Status, status) {
		return ctrl.Result{}, nil
	}

	err = r.Client.Status().Update(ctx, &pool)
	if err != nil {
		level.Error(r.Logger).Log("controller", "PoolStatusReconciler", "message", "failed to update the status", "ipaddresspool", req.NamespacedName, "error", err)
		return ctrl.Result{}, errRetry
	}

	return ctrl.Result{}, nil
}

func (r *PoolStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("PoolStatusController").
		Watches(&source.Channel{Source: r.PoolStatusChannel}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
