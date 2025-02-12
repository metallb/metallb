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

// MetalLBServiceBGPStatus defines the observed state of ServiceBGPStatus.
type MetalLBServiceBGPStatus struct {
	// Node indicates the node announcing the service.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Node string `json:"node,omitempty"`

	// ServiceName indicates the service this status represents.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	ServiceName string `json:"serviceName,omitempty"`

	// ServiceNamespace indicates the namespace of the service.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	ServiceNamespace string `json:"serviceNamespace,omitempty"`

	// Peers indicate the BGP peers for which the service is configured to be advertised to.
	// The service being actually advertised to a given peer depends on the session state and is not indicated here.
	Peers []string `json:"peers,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.status.node`
// +kubebuilder:printcolumn:name="Service Name",type=string,JSONPath=`.status.serviceName`
// +kubebuilder:printcolumn:name="Service Namespace",type=string,JSONPath=`.status.serviceNamespace`
// ServiceBGPStatus exposes the BGP peers a service is configured to be advertised to, per relevant node.
type ServiceBGPStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceBGPStatusSpec    `json:"spec,omitempty"`
	Status MetalLBServiceBGPStatus `json:"status,omitempty"`
}

// ServiceBGPStatusSpec defines the desired state of ServiceBGPStatus.
type ServiceBGPStatusSpec struct {
}

// +kubebuilder:object:root=true

// ServiceBGPStatusList contains a list of ServiceBGPStatus.
type ServiceBGPStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceBGPStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceBGPStatus{}, &ServiceBGPStatusList{})
}
