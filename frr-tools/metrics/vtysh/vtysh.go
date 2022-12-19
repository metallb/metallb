// SPDX-License-Identifier:Apache-2.0

package vtysh

import (
	"os/exec"
)

type Cli func(args string) (string, error)

func Run(args string) (string, error) {
	out, err := exec.Command("/usr/bin/vtysh", "-c", args).CombinedOutput()
	return string(out), err
}

var _ Cli = Run
