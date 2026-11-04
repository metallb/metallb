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

// deleteOnListClient emulates the manager field index the reconciler lists by and,
// when armed, deletes the listed statuses right after returning them.
type deleteOnListClient struct {
	client.Client
	serviceKey   string
	deleteOnList bool
}

func (c *deleteOnListClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	sl, ok := list.(*v1beta1.ServiceL2StatusList)
	if !ok {
		return c.Client.List(ctx, list, opts...)
	}

	// opts reference a field index that does not exist here, so re-resolve by label.
	all := &v1beta1.ServiceL2StatusList{}
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

var _ = Describe("Layer2StatusReconciler stale resourceVersion", func() {
	const (
		raceNode = "race-node"
		raceSvc  = "race-svc"
		raceNS   = "race-ns"
	)

	cleanup := func() {
		_ = k8sClient.DeleteAllOf(ctx, &v1beta1.ServiceL2Status{},
			client.InNamespace(speakerNamespace),
			client.MatchingLabels{LabelServiceName: raceSvc})
	}

	AfterEach(cleanup)

	It("recovers without a stale resourceVersion error when the owned status is deleted concurrently mid-reconcile", func() {
		inject := &deleteOnListClient{
			Client:     k8sClient,
			serviceKey: types.NamespacedName{Namespace: raceNS, Name: raceSvc}.String(),
		}
		r := &Layer2StatusReconciler{
			Client:     inject,
			Logger:     log.NewNopLogger(),
			NodeName:   raceNode,
			Namespace:  speakerNamespace,
			SpeakerPod: speakerPod,
			StatusFetcher: func(types.NamespacedName) []layer2.IPAdvertisement {
				return []layer2.IPAdvertisement{
					layer2.NewIPAdvertisement(net.IP("127.0.0.9"), true, sets.Set[string]{}),
				}
			},
		}

		existing := &v1beta1.ServiceL2Status{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "l2-",
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

		var list v1beta1.ServiceL2StatusList
		Expect(k8sClient.List(ctx, &list, client.MatchingLabels{
			LabelServiceName:      raceSvc,
			LabelServiceNamespace: raceNS,
			LabelAnnounceNode:     raceNode,
		})).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
		Expect(list.Items[0].Status.Node).To(Equal(raceNode))
	})
})
