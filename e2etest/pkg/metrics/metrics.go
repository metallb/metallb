// SPDX-License-Identifier:Apache-2.0

package metrics

import (
	"errors"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"

	"github.com/onsi/ginkgo/v2"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"go.universe.tf/e2etest/pkg/executor"

	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

const AgnhostImage = "registry.k8s.io/e2e-test-images/agnhost:2.43"

type PrometheusResponse struct {
	Status string                 `json:"status"`
	Data   prometheusResponseData `json:"data"`
}

type prometheusResponseData struct {
	ResultType string       `json:"resultType"`
	Result     model.Vector `json:"result"`
}

// ForPod returns the parsed metrics for the given target pod, scraping them
// via an ephemeral debug container with curl (for TLS 1.3 compatibility).
// The SA token is read from the prometheus container, and curl runs from
// the debug container.
func ForPod(cs clientset.Interface, promPod, target *corev1.Pod, namespace string) ([]map[string]*dto.MetricFamily, error) {
	ports := make([]int, 0)
	allMetrics := make([]map[string]*dto.MetricFamily, 0)
	for _, c := range target.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == "metricshttps" || p.Name == "frrmetricshttps" {
				ports = append(ports, int(p.ContainerPort))
			}
		}
	}

	tokenExec := executor.ForPod(promPod.Namespace, promPod.Name, "prometheus")
	token, err := tokenExec.Exec("cat", "/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, fmt.Errorf("failed to read SA token: %w", err)
	}

	curlExec, err := executor.ForPodDebug(cs, promPod.Namespace, promPod.Name, "prometheus", AgnhostImage)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug container: %w", err)
	}

	ginkgo.GinkgoWriter.Printf("[ForPod] target=%s podIP=%s ports=%v\n", target.Name, target.Status.PodIP, ports)

	for _, p := range ports {
		metricsPath := path.Join(net.JoinHostPort(target.Status.PodIP, strconv.Itoa(p)), "metrics")
		metricsURL := fmt.Sprintf("https://%s", metricsPath)
		metrics, err := curlExec.Exec("curl", "-s", "-k",
			"-H", fmt.Sprintf("Authorization: Bearer %s", token),
			metricsURL)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("[ForPod] curl error port %d: %v\n", p, err)
			return nil, errors.Join(err, fmt.Errorf("failed to scrape metrics for %s", target.Name))
		}
		ginkgo.GinkgoWriter.Printf("[ForPod] port %d: response length=%d, first 300 chars: %.300s\n", p, len(metrics), metrics)
		res, err := metricsFromString(metrics)
		if err != nil {
			return nil, err
		}
		ginkgo.GinkgoWriter.Printf("[ForPod] port %d: parsed %d metric families\n", p, len(res))
		allMetrics = append(allMetrics, res)
	}

	return allMetrics, nil
}

type CheckType bool

const (
	NotThere CheckType = false
	There    CheckType = true
)

// GaugeForLabels retrieves the value of the Gauge matching the given set of labels.
func GaugeForLabels(metricName string, labels map[string]string, metrics map[string]*dto.MetricFamily) (int, error) {
	return metricForLabels(metricName, labels, metrics, func(m *dto.Metric) int {
		return int(m.GetGauge().GetValue())
	})
}

// ValidateGaugeValue checks that the value corresponding to the given metric is the same as expected value.
func ValidateGaugeValue(expectedValue int, metricName string, labels map[string]string, allMetrics []map[string]*dto.MetricFamily) error {
	return ValidateGaugeValueCompare(func(value int) error {
		if value != expectedValue {
			return fmt.Errorf("expecting %d, got %d", expectedValue, value)
		}
		return nil
	}, metricName, labels, allMetrics)
}

// ValidateGaugeValueCompare checks that the value corresponding to the given metric against the given compare function.
func ValidateGaugeValueCompare(check func(int) error, metricName string, labels map[string]string, allMetrics []map[string]*dto.MetricFamily) error {
	found := false
	for _, m := range allMetrics {
		value, err := GaugeForLabels(metricName, labels, m)
		if err != nil {
			continue
		}
		err = check(value)
		if err != nil {
			return fmt.Errorf("invalid value %d for %s, %w", value, metricName, err)
		}
		found = true
	}

	if !found {
		return fmt.Errorf("metric %s not found", metricName)
	}
	return nil
}

// ValidateCounterValue checks that the value related to the given metric is at most the expectedMax value.
func ValidateCounterValue(check func(int) error, metricName string, labels map[string]string, allMetrics []map[string]*dto.MetricFamily) error {
	var err error
	var value int
	found := false
	for _, m := range allMetrics {
		value, err = CounterForLabels(metricName, labels, m)
		if err != nil {
			continue
		}
		found = true
		err := check(value)
		if err != nil {
			return fmt.Errorf("invalid value %d for %s, %w", value, metricName, err)
		}
	}

	if !found {
		return fmt.Errorf("metric %s not found", metricName)
	}
	return nil
}

// CounterForLabels retrieves the value of the Counter matching the given set of labels.
func CounterForLabels(metricName string, labels map[string]string, metrics map[string]*dto.MetricFamily) (int, error) {
	return metricForLabels(metricName, labels, metrics, func(m *dto.Metric) int {
		return int(m.GetCounter().GetValue())
	})
}

func GreaterOrEqualThan(min int) func(value int) error {
	return func(value int) error {
		if value < min {
			return fmt.Errorf("value %d is less than %d", value, min)
		}
		return nil
	}
}

func metricForLabels(metricName string, labels map[string]string, metrics map[string]*dto.MetricFamily, getValue func(m *dto.Metric) int) (int, error) {
	mf, ok := metrics[metricName]
	if !ok {
		return 0, fmt.Errorf("metric %s not in metrics", metricName)
	}
	mm := mf.GetMetric()
	for _, m := range mm {
		toMatch := len(labels)
		label := m.GetLabel()
		for _, l := range label {
			v, ok := labels[l.GetName()]
			if !ok {
				continue
			}
			if v != l.GetValue() {
				continue
			}
			toMatch--
		}
		if toMatch == 0 {
			return getValue(m), nil
		}
	}
	return 0, fmt.Errorf("label %s not found in metrics for %s", labels, metricName)
}

func metricsFromString(metrics string) (map[string]*dto.MetricFamily, error) {
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(strings.NewReader(metrics))
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("failed to parse metrics %s", metrics))
	}
	return mf, nil
}
