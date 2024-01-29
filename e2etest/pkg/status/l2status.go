// SPDX-License-Identifier:Apache-2.0

package status

import (
	"context"
	"fmt"

	"go.universe.tf/metallb/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetL2Status(cs client.Client, svc *v1.Service, nodeName string) (*v1beta1.ServiceL2Status, error) {
	s := &v1beta1.ServiceL2Status{}
	if err := cs.Get(context.TODO(), types.NamespacedName{
		Namespace: svc.Namespace,
		Name:      fmt.Sprintf("%s-%s", svc.Name, nodeName),
	}, s); err != nil {
		return nil, err
	}
	return s, nil
}

func GetSvcPossibleL2Status(cs client.Client, svc *v1.Service, nodes *v1.NodeList) ([]*v1beta1.ServiceL2Status, error) {
	var l2Statuses []*v1beta1.ServiceL2Status
	for _, node := range nodes.Items {
		s, err := GetL2Status(cs, svc, node.Name)
		if err != nil && errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		l2Statuses = append(l2Statuses, s)
	}
	return l2Statuses, nil
}
