// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AreaType is the OSPF area type. The type determines which LSA categories are
// permitted in the area and whether external route redistribution is allowed.
// +kubebuilder:validation:Enum=regular;stub;nssa;totally-stub
type AreaType string

const (
	// AreaTypeRegular is a normal OSPF area. All LSA types are allowed and
	// external routes (Type 5 LSAs) may be redistributed into it.
	AreaTypeRegular AreaType = "regular"
	// AreaTypeStub blocks Type 5 external LSAs. Service IPs cannot be
	// redistributed from a node whose only area is stub.
	AreaTypeStub AreaType = "stub"
	// AreaTypeNSSA allows external routes as Type 7 LSAs which are converted
	// to Type 5 at the ABR. Service IPs can be redistributed.
	AreaTypeNSSA AreaType = "nssa"
	// AreaTypeTotallyStub blocks both Type 3 summary and Type 5 external LSAs.
	// Service IPs cannot be redistributed from a node in a totally-stub area.
	AreaTypeTotallyStub AreaType = "totally-stub"
)

// OSPFArea declares an OSPF area and its type for use in an OSPFInstance.
type OSPFArea struct {
	// ID is the OSPF area identifier in dotted-quad notation (e.g. "0.0.0.0").
	// Area 0.0.0.0 is the backbone and its type is always regular.
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}$`
	ID string `json:"id"`

	// Type is the OSPF area type. Defaults to regular.
	// External route redistribution (service IPs) is only possible in regular
	// and nssa areas; stub and totally-stub areas do not permit it.
	// +kubebuilder:default=regular
	// +optional
	Type AreaType `json:"type,omitempty"`
}

// OSPFInterfaceConfig describes how a single network interface participates in OSPF.
type OSPFInterfaceConfig struct {
	// Name is the network interface name on the node (e.g. "eth0", "bond0").
	// No API validation is performed; an invalid name simply prevents the
	// OSPF neighborship from forming on that interface.
	Name string `json:"name"`

	// AreaID is the OSPF area this interface belongs to, in dotted-quad notation.
	// Must reference an area ID declared in spec.areas.
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}$`
	AreaID string `json:"areaID"`

	// Passive prevents OSPF hello packets from being sent on this interface
	// while still advertising its network prefix. Recommended for loopbacks
	// and host-facing interfaces.
	// +optional
	Passive bool `json:"passive,omitempty"`

	// HelloInterval is the interval between OSPF Hello packets on this interface.
	// Both ends of a link must use the same value.
	// +optional
	HelloInterval *metav1.Duration `json:"helloInterval,omitempty"`

	// DeadInterval is the time a neighbor must be silent before it is declared
	// down. Must be greater than HelloInterval; conventionally 4× HelloInterval.
	// Both ends of a link must use the same value.
	// +optional
	DeadInterval *metav1.Duration `json:"deadInterval,omitempty"`

	// Cost overrides the OSPF metric for this interface. When omitted FRR
	// derives the cost from the interface bandwidth.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Cost *uint32 `json:"cost,omitempty"`
}

// OSPFInstanceSpec defines the desired OSPF configuration for nodes matching
// the node selectors.
type OSPFInstanceSpec struct {
	// RouterID is the OSPF router ID in dotted-quad notation. When omitted
	// FRR selects the highest IPv4 address on the node.
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}$`
	// +optional
	RouterID string `json:"routerID,omitempty"`

	// Areas declares all OSPF areas used by this instance. At least one area
	// must be listed; every area referenced in spec.interfaces must appear here.
	// Area 0.0.0.0 (backbone) must be present when the node has interfaces in
	// multiple areas or acts as an ABR.
	// +kubebuilder:validation:MinItems=1
	Areas []OSPFArea `json:"areas"`

	// Interfaces lists the node interfaces that participate in OSPF.
	// Each entry assigns the interface to a specific area.
	// +kubebuilder:validation:MinItems=1
	Interfaces []OSPFInterfaceConfig `json:"interfaces"`

	// NodeSelectors limits which nodes this OSPFInstance is applied to.
	// When empty the instance is applied to all nodes.
	// +optional
	NodeSelectors []metav1.LabelSelector `json:"nodeSelectors,omitempty"`

	// VRF is the name of the VRF the OSPF process runs in.
	// +optional
	VRF string `json:"vrf,omitempty"`
}

// OSPFInstanceStatus defines the observed state of OSPFInstance.
type OSPFInstanceStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Router ID",type=string,JSONPath=`.spec.routerID`
//+kubebuilder:printcolumn:name="VRF",type=string,JSONPath=`.spec.vrf`

// OSPFInstance configures an OSPF routing process on nodes matching the node
// selectors. Service IPs from matching OSPFAdvertisements are redistributed
// into OSPF as external routes, making them reachable across the OSPF domain.
type OSPFInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OSPFInstanceSpec   `json:"spec,omitempty"`
	Status OSPFInstanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OSPFInstanceList contains a list of OSPFInstance.
type OSPFInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OSPFInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OSPFInstance{}, &OSPFInstanceList{})
}
