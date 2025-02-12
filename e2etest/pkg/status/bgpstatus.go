// SPDX-License-Identifier:Apache-2.0

package status

import (
	"context"
	"fmt"

	"go.universe.tf/e2etest/pkg/metallb"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"go.universe.tf/metallb/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Returns the ServiceBGPStatus resource for the given service and node.
func BGPForServiceAndNode(cs client.Client, svc *v1.Service, node string) (*v1beta1.ServiceBGPStatus, error) {
	statusList := v1beta1.ServiceBGPStatusList{}
	err := cs.List(context.TODO(), &statusList,
		client.InNamespace(metallb.Namespace),
		client.MatchingLabels{
			LabelServiceName:      svc.Name,
			LabelServiceNamespace: svc.Namespace,
			LabelAnnounceNode:     node,
		},
	)
	svcKey := fmt.Sprintf("%s/%s", svc.Name, svc.Namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get status for service %s on node %s, err: %w", svcKey, node, err)
	}

	if len(statusList.Items) == 0 {
		return nil, errors.NewNotFound(schema.ParseGroupResource("ServiceBGPStatus.metallb.io"), svcKey)
	}
	if len(statusList.Items) > 1 {
		return nil, fmt.Errorf("got more than 1 ServiceBGPStatus object for service %s node %s: %v", svcKey, node, statusList.Items)
	}

	return &statusList.Items[0], nil
}
