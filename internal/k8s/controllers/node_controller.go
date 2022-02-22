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

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeReconciler struct {
	client.Client
	Logger    log.Logger
	Scheme    *runtime.Scheme
	Namespace string
	Handler   func(log.Logger, *v1.Node) SyncState
}

//+kubebuilder:rbac:groups="",resources=node,verbs=get;list;watch;

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "NodeReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "NodeReconciler", "end reconcile", req.NamespacedName.String())

	var n v1.Node
	err := r.Get(ctx, req.NamespacedName, &n)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	res := r.Handler(r.Logger, &n)
	switch res {
	case SyncStateError:
		return ctrl.Result{}, nil
	case SyncStateReprocessAll:
		level.Error(r.Logger).Log("controller", "NodeReconciler", "error", "unexpected result reprocess all")
		return ctrl.Result{}, nil
	case SyncStateErrorNoRetry:
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Node{}).
		Complete(r)
}
