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

	k8snodes "go.universe.tf/metallb/internal/k8s/nodes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type NodeReconciler struct {
	client.Client
	Logger      log.Logger
	Scheme      *runtime.Scheme
	NodeName    string
	Namespace   string
	Handler     func(log.Logger, *corev1.Node) SyncState
	ForceReload func()
}

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "NodeReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "NodeReconciler", "end reconcile", req.NamespacedName.String())
	updates.Inc()

	var n corev1.Node
	err := r.Get(ctx, req.NamespacedName, &n)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	res := r.Handler(r.Logger, &n)
	switch res {
	case SyncStateError:
		updateErrors.Inc()
		return ctrl.Result{}, errRetry
	case SyncStateReprocessAll:
		level.Info(r.Logger).Log("controller", "NodeReconciler", "event", "force service reload")
		r.ForceReload()
		return ctrl.Result{}, nil
	case SyncStateErrorNoRetry:
		updateErrors.Inc()
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func NodeReconcilerPredicate() predicate.Predicate {
	allowDeletions := predicate.Funcs{
		DeleteFunc: func(_ event.DeleteEvent) bool { return true },
	}

	allowCreations := predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
	}

	nodeConditionNetworkAvailabilityStatusChanged := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, ok := e.ObjectOld.(*corev1.Node)
			if !ok {
				return false
			}

			newNode, ok := e.ObjectNew.(*corev1.Node)
			if !ok {
				return false
			}

			if k8snodes.IsNetworkUnavailable(oldNode) != k8snodes.IsNetworkUnavailable(newNode) {
				return true
			}

			return false
		},
	}

	nodeSpecSchedulableChanged := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, ok := e.ObjectOld.(*corev1.Node)
			if !ok {
				return false
			}

			newNode, ok := e.ObjectNew.(*corev1.Node)
			if !ok {
				return false
			}

			if oldNode.Spec.Unschedulable != newNode.Spec.Unschedulable {
				return true
			}

			return false
		},
	}

	return predicate.And(
		allowDeletions,
		allowCreations,
		predicate.Or(
			nodeConditionNetworkAvailabilityStatusChanged,
			nodeSpecSchedulableChanged,
			predicate.LabelChangedPredicate{},
		),
	)
}

func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		WithEventFilter(NodeReconcilerPredicate()).
		Complete(r)
}
