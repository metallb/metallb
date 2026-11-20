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
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	frrk8s "go.universe.tf/metallb/internal/bgp/frrk8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// FRRK8sConfigCleaner deletes MetalLB-generated FRRConfigurations for nodes
// that should no longer run a speaker. The speaker on the affected node may
// already be gone: label changes remove its pod, and the process does not get
// a chance to withdraw the configuration it published. Any remaining speaker
// watches the DaemonSet and the nodes and removes those objects, which makes
// FRR-K8s tear down the BGP sessions.
type FRRK8sConfigCleaner struct {
	client.Client
	Logger             log.Logger
	FRRK8sNamespace    string
	DaemonSetName      string
	DaemonSetNamespace string
}

func (r *FRRK8sConfigCleaner) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "FRRK8sConfigCleaner", "start reconcile", req.String())
	defer level.Info(r.Logger).Log("controller", "FRRK8sConfigCleaner", "end reconcile", req.String())
	updates.Inc()

	if r.DaemonSetName == "" || r.DaemonSetNamespace == "" {
		return ctrl.Result{}, nil
	}

	cfg := &frrv1beta1.FRRConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	nodeName, owned := metallbFRRNode(cfg)
	if !owned || cfg.Namespace != r.FRRK8sNamespace {
		return ctrl.Result{}, nil
	}

	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, client.ObjectKey{Name: r.DaemonSetName, Namespace: r.DaemonSetNamespace}, ds); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		updateErrors.Inc()
		level.Error(r.Logger).Log("controller", "FRRK8sConfigCleaner", "event", "failed to get speaker daemonset", "error", err)
		return ctrl.Result{}, err
	}

	node := &corev1.Node{}
	if err := r.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		if !apierrors.IsNotFound(err) {
			updateErrors.Inc()
			level.Error(r.Logger).Log("controller", "FRRK8sConfigCleaner", "event", "failed to get node", "node", nodeName, "error", err)
			return ctrl.Result{}, err
		}
		node = nil
	}
	if node != nil && nodeContinuesRunningDaemonPod(node, ds) {
		return ctrl.Result{}, nil
	}

	level.Info(r.Logger).Log("controller", "FRRK8sConfigCleaner", "event", "deleting FRR configuration for node that should not run a speaker", "node", nodeName, "config", cfg.Name, "namespace", cfg.Namespace)
	if err := r.Delete(ctx, cfg); err != nil {
		updateErrors.Inc()
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

func (r *FRRK8sConfigCleaner) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("frrk8sconfigcleaner").
		For(&frrv1beta1.FRRConfiguration{}, builder.WithPredicates(r.ownedConfigPredicate())).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.enqueueOwnedConfigs), builder.WithPredicates(nodeSchedulingChangedPredicate(""))).
		Watches(&appsv1.DaemonSet{}, handler.EnqueueRequestsFromMapFunc(r.enqueueOwnedConfigs), builder.WithPredicates(daemonSetSchedulingPredicate(r.DaemonSetNamespace, r.DaemonSetName))).
		Complete(r)
}

func (r *FRRK8sConfigCleaner) ownedConfigPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		cfg, ok := obj.(*frrv1beta1.FRRConfiguration)
		if !ok || cfg.Namespace != r.FRRK8sNamespace {
			return false
		}
		_, owned := metallbFRRNode(cfg)
		return owned
	})
}

func (r *FRRK8sConfigCleaner) enqueueOwnedConfigs(ctx context.Context, _ client.Object) []ctrl.Request {
	if r.DaemonSetName == "" {
		return nil
	}
	list := &frrv1beta1.FRRConfigurationList{}
	if err := r.List(ctx, list, client.InNamespace(r.FRRK8sNamespace)); err != nil {
		level.Error(r.Logger).Log("controller", "FRRK8sConfigCleaner", "event", "failed to list FRR configurations", "error", err)
		return nil
	}
	reqs := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		if _, owned := metallbFRRNode(&list.Items[i]); !owned {
			continue
		}
		reqs = append(reqs, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
	}
	return reqs
}

// metallbFRRNode returns the node a speaker-generated FRRConfiguration targets.
// MetalLB names the object metallb-<node> and pins it with a hostname selector.
// Both have to agree so a user-authored configuration is left alone.
func metallbFRRNode(cfg *frrv1beta1.FRRConfiguration) (string, bool) {
	if cfg == nil {
		return "", false
	}
	nodeName := cfg.Spec.NodeSelector.MatchLabels[corev1.LabelHostname]
	if nodeName == "" || cfg.Name != frrk8s.ConfigName(nodeName) {
		return "", false
	}
	return nodeName, true
}
