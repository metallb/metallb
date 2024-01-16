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
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const bgpExtrasConfigName = "bgpextras"

type ConfigReconciler struct {
	client.Client
	Logger         log.Logger
	Scheme         *runtime.Scheme
	Namespace      string
	Handler        func(log.Logger, *config.Config) SyncState
	ValidateConfig config.Validate
	ForceReload    func()
	BGPType        string
	currentConfig  *config.Config
}

func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return requestHandler(r, ctx, req)
}

var requestHandler = func(r *ConfigReconciler, ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "ConfigReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "ConfigReconciler", "end reconcile", req.NamespacedName.String())
	updates.Inc()

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

	var namespaces corev1.NamespaceList
	if err := r.List(ctx, &namespaces); err != nil {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get namespaces", "error", err)
		return ctrl.Result{}, err
	}

	var extrasMap corev1.ConfigMap
	key := client.ObjectKey{Name: bgpExtrasConfigName, Namespace: r.Namespace}
	if err := r.Get(ctx, key, &extrasMap); err != nil && !apierrors.IsNotFound(err) {
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "message", "failed to get the frr configmap", "error", err)
		return ctrl.Result{}, err
	}

	resources := config.ClusterResources{
		Pools:           ipAddressPools.Items,
		Peers:           bgpPeers.Items,
		BFDProfiles:     bfdProfiles.Items,
		L2Advs:          l2Advertisements.Items,
		BGPAdvs:         bgpAdvertisements.Items,
		Communities:     communities.Items,
		PasswordSecrets: secrets,
		Nodes:           nodes.Items,
		Namespaces:      namespaces.Items,
		BGPExtras:       extrasMap,
	}

	level.Debug(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs and Secrets", dumpClusterResources(&resources))

	cfg, err := toConfig(resources, r.ValidateConfig)
	if err != nil {
		configStale.Set(1)
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "error", "failed to parse the configuration", "error", err)
		return ctrl.Result{}, nil
	}

	if cfg.BGPExtras != "" {
		level.Info(r.Logger).Log("controller", "ConfigReconciler", "warning message", "BGP Extras provided, please note that this configuration is not supported and used at your own risk")
	}
	level.Debug(r.Logger).Log("controller", "ConfigReconciler", "rendered config", dumpConfig(cfg))
	if r.currentConfig != nil && reflect.DeepEqual(r.currentConfig, cfg) {
		level.Debug(r.Logger).Log("controller", "ConfigReconciler", "event", "configuration did not change, ignoring")
		return ctrl.Result{}, nil
	}

	r.currentConfig = cfg

	res := r.Handler(r.Logger, cfg)
	switch res {
	case SyncStateError:
		configStale.Set(1)
		updateErrors.Inc()
		// if the configuration load failed, we reset the current config because this is gonna lead to a retry
		// of the reconciliaton loop. If we don't reset, the retry will find the config identical and will exit,
		// which is not what we want here.
		r.currentConfig = nil
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs and Secrets", dumpClusterResources(&resources), "event", "reload failed, retry")
		return ctrl.Result{}, errRetry
	case SyncStateReprocessAll:
		level.Info(r.Logger).Log("controller", "ConfigReconciler", "event", "force service reload")
		r.ForceReload()
	case SyncStateErrorNoRetry:
		configStale.Set(1)
		updateErrors.Inc()
		level.Error(r.Logger).Log("controller", "ConfigReconciler", "metallb CRs and Secrets", dumpClusterResources(&resources), "event", "reload failed, no retry")
		return ctrl.Result{}, nil
	}

	configLoaded.Set(1)
	configStale.Set(0)
	level.Info(r.Logger).Log("controller", "ConfigReconciler", "event", "config reloaded")
	return ctrl.Result{}, nil
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return filterNodeEvent(e) && filterNamespaceEvent(e) && filterConfigmapEvent(e)
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1beta2.BGPPeer{}).
		Watches(&metallbv1beta1.IPAddressPool{}, &handler.EnqueueRequestForObject{}).
		Watches(&corev1.Node{}, &handler.EnqueueRequestForObject{}).
		Watches(&metallbv1beta1.BGPAdvertisement{}, &handler.EnqueueRequestForObject{}).
		Watches(&metallbv1beta1.L2Advertisement{}, &handler.EnqueueRequestForObject{}).
		Watches(&metallbv1beta1.BFDProfile{}, &handler.EnqueueRequestForObject{}).
		Watches(&metallbv1beta1.Community{}, &handler.EnqueueRequestForObject{}).
		Watches(&corev1.Secret{}, &handler.EnqueueRequestForObject{}).
		Watches(&corev1.Namespace{}, &handler.EnqueueRequestForObject{}).
		Watches(&corev1.ConfigMap{}, &handler.EnqueueRequestForObject{}).
		WithEventFilter(p).
		Complete(r)
}

func filterNodeEvent(e event.UpdateEvent) bool {
	newNodeObj, ok := e.ObjectNew.(*corev1.Node)
	if !ok {
		return true
	}
	oldNodeObj, ok := e.ObjectOld.(*corev1.Node)
	if !ok {
		return true
	}
	if labels.Equals(labels.Set(oldNodeObj.Labels), labels.Set(newNodeObj.Labels)) {
		return false
	}
	return true
}

func filterNamespaceEvent(e event.UpdateEvent) bool {
	newNamespaceObj, ok := e.ObjectNew.(*corev1.Namespace)
	if !ok {
		return true
	}
	oldNamespaceObj, ok := e.ObjectOld.(*corev1.Namespace)
	if !ok {
		return true
	}
	// If there is no changes in namespace labels, ignore event.
	if labels.Equals(labels.Set(oldNamespaceObj.Labels), labels.Set(newNamespaceObj.Labels)) {
		return false
	}
	return true
}

func filterConfigmapEvent(e event.UpdateEvent) bool {
	cm, ok := e.ObjectNew.(*corev1.ConfigMap)
	if !ok {
		return true
	}
	if cm.Name != bgpExtrasConfigName {
		return false
	}
	return true
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
