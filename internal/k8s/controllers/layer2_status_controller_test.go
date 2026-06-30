// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"net"

	"github.com/go-kit/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/layer2"
)

// churnInjectClient wraps a real client to make the #3063 reconcile-loop race
// deterministic. It does two things the production environment does on its own:
//
//  1. It emulates the manager field index ("status.serviceName") that the
//     reconciler lists by - the envtest direct client has no such index, so we
//     resolve the ServiceL2StatusList by the service-identifying labels instead.
//  2. When armed, it deletes the listed statuses right after returning them,
//     reproducing another speaker that briefly became leader and deleted this
//     node's status between our List and the CreateOrPatch that follows.
type churnInjectClient struct {
	client.Client
	serviceKey   string
	deleteOnList bool
}

func (c *churnInjectClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	sl, ok := list.(*v1beta1.ServiceL2StatusList)
	if !ok {
		return c.Client.List(ctx, list, opts...)
	}

	// opts are intentionally not forwarded: the production reconciler filters by
	// the manager field index (client.MatchingFields), which envtest's direct
	// client cannot satisfy, so we re-resolve the same set by label here.
	all := &v1beta1.ServiceL2StatusList{}
	if err := c.Client.List(ctx, all); err != nil {
		return err
	}
	for i := range all.Items {
		// Value copy on purpose: the reconciler must see a status carrying the
		// real resourceVersion (that is the object the bug mis-handles).
		item := all.Items[i]
		key := types.NamespacedName{
			Namespace: item.Labels[LabelServiceNamespace],
			Name:      item.Labels[LabelServiceName],
		}.String()
		if key == c.serviceKey {
			sl.Items = append(sl.Items, item)
		}
	}

	if c.deleteOnList {
		c.deleteOnList = false
		for i := range sl.Items {
			if err := c.Delete(ctx, sl.Items[i].DeepCopy()); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

var _ = Describe("Layer2StatusReconciler reconcile-loop regression (#3063)", func() {
	const (
		churnNode = "churn-node"
		churnSvc  = "churn-svc"
		churnNS   = "churn-ns"
	)

	cleanup := func() {
		_ = k8sClient.DeleteAllOf(ctx, &v1beta1.ServiceL2Status{},
			client.InNamespace(speakerNamespace),
			client.MatchingLabels{LabelServiceName: churnSvc})
	}

	AfterEach(cleanup)

	It("recovers without a stale resourceVersion error when the owned status is deleted concurrently mid-reconcile", func() {
		// Isolation: the suite's manager-driven Layer2StatusReconciler runs as
		// NodeName=testNodeName and only advertises testServiceName, so it ignores
		// these churn-node/churn-svc objects. This spec drives its own reconciler
		// instance directly and must not share an announce-node/service with the
		// suite, or it would race the background reconciler non-deterministically.
		Expect(churnNode).ToNot(Equal(testNodeName))
		Expect(churnSvc).ToNot(Equal(testServiceName))

		inject := &churnInjectClient{
			Client:     k8sClient,
			serviceKey: types.NamespacedName{Namespace: churnNS, Name: churnSvc}.String(),
		}
		r := &Layer2StatusReconciler{
			Client:     inject,
			Logger:     log.NewNopLogger(),
			NodeName:   churnNode,
			Namespace:  speakerNamespace,
			SpeakerPod: speakerPod,
			StatusFetcher: func(types.NamespacedName) []layer2.IPAdvertisement {
				return []layer2.IPAdvertisement{
					layer2.NewIPAdvertisement(net.IP("127.0.0.9"), true, sets.Set[string]{}),
				}
			},
		}

		// A previous reconcile created this node's status; it carries a resourceVersion.
		existing := &v1beta1.ServiceL2Status{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "l2-",
				Namespace:    speakerNamespace,
				Labels: map[string]string{
					LabelAnnounceNode:     churnNode,
					LabelServiceName:      churnSvc,
					LabelServiceNamespace: churnNS,
				},
			},
		}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())
		Expect(existing.ResourceVersion).ToNot(BeEmpty())

		// The upcoming reconcile lists the owned status, then it is deleted
		// concurrently (as another speaker that briefly became leader would do).
		inject.deleteOnList = true

		// Before the fix, the reconciler selected the listed object (carrying its
		// resourceVersion) as the CreateOrPatch target; once it was gone, CreateOrPatch
		// took the create path with a stale resourceVersion and failed with
		// "resourceVersion should not be set on objects to be created", looping forever.
		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: churnNS, Name: churnSvc}}
		_, err := r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())

		// A second reconcile (no concurrent delete) fills in the status subresource,
		// the same way the production controller re-reconciles its own create event.
		_, err = r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())

		// The node re-converges to exactly one owned status with the node recorded.
		var list v1beta1.ServiceL2StatusList
		Expect(k8sClient.List(ctx, &list, client.MatchingLabels{
			LabelServiceName:      churnSvc,
			LabelServiceNamespace: churnNS,
			LabelAnnounceNode:     churnNode,
		})).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
		Expect(list.Items[0].Status.Node).To(Equal(churnNode))
	})
})
