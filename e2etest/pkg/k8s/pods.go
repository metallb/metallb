// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"bytes"
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

func PodLogsSinceTime(cs clientset.Interface, pod *corev1.Pod, speakerContainerName string, sinceTime *metav1.Time) (string, error) {
	podLogOpt := corev1.PodLogOptions{
		Container: speakerContainerName,
		SinceTime: sinceTime,
	}
	return PodLogs(cs, pod, podLogOpt)
}

func PodLogs(cs clientset.Interface, pod *corev1.Pod, podLogOpts corev1.PodLogOptions) (string, error) {
	req := cs.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}

	str := buf.String()
	return str, nil
}
