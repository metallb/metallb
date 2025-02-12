// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilErrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/layer2"
)

type L2StatusFetcher func(types.NamespacedName) []layer2.IPAdvertisement

type l2StatusEvent struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

const (
	serviceIndexName = "status.serviceName"
)

func (evt *l2StatusEvent) DeepCopyObject() runtime.Object {
	res := new(l2StatusEvent)
	res.Name = evt.Name
	res.Namespace = evt.Namespace
	return res
}

func NewL2StatusEvent(namespace, name string) event.GenericEvent {
	evt := l2StatusEvent{}
	evt.Name = name
	evt.Namespace = namespace
	return event.GenericEvent{Object: &evt}
}

type Layer2StatusReconciler struct {
	client.Client
	Logger        log.Logger
	NodeName      string
	Namespace     string
	SpeakerPod    *v1.Pod
	ReconcileChan <-chan event.GenericEvent
	// fetch ipAdv object to get interface info
	StatusFetcher L2StatusFetcher
}

func (r *Layer2StatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "Layer2StatusReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "Layer2StatusReconciler", "end reconcile", req.NamespacedName.String())

	serviceName, serviceNamespace := req.Name, req.Namespace

	ipAdvS := r.StatusFetcher(types.NamespacedName{Name: serviceName, Namespace: serviceNamespace})
	var serviceL2statuses v1beta1.ServiceL2StatusList
	if err := r.Client.List(ctx, &serviceL2statuses, client.MatchingFields{
		serviceIndexName: types.NamespacedName{Name: serviceName, Namespace: serviceNamespace}.String(),
	}); err != nil {
		return ctrl.Result{}, err
	}

	var errs []error
	if len(ipAdvS) == 0 {
		for key, item := range serviceL2statuses.Items {
			if item.Status.Node != r.NodeName {
				continue
			}
			if err := r.Client.Delete(ctx, &serviceL2statuses.Items[key]); err != nil && !errors.IsNotFound(err) {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return ctrl.Result{}, utilErrors.NewAggregate(errs)
		}
		return ctrl.Result{}, nil
	}

	// creating a brand new cr
	var state = &v1beta1.ServiceL2Status{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "l2-",
			Namespace:    r.Namespace,
		},
	}
	// update an existing cr
	if len(serviceL2statuses.Items) > 0 {
		state = &serviceL2statuses.Items[0]
	}

	desiredStatus := r.buildDesiredStatus(ipAdvS, serviceName, serviceNamespace)
	if reflect.DeepEqual(state.Status, desiredStatus) {
		return ctrl.Result{}, nil
	}

	var result controllerutil.OperationResult
	var err error
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
		// If the object is created for the first time, we have to requeue it to ensure that the status is updated.
		return ctrl.Result{Requeue: true}, nil
	}

	level.Debug(r.Logger).Log("controller", "Layer2StatusReconciler", "updated state", dumpResource(state))
	return ctrl.Result{}, nil
}

func (r *Layer2StatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.NewPredicateFuncs(func(object client.Object) bool {
		if s, ok := object.(*v1beta1.ServiceL2Status); ok {
			// only objects with complete labels that can illustrate which service it is related to
			// can trigger the reconciler
			label := s.GetLabels()
			if label == nil {
				level.Error(r.Logger).Log("controller", "Layer2StatusReconciler", "missing meta", "object", object)
				return false
			}
			if _, ok = label[LabelServiceName]; !ok {
				level.Error(r.Logger).Log("controller", "Layer2StatusReconciler", "missing meta", "object", object)
				return false
			}
			if _, ok = label[LabelServiceNamespace]; !ok {
				level.Error(r.Logger).Log("controller", "Layer2StatusReconciler", "missing meta", "object", object)
				return false
			}
			var node string
			if node, ok = label[LabelAnnounceNode]; !ok {
				level.Error(r.Logger).Log("controller", "Layer2StatusReconciler", "missing meta", "object", object)
				return false
			}
			// only trigger the reconciler if the service is announced by this node
			if node != r.NodeName {
				return false
			}
		}
		return true
	})
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1beta1.ServiceL2Status{}, serviceIndexName,
		func(rawObj client.Object) []string {
			s, ok := rawObj.(*v1beta1.ServiceL2Status)
			if s == nil {
				level.Error(r.Logger).Log("controller", "fieldindexer", "error", "received nil ServiceL2Status")
				return nil
			}
			if !ok {
				level.Error(r.Logger).Log("controller", "fieldindexer", "error", "received object that is not ServiceL2Status", "object", rawObj.GetObjectKind().GroupVersionKind().Kind)
				return nil
			}
			label := s.GetLabels()
			if label == nil {
				level.Error(r.Logger).Log("controller", "fieldindexer", "error", "received ServiceL2Status without label", "meta", fmt.Sprintf("%s/%s", s.Name, s.Namespace))
				return nil
			}
			return []string{types.NamespacedName{
				Name:      label[LabelServiceName],
				Namespace: label[LabelServiceNamespace]}.String()}
		}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("servicel2status").
		// for crs, we build meta from cr label which indicate the service information
		Watches(&v1beta1.ServiceL2Status{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, object client.Object) []reconcile.Request {
				level.Debug(r.Logger).Log("controller", "Layer2StatusReconciler", "enqueueing", "object", object)
				label := object.GetLabels()
				return []reconcile.Request{{NamespacedName: types.NamespacedName{
					Name:      label[LabelServiceName],
					Namespace: label[LabelServiceNamespace],
				}}}
			})).
		// for events from channel, use the meta directly
		WatchesRawSource(source.Channel(r.ReconcileChan, &handler.EnqueueRequestForObject{})).
		WithEventFilter(p).
		Complete(r)
}

func (r *Layer2StatusReconciler) buildDesiredStatus(
	advertisements []layer2.IPAdvertisement,
	serviceName,
	serviceNamespace string,
) v1beta1.MetalLBServiceL2Status {
	// todo: add advertise ip or not?
	s := v1beta1.MetalLBServiceL2Status{
		Node:             r.NodeName,
		ServiceName:      serviceName,
		ServiceNamespace: serviceNamespace,
	}
	// multiple advertisement objects share all fields except lb ip, so we use the first one
	adv := advertisements[0]
	if !adv.IsAllInterfaces() {
		for inf := range adv.GetInterfaces() {
			s.Interfaces = append(s.Interfaces, v1beta1.InterfaceInfo{Name: inf})
		}
	}
	return s
}
