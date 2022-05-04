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

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"go.universe.tf/metallb/internal/k8s/epslices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"

	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ServiceReconciler struct {
	client.Client
	Logger    log.Logger
	Scheme    *runtime.Scheme
	Namespace string
	Handler   func(log.Logger, string, *v1.Service, epslices.EpsOrSlices) SyncState
	Endpoints NeedEndPoints
	Reload    chan event.GenericEvent
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !isReloadReq(req) {
		return r.reconcileService(ctx, req)
	}
	return r.reprocessAll(ctx, req)
}

func (r *ServiceReconciler) reconcileService(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "ServiceReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "ServiceReconciler", "end reconcile", req.NamespacedName.String())

	var service *v1.Service

	service, err := r.serviceFor(ctx, req.NamespacedName)
	if err != nil {
		level.Error(r.Logger).Log("controller", "ServiceReconciler", "message", "failed to get service", "service", req.NamespacedName, "error", err)
		return ctrl.Result{}, err
	}

	epSlices, err := epsOrSlicesForServices(ctx, r, req.NamespacedName, r.Endpoints)
	if err != nil {
		level.Error(r.Logger).Log("controller", "ServiceReconciler", "message", "failed to get endpoints", "service", req.NamespacedName, "error", err)
		return ctrl.Result{}, err
	}
	if service != nil {
		level.Debug(r.Logger).Log("controller", "ServiceReconciler", "processing service", dumpResource(service))
	} else {
		level.Debug(r.Logger).Log("controller", "ServiceReconciler", "processing deletion on service", req.NamespacedName.String())
	}

	res := r.Handler(r.Logger, req.NamespacedName.String(), service, epSlices)
	switch res {
	case SyncStateError:
		level.Info(r.Logger).Log("controller", "ServiceReconciler", "name", req.NamespacedName.String(), "service", dumpResource(service), "endpoints", dumpResource(epSlices), "event", "failed to handle service")
		return ctrl.Result{}, retryError
	case SyncStateReprocessAll:
		level.Info(r.Logger).Log("controller", "ServiceReconciler", "event", "force service reload")
		r.forceReload()
		return ctrl.Result{}, nil
	case SyncStateErrorNoRetry:
		level.Error(r.Logger).Log("controller", "ServiceReconciler", "name", req.NamespacedName.String(), "service", dumpResource(service), "endpoints", dumpResource(epSlices), "event", "failed to handle service")
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Endpoints == EndpointSlices {
		return ctrl.NewControllerManagedBy(mgr).
			For(&v1.Service{}).
			Watches(&source.Kind{Type: &discovery.EndpointSlice{}},
				handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
					epSlice, ok := obj.(*discovery.EndpointSlice)
					if !ok {
						level.Error(r.Logger).Log("controller", "ServiceReconciler", "error", "received an object that is not epslice")
						return []reconcile.Request{}
					}
					serviceName, err := epslices.ServiceKeyForSlice(epSlice)
					if err != nil {
						level.Error(r.Logger).Log("controller", "ServiceReconciler", "message", "failed to get serviceName for slice", "error", err, "epslice", epSlice.Name)
						return []reconcile.Request{}
					}
					level.Debug(r.Logger).Log("controller", "ServiceReconciler", "enqueueing", serviceName, "epslice", dumpResource(epSlice))
					return []reconcile.Request{{NamespacedName: serviceName}}
				})).
			Watches(&source.Channel{Source: r.Reload}, &handler.EnqueueRequestForObject{}).
			Complete(r)
	}
	if r.Endpoints == Endpoints {
		return ctrl.NewControllerManagedBy(mgr).
			For(&v1.Service{}).
			Watches(&source.Kind{Type: &v1.Endpoints{}},
				handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
					endpoints, ok := obj.(*v1.Endpoints)
					if !ok {
						level.Error(r.Logger).Log("controller", "ServiceReconciler", "error", "received an object that is not an endpoint")
						return []reconcile.Request{}
					}
					name := types.NamespacedName{Name: endpoints.Name, Namespace: endpoints.Namespace}
					level.Debug(r.Logger).Log("controller", "ServiceReconciler", "enqueueing", name, "endpoints", dumpResource(endpoints))
					return []reconcile.Request{{NamespacedName: name}}
				})).
			Watches(&source.Channel{Source: r.Reload}, &handler.EnqueueRequestForObject{}).
			Complete(r)

	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Service{}).
		Watches(&source.Channel{Source: r.Reload}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

func (r *ServiceReconciler) serviceFor(ctx context.Context, name types.NamespacedName) (*v1.Service, error) {
	var res v1.Service
	err := r.Get(ctx, name, &res)
	if apierrors.IsNotFound(err) { // in case of delete, get fails and we need to pass nil to the handler
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}
