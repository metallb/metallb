// SPDX-License-Identifier:Apache-2.0

package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"go.universe.tf/e2etest/pkg/executor"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// ValidateOnPrometheus checks the existence of the given metric directly on prometheus pods.
func ValidateOnPrometheus(prometheusPod *corev1.Pod, query string, expected CheckType) error {
	exec := executor.ForPod(prometheusPod.Namespace, prometheusPod.Name, prometheusPod.Spec.Containers[0].Name)
	url := fmt.Sprintf("localhost:9090/api/v1/query?%s", (url.Values{"query": []string{query}}).Encode())
	stdout, err := exec.Exec("wget", "-qO-", url)
	if err != nil {
		return err
	}
	// check query result, if this is a new error log it, otherwise remain silent
	var result PrometheusResponse
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return fmt.Errorf("unable to parse query response for %s: %v", query, err)
	}
	metrics := result.Data.Result
	if result.Status != "success" {
		data, _ := json.MarshalIndent(metrics, "", "  ")
		return fmt.Errorf("promQL query: %s had reported incorrect status:\n%s", query, data)
	}

	if expected == NotThere && len(metrics) > 0 {
		data, _ := json.MarshalIndent(result.Data.Result, "", "  ")
		return fmt.Errorf("promQL query returned unexpected results:\n%s\n%s", query, data)
	}
	if expected == There && len(metrics) == 0 {
		data, _ := json.MarshalIndent(result.Data.Result, "", "  ")
		return fmt.Errorf("promQL query returned unexpected results:\n%s\n%s", query, data)
	}

	return nil
}

// PrometheusPod returns a prometheus pod.
func PrometheusPod(cs clientset.Interface, namespace string) (*corev1.Pod, error) {
	promPods, err := cs.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=prometheus",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list prometheus pods: %v", err)
	}
	if len(promPods.Items) == 0 {
		return nil, fmt.Errorf("no prometheus pods found")
	}

	return promPods.Items[0].DeepCopy(), nil
}
