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
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ConfigReconciler struct {
	client.Client
	Logger         log.Logger
	Scheme         *runtime.Scheme
	Namespace      string
	Handler        func(log.Logger, *config.Config) SyncState
	ValidateConfig config.Validate
	ForceReload    func()
	BGPType        string
}

func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "ConfigReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "ConfigReconciler", "end reconcile", req.NamespacedName.String())

	var addressPools metallbv1beta1.AddressPoolList
	if err := r.List(ctx, &addressPools, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get addresspools", "error", err)
		return ctrl.Result{}, err
	}

	var ipAddressPools metallbv1beta1.IPAddressPoolList
	if err := r.List(ctx, &ipAddressPools, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to get ipaddresspools", "error", err)
		return ctrl.Result{}, err
	}

	var bgpPeers metallbv1beta2.BGPPeerList
	if err := r.List(ctx, &bgpPeers, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get bgppeers", "error", err)
		return ctrl.Result{}, err
	}

	var bfdProfiles metallbv1beta1.BFDProfileList
	if err := r.List(ctx, &bfdProfiles, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get bfdprofiles", "error", err)
		return ctrl.Result{}, err
	}

	var l2Advertisements metallbv1beta1.L2AdvertisementList
	if err := r.List(ctx, &l2Advertisements, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get l2 advertisements", "error", err)
		return ctrl.Result{}, err
	}

	var bgpAdvertisements metallbv1beta1.BGPAdvertisementList
	if err := r.List(ctx, &bgpAdvertisements, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get bgp advertisements", "error", err)
		return ctrl.Result{}, err
	}

	var communities metallbv1beta1.CommunityList
	if err := r.List(ctx, &communities, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to get communities", "error", err)
		return ctrl.Result{}, err
	}

	secrets, err := r.getSecrets(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get nodes", "error", err)
		return ctrl.Result{}, err
	}

	resources := config.ClusterResources{
		Pools:              ipAddressPools.Items,
		Peers:              bgpPeers.Items,
		BFDProfiles:        bfdProfiles.Items,
		L2Advs:             l2Advertisements.Items,
		BGPAdvs:            bgpAdvertisements.Items,
		LegacyAddressPools: addressPools.Items,
		Communities:        communities.Items,
		PasswordSecrets:    secrets,
		Nodes:              nodes.Items,
	}

	level.Debug(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs and Secrets", dumpClusterResources(&resources))

	cfg, err := config.For(resources, r.ValidateConfig)
	if err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to parse the configuration", "error", err)
		return ctrl.Result{}, nil
	}

	level.Debug(r.Logger).Log("controller", "ConfigReconciler", "rendered config", spew.Sdump(cfg))

	res := r.Handler(r.Logger, cfg)
	switch res {
	case SyncStateError:
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs and Secrets", dumpClusterResources(&resources), "event", "reload failed, retry")
		return ctrl.Result{}, retryError
	case SyncStateReprocessAll:
		level.Info(r.Logger).Log("controller", "ConfigReconciler", "event", "force service reload")
		r.ForceReload()
	case SyncStateErrorNoRetry:
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs and Secrets", dumpClusterResources(&resources), "event", "reload failed, no retry")
		return ctrl.Result{}, nil
	}

	level.Info(r.Logger).Log("controller", "ConfigReconciler", "event", "config reloaded")
	return ctrl.Result{}, nil
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1beta2.BGPPeer{}).
		Watches(&source.Kind{Type: &metallbv1beta1.IPAddressPool{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &metallbv1beta1.BGPAdvertisement{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &metallbv1beta1.L2Advertisement{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &metallbv1beta1.BFDProfile{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &metallbv1beta1.AddressPool{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &metallbv1beta1.Community{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

func (r *ConfigReconciler) getSecrets(ctx context.Context) (map[string]corev1.Secret, error) {
	var secrets corev1.SecretList
	if err := r.List(ctx, &secrets, client.InNamespace(r.Namespace)); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to get secrets", "error", err)
		return nil, err
	}
	secretsMap := make(map[string]corev1.Secret)
	for _, secret := range secrets.Items {
		secretsMap[secret.Name] = secret
	}
	return secretsMap, nil
}
