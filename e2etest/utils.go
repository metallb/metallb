package e2e

import (
	"fmt"
	"os/exec"

	"k8s.io/kubernetes/test/e2e/framework"
)

// DescribeSvc logs the output of kubectl describe svc for the given namespace
func DescribeSvc(ns string) {
	framework.Logf("\nOutput of kubectl describe svc:\n")
	desc, _ := framework.RunKubectl(
		ns, "describe", "svc", fmt.Sprintf("--namespace=%v", ns))
	framework.Logf(desc)
}

func runCommand(bgp bool, name string, args ...string) (string, error) {
	if bgp {
		// prepend "docker exec frr"
		cmd := []string{"exec", "frr", name}
		name = "docker"
		args = append(cmd, args...)
	}
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}
