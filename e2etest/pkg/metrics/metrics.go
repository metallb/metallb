// SPDX-License-Identifier:Apache-2.0

package metrics

import (
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"

	"github.com/pkg/errors"
	"k8s.io/kubernetes/test/e2e/framework"
)

// MetricsForPod returns the parsed metrics for the given pod, scraping them
// from the executor pod.
func ForPod(executor, target *corev1.Pod, namespace string) ([]map[string]*dto.MetricFamily, error) {
	ports := make([]int, 0)
	allMetrics := make([]map[string]*dto.MetricFamily, 0)
	for _, c := range target.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == "monitoring" {
				ports = append(ports, int(p.ContainerPort))
			}
		}
	}

	for _, p := range ports {
		metricsUrl := path.Join(net.JoinHostPort(target.Status.PodIP, strconv.Itoa(p)), "metrics")
		metrics, err := framework.RunKubectl(namespace, "exec", executor.Name, "--", "wget", "-qO-", metricsUrl)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to scrape metrics for %s", target.Name)
		}
		res, err := metricsFromString(metrics)
		if err != nil {
			return nil, err
		}
		allMetrics = append(allMetrics, res)
	}

	return allMetrics, nil
}

// GaugeForLabels retrieves the value of the Gauge matching the given set of labels.
func GaugeForLabels(metricName string, labels map[string]string, metrics map[string]*dto.MetricFamily) (int, error) {
	return metricForLabels(metricName, labels, metrics, func(m *dto.Metric) int {
		return int(m.GetGauge().GetValue())
	})
}

// CounterForLabels retrieves the value of the Counter matching the given set of labels.
func CounterForLabels(metricName string, labels map[string]string, metrics map[string]*dto.MetricFamily) (int, error) {
	return metricForLabels(metricName, labels, metrics, func(m *dto.Metric) int {
		return int(m.GetCounter().GetValue())
	})
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
		return nil, errors.Wrapf(err, "failed to parse metrics %s", metrics)
	}
	return mf, nil
}
