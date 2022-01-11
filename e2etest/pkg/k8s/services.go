// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"fmt"

	"k8s.io/kubernetes/test/e2e/framework"
)

// DescribeSvc logs the output of kubectl describe svc for the given namespace.
func DescribeSvc(ns string) {
	framework.Logf("\nOutput of kubectl describe svc:\n")
	desc, _ := framework.RunKubectl(
		ns, "describe", "svc", fmt.Sprintf("--namespace=%v", ns))
	framework.Logf(desc)
}
