/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true

// MetalLBServiceL2Status defines the observed state of ServiceL2Status.
type MetalLBServiceL2Status struct {
	// Node indicates the node that receives the directed traffic
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Node string `json:"node,omitempty"`
	// ServiceName indicates the service this status represents
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	ServiceName string `json:"serviceName,omitempty"`
	// ServiceNamespace indicates the namespace of the service
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	ServiceNamespace string `json:"serviceNamespace,omitempty"`
	// Interfaces indicates the interfaces that receive the directed traffic
	Interfaces []InterfaceInfo `json:"interfaces,omitempty"`
}

// InterfaceInfo defines interface info of layer2 announcement.
type InterfaceInfo struct {
	// Name the name of network interface card
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Allocated Node",type=string,JSONPath=`.status.node`
// +kubebuilder:printcolumn:name="Service Name",type=string,JSONPath=`.status.serviceName`
// +kubebuilder:printcolumn:name="Service Namespace",type=string,JSONPath=`.status.serviceNamespace`

// ServiceL2Status reveals the actual traffic status of loadbalancer services in layer2 mode.
type ServiceL2Status struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceL2StatusSpec    `json:"spec,omitempty"`
	Status MetalLBServiceL2Status `json:"status,omitempty"`
}

// ServiceL2StatusSpec defines the desired state of ServiceL2Status.
type ServiceL2StatusSpec struct {
}

// +kubebuilder:object:root=true

// ServiceL2StatusList contains a list of ServiceL2Status.
type ServiceL2StatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceL2Status `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceL2Status{}, &ServiceL2StatusList{})
}
