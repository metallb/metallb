// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OSPFMetricType is the OSPF external route metric type.
// Type 2 costs are not added to the intra-area path cost (default, recommended
// for external redistributed routes). Type 1 costs are comparable to internal
// OSPF link costs and are added during best-path selection.
// +kubebuilder:validation:Enum=1;2
type OSPFMetricType uint32

const (
	OSPFMetricType1 OSPFMetricType = 1
	OSPFMetricType2 OSPFMetricType = 2
)

// OSPFAdvertisementSpec defines which IP address pools are redistributed into
// OSPF and with what metric parameters.
type OSPFAdvertisementSpec struct {
	// IPAddressPools selects IP address pools by name whose allocated IPs will
	// be redistributed into OSPF. When both IPAddressPools and
	// IPAddressPoolSelectors are empty, all pools are selected.
	// +optional
	IPAddressPools []string `json:"ipAddressPools,omitempty"`

	// IPAddressPoolSelectors selects IP address pools by label.
	// +optional
	IPAddressPoolSelectors []metav1.LabelSelector `json:"ipAddressPoolSelectors,omitempty"`

	// NodeSelectors limits the nodes from which IPs are redistributed into OSPF.
	// When empty all nodes with a matching OSPFInstance perform redistribution.
	// +optional
	NodeSelectors []metav1.LabelSelector `json:"nodeSelectors,omitempty"`

	// ServiceSelectors limits redistribution to services matching at least one
	// selector. When empty all services in selected pools are redistributed.
	// +optional
	ServiceSelectors []metav1.LabelSelector `json:"serviceSelectors,omitempty"`

	// Metric sets the OSPF external metric for redistributed prefixes.
	// When 0 FRR uses its default metric.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=16777214
	// +optional
	Metric uint32 `json:"metric,omitempty"`

	// MetricType sets the OSPF external metric type for redistributed prefixes.
	// Defaults to 2.
	// +kubebuilder:default=2
	// +optional
	MetricType OSPFMetricType `json:"metricType,omitempty"`
}

// OSPFAdvertisementStatus defines the observed state of OSPFAdvertisement.
type OSPFAdvertisementStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="IPAddressPools",type=string,JSONPath=`.spec.ipAddressPools`
//+kubebuilder:printcolumn:name="Node Selectors",type=string,JSONPath=`.spec.nodeSelectors`,priority=10
//+kubebuilder:printcolumn:name="Metric",type=integer,JSONPath=`.spec.metric`
//+kubebuilder:printcolumn:name="Metric Type",type=integer,JSONPath=`.spec.metricType`

// OSPFAdvertisement controls redistribution of LoadBalancer IPs from selected
// IP address pools into OSPF as external routes.
type OSPFAdvertisement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OSPFAdvertisementSpec   `json:"spec,omitempty"`
	Status OSPFAdvertisementStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OSPFAdvertisementList contains a list of OSPFAdvertisement.
type OSPFAdvertisementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OSPFAdvertisement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OSPFAdvertisement{}, &OSPFAdvertisementList{})
}
