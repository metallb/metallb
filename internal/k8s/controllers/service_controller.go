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
	Logger            log.Logger
	Scheme            *runtime.Scheme
	Namespace         string
	Handler           func(log.Logger, string, *v1.Service, []discovery.EndpointSlice) SyncState
	Endpoints         bool
	LoadBalancerClass string
	Reload            chan event.GenericEvent
	// initialLoadPerformed is set after the first time we call reprocessAll.
	// This is required because we want the first time we load the services to follow the assigned first, non assigned later order.
	// This allows avoiding to have services with already assigned IP to get their IP stolen by other services.
	initialLoadPerformed bool
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
	updates.Inc()

	var service *v1.Service

	if !r.initialLoadPerformed {
		level.Debug(r.Logger).Log("controller", "ServiceReconciler", "message", "filtered service, still waiting for the initial load to be performed")
		return ctrl.Result{}, nil
	}

	service, err := r.serviceFor(ctx, req.NamespacedName)
	if err != nil {
		level.Error(r.Logger).Log("controller", "ServiceReconciler", "message", "failed to get service", "service", req.NamespacedName, "error", err)
		return ctrl.Result{}, err
	}

	if filterByLoadBalancerClass(service, r.LoadBalancerClass) {
		level.Debug(r.Logger).Log("controller", "ServiceReconciler", "filtered service", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	epSlices := []discovery.EndpointSlice{}
	if r.Endpoints {
		epSlices, err = epSlicesForService(ctx, r, req.NamespacedName)
		if err != nil {
			level.Error(r.Logger).Log("controller", "ServiceReconciler", "message", "failed to get endpoints", "service", req.NamespacedName, "error", err)
			return ctrl.Result{}, err
		}
	}
	if service != nil {
		level.Debug(r.Logger).Log("controller", "ServiceReconciler", "processing service", dumpResource(service))
	} else {
		level.Debug(r.Logger).Log("controller", "ServiceReconciler", "processing deletion on service", req.NamespacedName.String())
	}

	res := r.Handler(r.Logger, req.NamespacedName.String(), service, epSlices)
	switch res {
	case SyncStateError:
		updateErrors.Inc()
		level.Info(r.Logger).Log("controller", "ServiceReconciler", "name", req.NamespacedName.String(), "service", dumpResource(service), "endpoints", dumpResource(epSlices), "event", "failed to handle service")
		return ctrl.Result{}, errRetry
	case SyncStateReprocessAll:
		level.Info(r.Logger).Log("controller", "ServiceReconciler", "event", "force service reload")
		r.forceReload()
		return ctrl.Result{}, nil
	case SyncStateErrorNoRetry:
		updateErrors.Inc()
		level.Error(r.Logger).Log("controller", "ServiceReconciler", "name", req.NamespacedName.String(), "service", dumpResource(service), "endpoints", dumpResource(epSlices), "event", "failed to handle service")
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Endpoints {
		return ctrl.NewControllerManagedBy(mgr).
			For(&v1.Service{}).
			Watches(&discovery.EndpointSlice{},
				handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
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
			WatchesRawSource(source.Channel(r.Reload, &handler.EnqueueRequestForObject{})).
			Complete(r)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Service{}).
		WatchesRawSource(source.Channel(r.Reload, &handler.EnqueueRequestForObject{})).
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

func filterByLoadBalancerClass(service *v1.Service, loadBalancerClass string) bool {
	// When receiving a delete, we can't make logic on the service so we
	// rely on the application logic that will receive a delete on a service it
	// did not handle and discard it.
	if service == nil {
		return false
	}
	if service.Spec.LoadBalancerClass == nil && loadBalancerClass != "" {
		return true
	}
	if service.Spec.LoadBalancerClass == nil && loadBalancerClass == "" {
		return false
	}
	if *service.Spec.LoadBalancerClass != loadBalancerClass {
		return true
	}
	return false
}
