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

	"github.com/go-kit/log/level"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
	evt.Namespace = "metallbreload"
	return event.GenericEvent{Object: &evt}
}

func isReloadReq(req ctrl.Request) bool {
	if req.Name == "reload" && req.Namespace == "metallbreload" {
		return true
	}
	return false
}

func (r *ServiceReconciler) reprocessAll(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "ServiceReconciler - reprocessAll", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "ServiceReconciler - reprocessAll", "end reconcile", req.NamespacedName.String())

	var services v1.ServiceList
	if err := r.List(ctx, &services); err != nil {
		level.Error(r.Logger).Log("controller", "ServiceReconciler - reprocessAll", "message", "failed to list the services", "error", err)
		return ctrl.Result{}, err
	}

	retry := false
	for _, service := range services.Items {
		serviceName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
		eps, err := epsOrSlicesForServices(ctx, r, serviceName, r.Endpoints)
		if err != nil {
			level.Error(r.Logger).Log("controller", "ServiceReconciler - reprocessAll", "message", "failed to get endpoints", "service", serviceName.String(), "error", err)
			return ctrl.Result{}, err
		}

		level.Debug(r.Logger).Log("controller", "ServiceReconciler - reprocessAll", "reprocessing service", dumpResource(service))

		res := r.Handler(r.Logger, serviceName.String(), &service, eps)
		switch res {
		case SyncStateError:
			level.Error(r.Logger).Log("controller", "ServiceReconciler - reprocessAll", "name", serviceName, "service", dumpResource(service), "endpoints", dumpResource(eps), "event", "failed to handle service, retry")
			retry = true
		case SyncStateReprocessAll:
			retry = true
		case SyncStateErrorNoRetry:
			level.Error(r.Logger).Log("controller", "ServiceReconciler - reprocessAll", "name", serviceName, "service", dumpResource(service), "endpoints", dumpResource(eps), "event", "failed to handle service, no retry")
		}
	}
	if retry {
		// in case we want to retry, we return an error to trigger the exponential backoff mechanism so that
		// this controller won't loop at full speed
		level.Info(r.Logger).Log("controller", "ServiceReconciler - reprocessAll", "event", "force service reload")
		return ctrl.Result{}, retryError
	}
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) forceReload() {
	r.Reload <- NewReloadEvent()
}
