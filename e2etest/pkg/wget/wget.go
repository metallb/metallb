// SPDX-License-Identifier:Apache-2.0

package wget

import (
	"os/exec"

	"go.universe.tf/metallb/e2etest/pkg/executor"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	NetworkFailure = 4
	retryLimit     = 4
)

func Do(address string, exc executor.Executor) error {
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
