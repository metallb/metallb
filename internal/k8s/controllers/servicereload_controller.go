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

	"github.com/davecgh/go-spew/spew"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"go.universe.tf/metallb/internal/k8s/epslices"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type reloadEvent struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (evt *reloadEvent) DeepCopyObject() runtime.Object {
	res := new(reloadEvent)
	res.Name = evt.Name
	res.Namespace = evt.Namespace
	return res
}

func NewReloadEvent() event.GenericEvent {
	evt := reloadEvent{}
	evt.Name = "reload"
	evt.Namespace = "reload"
	return event.GenericEvent{Object: &evt}
}

type ServiceReloadReconciler struct {
	client.Client
	Log       log.Logger
	Scheme    *runtime.Scheme
	Endpoints NeedEndPoints
	Handler   func(log.Logger, string, *v1.Service, epslices.EpsOrSlices) SyncState
	Reload    chan event.GenericEvent
}

func (r *ServiceReloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Log).Log("controller", "ServiceReloadReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Log).Log("controller", "ServiceReloadReconciler", "end reconcile", req.NamespacedName.String())

	return r.reprocessAllServices(ctx)
}

func (r *ServiceReloadReconciler) reprocessAllServices(ctx context.Context) (ctrl.Result, error) {
	var services v1.ServiceList
	if err := r.List(ctx, &services); err != nil {
		level.Error(r.Log).Log("controller", "ServiceReloadReconciler", "error", "failed to list the services", "error", err)
		return ctrl.Result{}, err
	}

	retry := true
	for _, service := range services.Items {
		serviceName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
		eps, err := epsOrSlicesForServices(ctx, r, serviceName, r.Endpoints)
		if err != nil {
			level.Error(r.Log).Log("controller", "ServiceReconciler", "error", "failed to get endpoints", "service", serviceName.String(), "error", err)
			return ctrl.Result{}, err
		}

		level.Debug(r.Log).Log("controller", "ServiceReloadReconciler", "reprocessing service", spew.Sdump(service))

		res := r.Handler(r.Log, serviceName.String(), &service, eps)
		switch res {
		case SyncStateError:
			level.Error(r.Log).Log("controller", "ServiceReloadReconciler", "name", serviceName, "service", spew.Sdump(service), "endpoints", spew.Sdump(eps), "event", "failed to handle service, retry")
			retry = true
		case SyncStateReprocessAll:
			retry = true
		case SyncStateErrorNoRetry:
			level.Error(r.Log).Log("controller", "ServiceReloadReconciler", "name", serviceName, "service", spew.Sdump(service), "endpoints", spew.Sdump(eps), "event", "failed to handle service, no retry")
		}
	}
	if retry {
		// in case we want to retry, we return an error to trigger the exponential backoff mechanism so that
		// this controller won't loop at full speed
		level.Info(r.Log).Log("controller", "ConfigReconciler", "event", "force service reload")
		return ctrl.Result{}, retryError
	}
	return ctrl.Result{}, nil
}

func (r *ServiceReloadReconciler) Start(ctx context.Context, mgr ctrl.Manager) error {
	c, err := controller.NewUnmanaged("ServiceReloadReconciler", mgr,
		controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Channel{Source: r.Reload}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	// Since we are using a raw controller (as opposed to using the manager's builder),
	// we need to start the controller manually.
	go func() {
		err := c.Start(ctx)
		if err != nil {
			level.Error(r.Log).Log("controller", "ServiceReloadReconciler", "failed to start", err)
		}
	}()
	return nil
}
