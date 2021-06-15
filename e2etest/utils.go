package e2e

import (
	"fmt"
	"os/exec"

	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	NetworkFailure = 4
	retryLimit     = 4
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

func wgetRetry(bgp bool, address string) error {
	retrycnt := 0
	code := 0
	var err error

	// Retry loop to handle wget NetworkFailure errors
	for {
		_, err = runCommand(bgp, "wget", "-O-", "-q", address, "-T", "60")
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
