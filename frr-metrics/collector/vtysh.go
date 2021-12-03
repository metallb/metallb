// SPDX-License-Identifier:Apache-2.0

package collector

import (
	"os/exec"
)

func runVtysh(args ...string) (string, error) {
	newArgs := append([]string{"-c"}, args...)
	out, err := exec.Command("/usr/bin/vtysh", newArgs...).CombinedOutput()
	return string(out), err
}
