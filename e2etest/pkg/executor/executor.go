// SPDX-License-Identifier:Apache-2.0

package executor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

var Kubectl string

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
	if Kubectl == "" {
		return "", errors.New("the kubectl parameter is not set")
	}
	fullargs := append([]string{"exec", p.name, "-n", p.namespace, "-c", p.container, "--", cmd}, args...)

	c := exec.Command(Kubectl, fullargs...)
	var stdoutBuf, stderrBuf bytes.Buffer
	c.Stdout = &stdoutBuf
	c.Stderr = &stderrBuf
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("kubectl exec failed: %w, stderr: %s", err, stderrBuf.String())
	}
	return stdoutBuf.String(), nil
}

// add ephemeral container to deal with distroless image
type podDebugExecutor struct {
	namespace string
	name      string
	container string
	image     string
}

func ForPodDebug(namespace, name, container, image string) Executor {
	return &podDebugExecutor{
		namespace: namespace,
		name:      name,
		container: container,
		image:     image,
	}
}

func (pd *podDebugExecutor) Exec(cmd string, args ...string) (string, error) {
	if Kubectl == "" {
		return "", errors.New("the kubectl parameter is not set")
	}

	imageArg := "--image=" + pd.image
	targetArg := "--target=" + pd.container
	debuggerArg := pd.container + "-debugger-" + utilrand.String(5)

	fullargs := append([]string{"debug", "-it", "-n", pd.namespace, "-c", debuggerArg, targetArg, imageArg, pd.name, "--", cmd}, args...)
	out, err := exec.Command(Kubectl, fullargs...).CombinedOutput()
	return string(out), err
}
