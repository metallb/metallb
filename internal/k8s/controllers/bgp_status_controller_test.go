// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"

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
)

// churnInjectBGPClient is the ServiceBGPStatus counterpart of churnInjectClient:
// it emulates the manager field index the reconciler lists by (envtest's direct
// client has none, so we resolve by the service-identifying labels instead) and,
// when armed, deletes the listed statuses right after returning them to
// reproduce another speaker that briefly became leader and deleted this node's
// status between our List and the CreateOrPatch that follows (#3063).
type churnInjectBGPClient struct {
	client.Client
	serviceKey   string
	deleteOnList bool
}

func (c *churnInjectBGPClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	sl, ok := list.(*v1beta1.ServiceBGPStatusList)
	if !ok {
		return c.Client.List(ctx, list, opts...)
	}

	// opts are intentionally not forwarded: the production reconciler filters by
	// the manager field index (client.MatchingFields), which envtest's direct
	// client cannot satisfy, so we re-resolve the same set by label here.
	all := &v1beta1.ServiceBGPStatusList{}
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

var _ = Describe("ServiceBGPStatusReconciler reconcile-loop regression (#3063)", func() {
	const (
		churnNode = "churn-bgp-node"
		churnSvc  = "churn-bgp-svc"
		churnNS   = "churn-bgp-ns"
	)

	cleanup := func() {
		_ = k8sClient.DeleteAllOf(ctx, &v1beta1.ServiceBGPStatus{},
			client.InNamespace(speakerNamespace),
			client.MatchingLabels{LabelServiceName: churnSvc})
	}

	AfterEach(cleanup)

	It("recovers without a stale resourceVersion error when the owned status is deleted concurrently mid-reconcile", func() {
		// Isolation: the suite's manager-driven ServiceBGPStatusReconciler runs as
		// NodeName=testNodeName and only advertises testServiceName, so it ignores
		// these churn-node/churn-svc objects. This spec drives its own reconciler
		// instance directly and must not share an announce-node/service with the
		// suite, or it would race the background reconciler non-deterministically.
		Expect(churnNode).ToNot(Equal(testNodeName))
		Expect(churnSvc).ToNot(Equal(testServiceName))

		inject := &churnInjectBGPClient{
			Client:     k8sClient,
			serviceKey: types.NamespacedName{Namespace: churnNS, Name: churnSvc}.String(),
		}
		r := &ServiceBGPStatusReconciler{
			Client:     inject,
			Logger:     log.NewNopLogger(),
			NodeName:   churnNode,
			Namespace:  speakerNamespace,
			SpeakerPod: speakerPod,
			PeersFetcher: func(string) sets.Set[string] {
				return sets.New[string]("peer-a")
			},
		}

		// A previous reconcile created this node's status; it carries a resourceVersion.
		existing := &v1beta1.ServiceBGPStatus{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "bgp-",
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
		var list v1beta1.ServiceBGPStatusList
		Expect(k8sClient.List(ctx, &list, client.MatchingLabels{
			LabelServiceName:      churnSvc,
			LabelServiceNamespace: churnNS,
			LabelAnnounceNode:     churnNode,
		})).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
		Expect(list.Items[0].Status.Node).To(Equal(churnNode))
	})
})
