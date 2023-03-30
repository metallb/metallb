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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
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

func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newNodeObj, ok := e.ObjectNew.(*corev1.Node)
			if !ok {
				level.Error(r.Logger).Log("controller", "NodeReconciler", "error", "new object is not node", "name", newNodeObj.GetName())
				return true
			}
			oldNodeObj, ok := e.ObjectOld.(*corev1.Node)
			if !ok {
				level.Error(r.Logger).Log("controller", "NodeReconciler", "error", "old object is not node", "name", oldNodeObj.GetName())
				return true
			}
			// If there is no changes in node labels, ignore event.
			if labels.Equals(labels.Set(oldNodeObj.Labels), labels.Set(newNodeObj.Labels)) &&
				reflect.DeepEqual(oldNodeObj.Status.Conditions, newNodeObj.Status.Conditions) {
				return false
			}
			return true
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		WithEventFilter(p).
		Complete(r)
}
