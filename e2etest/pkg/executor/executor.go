// SPDX-License-Identifier:Apache-2.0

package executor

import (
	"os"
	"os/exec"

	"k8s.io/kubernetes/test/e2e/framework/kubectl"
)

type Executor interface {
	Exec(cmd string, args ...string) (string, error)
	Debug(cmd string, args ...string) (string, error)
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

func (h hostExecutor) Debug(cmd string, args ...string) (string, error) {
	return h.Exec(cmd, args...)
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

func (e *containerExecutor) Debug(cmd string, args ...string) (string, error) {
	return e.Exec(cmd, args...)
}

type podExecutor struct {
	namespace string
	name      string
	container string
}

func ForPod(namespace, name, container string) Executor {
	return &podExecutor{
		namespace: namespace,
		name:      name,
		container: container,
	}
}

func (p *podExecutor) Exec(cmd string, args ...string) (string, error) {
	fullArgs := append([]string{"exec", p.name, "-c", p.container, "--", cmd}, args...)
	res, err := kubectl.RunKubectl(p.namespace, fullArgs...)
	if err != nil {
		return "", err
	}
	return res, nil
}

func (p *podExecutor) Debug(cmd string, args ...string) (string, error) {
	fullArgs := append([]string{"debug", p.name, "--image=busybox", "--container=pod-executor-debugger", "--stdin", "--tty", "--", cmd}, args...)
	res, err := kubectl.RunKubectl(p.namespace, fullArgs...)
	if err != nil {
		return "", err
	}
	return res, nil
}
