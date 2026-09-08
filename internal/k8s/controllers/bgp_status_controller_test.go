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

// deleteOnListBGPClient is the ServiceBGPStatus counterpart of deleteOnListClient.
type deleteOnListBGPClient struct {
	client.Client
	serviceKey   string
	deleteOnList bool
}

func (c *deleteOnListBGPClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	sl, ok := list.(*v1beta1.ServiceBGPStatusList)
	if !ok {
		return c.Client.List(ctx, list, opts...)
	}

	// opts reference a field index that does not exist here, so re-resolve by label.
	all := &v1beta1.ServiceBGPStatusList{}
	if err := c.Client.List(ctx, all); err != nil {
		return err
	}
	for i := range all.Items {
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

var _ = Describe("ServiceBGPStatusReconciler stale resourceVersion", func() {
	const (
		raceNode = "race-bgp-node"
		raceSvc  = "race-bgp-svc"
		raceNS   = "race-bgp-ns"
	)

	cleanup := func() {
		_ = k8sClient.DeleteAllOf(ctx, &v1beta1.ServiceBGPStatus{},
			client.InNamespace(speakerNamespace),
			client.MatchingLabels{LabelServiceName: raceSvc})
	}

	AfterEach(cleanup)

	It("recovers without a stale resourceVersion error when the owned status is deleted concurrently mid-reconcile", func() {
		inject := &deleteOnListBGPClient{
			Client:     k8sClient,
			serviceKey: types.NamespacedName{Namespace: raceNS, Name: raceSvc}.String(),
		}
		r := &ServiceBGPStatusReconciler{
			Client:     inject,
			Logger:     log.NewNopLogger(),
			NodeName:   raceNode,
			Namespace:  speakerNamespace,
			SpeakerPod: speakerPod,
			PeersFetcher: func(string) sets.Set[string] {
				return sets.New[string]("peer-a")
			},
		}

		existing := &v1beta1.ServiceBGPStatus{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "bgp-",
				Namespace:    speakerNamespace,
				Labels: map[string]string{
					LabelAnnounceNode:     raceNode,
					LabelServiceName:      raceSvc,
					LabelServiceNamespace: raceNS,
				},
			},
		}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())
		Expect(existing.ResourceVersion).ToNot(BeEmpty())

		inject.deleteOnList = true

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: raceNS, Name: raceSvc}}
		_, err := r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())

		// A second reconcile (no concurrent delete) fills in the status subresource.
		_, err = r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())

		var list v1beta1.ServiceBGPStatusList
		Expect(k8sClient.List(ctx, &list, client.MatchingLabels{
			LabelServiceName:      raceSvc,
			LabelServiceNamespace: raceNS,
			LabelAnnounceNode:     raceNode,
		})).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
		Expect(list.Items[0].Status.Node).To(Equal(raceNode))
	})
})
