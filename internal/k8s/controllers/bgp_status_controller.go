// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type PeersForService func(key string) sets.Set[string]

type bgpStatusEvent struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (evt *bgpStatusEvent) DeepCopyObject() runtime.Object {
	res := new(bgpStatusEvent)
	res.Name = evt.Name
	res.Namespace = evt.Namespace
	return res
}

func NewBGPStatusEvent(namespace, name string) event.GenericEvent {
	evt := bgpStatusEvent{}
	evt.Name = name
	evt.Namespace = namespace
	return event.GenericEvent{Object: &evt}
}

type ServiceBGPStatusReconciler struct {
	client.Client
	Logger        log.Logger
	NodeName      string
	Namespace     string
	SpeakerPod    *v1.Pod
	ReconcileChan <-chan event.GenericEvent
	PeersFetcher  PeersForService
}

func (r *ServiceBGPStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "ServiceBGPStatus", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "ServiceBGPStatus", "end reconcile", req.NamespacedName.String())

	serviceName, serviceNamespace := req.Name, req.Namespace

	var serviceBGPStatuses v1beta1.ServiceBGPStatusList
	err := r.Client.List(ctx, &serviceBGPStatuses, client.MatchingFields{
		serviceIndexName: indexFor(serviceNamespace, serviceName, r.NodeName),
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	peers := r.PeersFetcher(req.NamespacedName.String())
	if peers.Len() == 0 {
		errs := []error{}
		for i := range serviceBGPStatuses.Items {
			if serviceBGPStatuses.Items[i].Labels[LabelAnnounceNode] != r.NodeName { // shouldn't happen because of the indexing, just in case
				continue
			}
			if err := r.Client.Delete(ctx, &serviceBGPStatuses.Items[i]); err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, err)
			}
		}

		return ctrl.Result{}, utilerrors.NewAggregate(errs)
	}

	deleteRedundantErrs := []error{}
	if len(serviceBGPStatuses.Items) > 1 {
		// We shouldn't get here, just in case the controller created redundant resources
		for i := range serviceBGPStatuses.Items[1:] {
			if serviceBGPStatuses.Items[i+1].Labels[LabelAnnounceNode] != r.NodeName {
				continue
			}
			if err := r.Client.Delete(ctx, &serviceBGPStatuses.Items[i+1]); err != nil && !apierrors.IsNotFound(err) {
				deleteRedundantErrs = append(deleteRedundantErrs, err)
			}
		}
	}

	if len(deleteRedundantErrs) > 0 {
		return ctrl.Result{}, utilerrors.NewAggregate(deleteRedundantErrs)
	}

	var state = &v1beta1.ServiceBGPStatus{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "bgp-",
			Namespace:    r.Namespace,
		},
	}

	if len(serviceBGPStatuses.Items) > 0 {
		state = &serviceBGPStatuses.Items[0]
	}

	desiredStatus := v1beta1.MetalLBServiceBGPStatus{
		Node:             r.NodeName,
		ServiceName:      serviceName,
		ServiceNamespace: serviceNamespace,
		Peers:            sets.List(peers),
	}

	if reflect.DeepEqual(state.Status, desiredStatus) {
		return ctrl.Result{}, nil
	}

	var result controllerutil.OperationResult
	result, err = controllerutil.CreateOrPatch(ctx, r.Client, state, func() error {
		state.Labels = map[string]string{
			LabelAnnounceNode:     r.NodeName,
			LabelServiceName:      serviceName,
			LabelServiceNamespace: serviceNamespace,
		}
		state.Status = desiredStatus
		err = controllerutil.SetOwnerReference(r.SpeakerPod, state, r.Scheme())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if result == controllerutil.OperationResultCreated {
		// According to https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/controller/controllerutil#CreateOrPatch
		// If the object is created, we have to patch it again to ensure the status is created.
		// This will happen when we reconcile the creation event.
		level.Debug(r.Logger).Log("controller", "ServiceBGPStatus", "created state", dumpResource(state))
		return ctrl.Result{}, nil
	}

	level.Debug(r.Logger).Log("controller", "ServiceBGPStatus", "updated state", dumpResource(state))
	return ctrl.Result{}, nil
}

func (r *ServiceBGPStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.NewPredicateFuncs(func(o client.Object) bool {
		_, ok := o.(*v1beta1.ServiceBGPStatus)
		if !ok {
			return true
		}

		labels := o.GetLabels()

		if labels == nil {
			level.Error(r.Logger).Log("controller", "ServiceBGPStatus", "object has no labels", o)
			return false
		}

		if _, ok = labels[LabelServiceName]; !ok {
			level.Error(r.Logger).Log("controller", "ServiceBGPStatus", "object does not have the servicename label", o)
			return false
		}

		if _, ok = labels[LabelServiceNamespace]; !ok {
			level.Error(r.Logger).Log("controller", "ServiceBGPStatus", "object does not have the servicenamespace label", o)
			return false
		}

		var node string
		if node, ok = labels[LabelAnnounceNode]; !ok {
			level.Error(r.Logger).Log("controller", "ServiceBGPStatus", "object does not have the node name label", o)
			return false
		}

		if node != r.NodeName {
			return false
		}

		return true
	})

	err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1beta1.ServiceBGPStatus{}, serviceIndexName,
		func(o client.Object) []string {
			s, ok := o.(*v1beta1.ServiceBGPStatus)
			if s == nil {
				level.Error(r.Logger).Log("controller", "fieldindexer", "error", "received nil ServiceBGPStatus")
				return nil
			}

			if !ok {
				level.Error(r.Logger).Log("controller", "fieldindexer", "error", "received object that is not ServiceBGPStatus", "object", o)
				return nil
			}

			labels := s.GetLabels()
			if labels == nil {
				level.Error(r.Logger).Log("controller", "fieldindexer", "error", "received ServiceBGPStatus without labels", "object", o)
				return nil
			}

			return []string{indexFor(labels[LabelServiceNamespace], labels[LabelServiceName], labels[LabelAnnounceNode])}
		})

	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("servicebgpstatus").
		Watches(&v1beta1.ServiceBGPStatus{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, object client.Object) []reconcile.Request {
				level.Debug(r.Logger).Log("controller", "ServiceBGPStatus", "enqueueing", "object", object)
				labels := object.GetLabels()
				return []reconcile.Request{{NamespacedName: types.NamespacedName{
					Name:      labels[LabelServiceName],
					Namespace: labels[LabelServiceNamespace],
				}}}
			})).
		WatchesRawSource(source.Channel(r.ReconcileChan, &handler.EnqueueRequestForObject{})).
		WithEventFilter(p).
		Complete(r)
}

func indexFor(svcNs, svcName, node string) string {
	return fmt.Sprintf("%s/%s-%s", svcNs, svcName, node)
}
