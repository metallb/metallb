// SPDX-License-Identifier:Apache-2.0

package validate

import "sigs.k8s.io/controller-runtime/pkg/client"

type ClusterObjects interface {
	Validate(lists ...client.ObjectList) error
}
