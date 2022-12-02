// SPDX-License-Identifier:Apache-2.0

package collector

import (
	"os/exec"
)

func runVtysh(args string) (string, error) {
	out, err := exec.Command("/usr/bin/vtysh", "-c", args).CombinedOutput()
	return string(out), err
}
