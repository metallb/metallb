// SPDX-License-Identifier:Apache-2.0

package e2e

import (
	"fmt"
	"os/exec"

	"go.universe.tf/metallb/e2etest/pkg/executor"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	NetworkFailure = 4
	retryLimit     = 4
)

// DescribeSvc logs the output of kubectl describe svc for the given namespace.
func DescribeSvc(ns string) {
	framework.Logf("\nOutput of kubectl describe svc:\n")
	desc, _ := framework.RunKubectl(
		ns, "describe", "svc", fmt.Sprintf("--namespace=%v", ns))
	framework.Logf(desc)
}

func wgetRetry(address string, exc executor.Executor) error {
	retrycnt := 0
	code := 0
	var err error

	// Retry loop to handle wget NetworkFailure errors
	for {
		_, err = exc.Exec("wget", "-O-", "-q", address, "-T", "60")
		if exitErr, ok := err.(*exec.ExitError); err != nil && ok {
			code = exitErr.ExitCode()
		} else {
			break
		}
		if retrycnt < retryLimit && code == NetworkFailure {
			framework.Logf(" wget failed with code %d, err %s retrycnt %d\n", code, err, retrycnt)
			retrycnt++
		} else {
			break
		}
	}
	return err
}

func tweakServicePort(svc *v1.Service) {
	if servicePodPort != 80 {
		// if servicePodPort is non default, then change service spec.
		svc.Spec.Ports[0].TargetPort = intstr.FromInt(int(servicePodPort))
	}
}

func tweakRCPort(rc *v1.ReplicationController) {
	if servicePodPort != 80 {
		// if servicePodPort is non default, then change pod's spec
		rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", servicePodPort), fmt.Sprintf("--udp-port=%d", servicePodPort)}
		rc.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port = intstr.FromInt(int(servicePodPort))
	}
}
