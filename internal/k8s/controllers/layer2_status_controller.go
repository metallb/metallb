// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

type StatusFetcher func(types.NamespacedName) []layer2.IPAdvertisement

type l2StatusEvent struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

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
	ReconcileChan <-chan event.GenericEvent
	// fetch ipAdv object to get interface info
	StatusFetcher StatusFetcher
}

func (r *Layer2StatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	level.Info(r.Logger).Log("controller", "Layer2StatusReconciler", "start reconcile", req.NamespacedName.String())
	defer level.Info(r.Logger).Log("controller", "Layer2StatusReconciler", "end reconcile", req.NamespacedName.String())

	svcName := strings.TrimSuffix(req.Name, fmt.Sprintf("-%s", r.NodeName))
	ipAdvS := r.StatusFetcher(types.NamespacedName{Name: svcName, Namespace: req.Namespace})

	if len(ipAdvS) == 0 {
		err := r.Client.Delete(ctx, &v1beta1.ServiceL2Status{ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: req.Namespace}})
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	state := &v1beta1.ServiceL2Status{ObjectMeta: metav1.ObjectMeta{
		Name:      req.Name,
		Namespace: req.Namespace,
	}}
	err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, state)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	desiredStatus := r.buildDesiredStatus(ipAdvS)
	if reflect.DeepEqual(state.Status, desiredStatus) {
		return ctrl.Result{}, nil
	}

	var result controllerutil.OperationResult
	result, err = controllerutil.CreateOrPatch(ctx, r.Client, state, func() error {
		state.Labels = map[string]string{LabelAnnounceNode: r.NodeName, LabelServiceName: svcName}
		state.Status = desiredStatus
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
			return strings.HasSuffix(s.Name, r.NodeName)
		}
		return true
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.ServiceL2Status{}).
		WatchesRawSource(&source.Channel{Source: r.ReconcileChan}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, object client.Object) []reconcile.Request {
				evt, ok := object.(*l2StatusEvent)
				if !ok {
					level.Error(r.Logger).Log("controller", "Layer2StatusReconciler", "received an object that is not a l2StatusEvent from channel", "object", object)
					return []reconcile.Request{}
				}
				level.Debug(r.Logger).Log("controller", "Layer2StatusReconciler", "enqueueing", "object", evt)
				// unify req.Name to svcName-nodeName
				return []reconcile.Request{{NamespacedName: types.NamespacedName{
					Name:      fmt.Sprintf("%s-%s", evt.Name, r.NodeName),
					Namespace: evt.Namespace}}}
			})).
		WithEventFilter(p).
		Complete(r)
}

func (r *Layer2StatusReconciler) buildDesiredStatus(advertisements []layer2.IPAdvertisement) v1beta1.MetalLBServiceL2Status {
	// todo: add advertise ip or not?
	s := v1beta1.MetalLBServiceL2Status{
		Node: r.NodeName,
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
