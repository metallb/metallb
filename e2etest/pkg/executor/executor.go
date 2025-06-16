// SPDX-License-Identifier:Apache-2.0

package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
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
		return stderrBuf.String(), fmt.Errorf("kubectl exec failed: %w, stderr: %s", err, stderrBuf.String())
	}
	return stdoutBuf.String(), nil
}

// add ephemeral container to deal with distroless image
type podDebugExecutor struct {
	namespace string
	name      string
	container string
	image     string
	ephemeral string
	cs        clientset.Interface
}

func ForPodDebug(cs clientset.Interface, namespace, podName, targetContainer, image string) (Executor, error) {
	ret := &podDebugExecutor{
		namespace: namespace,
		name:      podName,
		container: targetContainer,
		ephemeral: "debugger-" + targetContainer,
		cs:        cs,
	}

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:  ret.ephemeral,
			Image: image,
		},
		TargetContainerName: targetContainer,
	}

	ctx := context.Background()
	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	containerExists := false
	for _, ec := range pod.Spec.EphemeralContainers {
		if ec.Name == ret.ephemeral {
			containerExists = true
			break
		}
	}

	if !containerExists {
		if err := addEphemeralContainerToPod(ctx, cs, pod, ephemeralContainer); err != nil {
			return nil, fmt.Errorf("failed to create ephemeral container: %w", err)
		}
	}

	err = wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 20*time.Second, false, func(context.Context) (bool, error) {
		updatedPod, err := cs.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, status := range updatedPod.Status.EphemeralContainerStatuses {
			if status.Name == ret.ephemeral {
				return status.State.Running != nil, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (pd *podDebugExecutor) Exec(cmd string, args ...string) (string, error) {
	if Kubectl == "" {
		return "", errors.New("the kubectl parameter is not set")
	}
	fullargs := append([]string{"exec", "-n", pd.namespace, "-c", pd.ephemeral, pd.name, "--", cmd}, args...)
	out, err := exec.Command(Kubectl, fullargs...).CombinedOutput()
	return string(out), err
}

func addEphemeralContainerToPod(ctx context.Context, cs clientset.Interface, pod *corev1.Pod, ephemeralContainer corev1.EphemeralContainer) error {
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ephemeralContainer)
	_, err := cs.CoreV1().Pods(pod.Namespace).UpdateEphemeralContainers(ctx, pod.Name, pod, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
