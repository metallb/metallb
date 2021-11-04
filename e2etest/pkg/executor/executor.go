// SPDX-License-Identifier:Apache-2.0

package executor

import (
	"os"
	"os/exec"
)

type Executor interface {
	Exec(cmd string, args ...string) (string, error)
}

type hostExecutor struct{}

var (
	Host             hostExecutor
	ContainerRuntime = "docker"
)

func init() {
	if cr := os.Getenv("CONTAINER_RUNTIME"); len(cr) != 0 {
		ContainerRuntime = cr
	}
}

func (hostExecutor) Exec(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).CombinedOutput()
	return string(out), err
}

func ForContainer(containerName string) Executor {
	return &containerExecutor{container: containerName}
}

type containerExecutor struct {
	container string
}

func (e *containerExecutor) Exec(cmd string, args ...string) (string, error) {
	newArgs := append([]string{"exec", e.container, cmd}, args...)
	out, err := exec.Command(ContainerRuntime, newArgs...).CombinedOutput()
	return string(out), err
}
