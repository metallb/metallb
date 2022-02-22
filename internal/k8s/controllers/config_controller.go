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
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/config"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const RetryPeriod = 10 * time.Second

type ConfigReconciler struct {
	client.Client
	Logger         log.Logger
	Scheme         *runtime.Scheme
	Namespace      string
	Handler        func(log.Logger, *config.Config) SyncState
	ValidateConfig config.Validate
	ForceReload    func()
}

//+kubebuilder:rbac:groups=metallb.io,resources=bgppeers,verbs=get;list;watch;
//+kubebuilder:rbac:groups=metallb.io,resources=addresspools,verbs=get;list;watch;create;
//+kubebuilder:rbac:groups=metallb.io,resources=bfdprofiles,verbs=get;list;watch;create;

func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "ConfigReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "ConfigReconciler", "end reconcile", req.NamespacedName.String())

	var addressPools metallbv1beta1.AddressPoolList
	if err := r.List(ctx, &addressPools, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to get addresspools", "error", err)
		return ctrl.Result{}, err
	}

	var bgpPeers metallbv1beta1.BGPPeerList
	if err := r.List(ctx, &bgpPeers, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to get bgppeers", "error", err)
		return ctrl.Result{}, err
	}

	var bfdProfiles metallbv1beta1.BFDProfileList
	if err := r.List(ctx, &bfdProfiles, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to get bfdprofiles", "error", err)
		return ctrl.Result{}, err
	}

	metallbCRs := config.ClusterResources{
		Pools:       addressPools.Items,
		Peers:       bgpPeers.Items,
		BFDProfiles: bfdProfiles.Items,
	}

	level.Debug(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs", spew.Sdump(metallbCRs))

	cfg, err := config.For(metallbCRs, r.ValidateConfig)
	if err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to parse the configuration", "error", err)
		return ctrl.Result{}, nil
	}

	level.Debug(r.Logger).Log("controller", "ConfigReconciler", "rendered config", spew.Sdump(cfg))

	res := r.Handler(r.Logger, cfg)
	switch res {
	case SyncStateError:
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs", spew.Sdump(metallbCRs), "event", "reload failed, retry")
		return ctrl.Result{}, retryError
	case SyncStateReprocessAll:
		level.Info(r.Logger).Log("controller", "ConfigReconciler", "event", "force service reload")
		r.ForceReload()
	case SyncStateErrorNoRetry:
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs", spew.Sdump(metallbCRs), "event", "reload failed, no retry")
		return ctrl.Result{}, nil
	}

	level.Info(r.Logger).Log("controller", "ConfigReconciler", "event", "config reloaded")
	return ctrl.Result{}, nil
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetNamespace() == r.Namespace
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetNamespace() == r.Namespace
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetNamespace() == r.Namespace
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return e.Object.GetNamespace() == r.Namespace
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1beta1.BGPPeer{}, builder.WithPredicates(p)).
		Watches(&source.Kind{Type: &metallbv1beta1.AddressPool{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(p)).
		Watches(&source.Kind{Type: &metallbv1beta1.BFDProfile{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(p)).
		Complete(r)
}
