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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ServiceReconciler struct {
	client.Client
	Logger      log.Logger
	Scheme      *runtime.Scheme
	Namespace   string
	Handler     func(log.Logger, string, *v1.Service, epslices.EpsOrSlices) SyncState
	Endpoints   NeedEndPoints
	ForceReload func()
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "ServiceReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "ServiceReconciler", "end reconcile", req.NamespacedName.String())

	var service *v1.Service

	service, err := r.serviceFor(ctx, req.NamespacedName)
	if err != nil {
		level.Error(r.Logger).Log("controller", "ServiceReconciler", "error", "failed to get service", "service", req.NamespacedName, "error", err)
		return ctrl.Result{}, err
	}

	epSlices, err := epsOrSlicesForServices(ctx, r, req.NamespacedName, r.Endpoints)
	if err != nil {
		level.Error(r.Logger).Log("controller", "ServiceReconciler", "error", "failed to get endpoints", "service", req.NamespacedName, "error", err)
		return ctrl.Result{}, err
	}
	if service != nil {
		level.Debug(r.Logger).Log("controller", "ServiceReconciler", "processing service", spew.Sdump(service))
	} else {
		level.Debug(r.Logger).Log("controller", "ServiceReconciler", "processing deletion on service", req.NamespacedName.String())
	}

	res := r.Handler(r.Logger, req.NamespacedName.String(), service, epSlices)
	switch res {
	case SyncStateError:
		level.Error(r.Logger).Log("controller", "ServiceReconciler", "name", req.NamespacedName.String(), "service", spew.Sdump(service), "endpoints", spew.Sdump(epSlices), "event", "failed to handle service")
		return ctrl.Result{}, nil
	case SyncStateReprocessAll:
		level.Info(r.Logger).Log("controller", "ServiceReconciler", "event", "force service reload")
		r.ForceReload()
		return ctrl.Result{}, nil
	case SyncStateErrorNoRetry:
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
						level.Error(r.Logger).Log("controller", "ServiceReconciler", "error", "failed to get serviceName for slice", "error", err, "epslice", epSlice.Name)
						return []reconcile.Request{}
					}
					return []reconcile.Request{{NamespacedName: serviceName}}
				})).
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
					return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: endpoints.Name, Namespace: endpoints.Namespace}}}
				})).
			Complete(r)

	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Service{}).
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
