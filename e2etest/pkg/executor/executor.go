package executor

import "os/exec"

type Executor interface {
	Exec(cmd string, args ...string) (string, error)
}

type hostExecutor struct{}

var Host hostExecutor

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
	out, err := exec.Command("docker", newArgs...).CombinedOutput()
	return string(out), err
}
