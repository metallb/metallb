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

func L2ForService(cs client.Client, svc *v1.Service) (*v1beta1.ServiceL2Status, error) {
	statusList := v1beta1.ServiceL2StatusList{}
	err := cs.List(context.TODO(), &statusList,
		client.InNamespace(metallb.Namespace),
		client.MatchingLabels{
			LabelServiceName:      svc.Name,
			LabelServiceNamespace: svc.Namespace,
		},
	)
	if err != nil {
		return nil, err
	}
	statuses := statusList.Items
	if len(statuses) == 0 {
		return nil, errors.NewNotFound(schema.ParseGroupResource("serviceL2Status.metallb.io"), fmt.Sprintf("%s/%s", svc.Name, svc.Namespace))
	}
	if len(statuses) > 1 {
		return nil, fmt.Errorf("got more than 1 serviceL2Status object: %d", len(statuses))
	}
	return &(statuses[0]), nil
}
