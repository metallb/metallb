// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kit/log"
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	frrk8s "go.universe.tf/metallb/internal/bgp/frrk8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestNewFRRK8sReconcilerPredicate(t *testing.T) {
	const frrk8sNamespace = "frr-k8s-system"

	p := NewFRRK8sReconcilerPredicate(frrk8sNamespace)

	frrConfig := func(name, namespace string) *frrv1beta1.FRRConfiguration {
		return &frrv1beta1.FRRConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		}
	}

	tests := map[string]struct {
		event any
		want  bool
	}{
		"CreateFunc own config": {
			event: event.CreateEvent{Object: frrConfig(frrk8s.ConfigName("node1"), frrk8sNamespace)},
			want:  true,
		},
		"CreateFunc other config in frr-k8s namespace": {
			event: event.CreateEvent{Object: frrConfig("generic", frrk8sNamespace)},
			want:  true,
		},
		"UpdateFunc other config in frr-k8s namespace": {
			event: event.UpdateEvent{
				ObjectOld: frrConfig("generic", frrk8sNamespace),
				ObjectNew: frrConfig("generic", frrk8sNamespace),
			},
			want: true,
		},
		"DeleteFunc other config in frr-k8s namespace": {
			event: event.DeleteEvent{Object: frrConfig("generic", frrk8sNamespace)},
			want:  true,
		},
		"DeleteFunc config in another namespace": {
			event: event.DeleteEvent{Object: frrConfig("generic", "other-namespace")},
			want:  false,
		},
		"UpdateFunc config in another namespace": {
			event: event.UpdateEvent{
				ObjectOld: frrConfig("generic", "other-namespace"),
				ObjectNew: frrConfig("generic", "other-namespace"),
			},
			want: false,
		},
		"GenericFunc reload event": {
			event: event.GenericEvent{Object: NewFRRK8sConfigEvent().Object},
			want:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var got bool
			switch e := test.event.(type) {
			case event.CreateEvent:
				got = p.Create(e)
			case event.UpdateEvent:
				got = p.Update(e)
			case event.DeleteEvent:
				got = p.Delete(e)
			case event.GenericEvent:
				got = p.Generic(e)
			default:
				t.Fatalf("unknown event type: %T", e)
			}

			if got != test.want {
				t.Errorf("predicate returned %v, want %v", got, test.want)
			}
		})
	}
}

// When the frr-k8s webhook rejects MetalLB's configuration, a later event for
// the conflicting configuration must be enough to create it.
func TestFRRK8sReconcilerRetriesOnOtherConfigEvent(t *testing.T) {
	const (
		frrk8sNamespace = "frr-k8s-system"
		nodeName        = "node1"
	)

	scheme := runtime.NewScheme()
	if err := frrv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("frrv1beta1: add to scheme failed: %v", err)
	}

	rejectCreate := true
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if rejectCreate {
					return apierrors.NewForbidden(frrv1beta1.GroupVersion.WithResource("frrconfigurations").GroupResource(),
						obj.GetName(), errors.New("conflicts with frrconfig-generic"))
				}
				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()

	desired := frrv1beta1.FRRConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: frrk8s.ConfigName(nodeName), Namespace: frrk8sNamespace},
		Spec: frrv1beta1.FRRConfigurationSpec{
			BGP: frrv1beta1.BGPConfig{
				Routers: []frrv1beta1.Router{{ASN: 65000}},
			},
		},
	}

	r := &FRRK8sReconciler{
		Client:               cl,
		Logger:               log.NewNopLogger(),
		Scheme:               scheme,
		NodeName:             nodeName,
		FRRK8sNamespace:      frrk8sNamespace,
		desiredConfiguration: desired.DeepCopy(),
	}

	ctx := context.Background()
	if _, err := r.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Name: desired.Name, Namespace: frrk8sNamespace},
	}); err == nil {
		t.Fatalf("expected the first reconcile to fail while the webhook rejects the configuration")
	}

	// The conflicting configuration is fixed: its event reaches the reconciler.
	rejectCreate = false
	if _, err := r.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "frrconfig-generic", Namespace: frrk8sNamespace},
	}); err != nil {
		t.Fatalf("reconcile after the conflict was resolved failed: %v", err)
	}

	got := frrv1beta1.FRRConfiguration{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(&desired), &got); err != nil {
		t.Fatalf("expected %s to be created: %v", desired.Name, err)
	}
	if len(got.Spec.BGP.Routers) != 1 || got.Spec.BGP.Routers[0].ASN != 65000 {
		t.Errorf("unexpected spec for %s: %+v", desired.Name, got.Spec)
	}
}
