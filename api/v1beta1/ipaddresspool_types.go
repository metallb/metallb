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

// IPAddressPoolSpec defines the desired state of IPAddressPool.
type IPAddressPoolSpec struct {
	// A list of IP address ranges over which MetalLB has authority.
	// You can list multiple ranges in a single pool, they will all share the
	// same settings. Each range can be either a CIDR prefix, or an explicit
	// start-end range of IPs.
	Addresses []string `json:"addresses"`

	// AutoAssign flag used to prevent MetallB from automatic allocation
	// for a pool.
	// +optional
	// +kubebuilder:default:=true
	AutoAssign *bool `json:"autoAssign,omitempty"`

	// AvoidBuggyIPs prevents addresses ending with .0 and .255
	// to be used by a pool.
	// +optional
	// +kubebuilder:default:=false
	AvoidBuggyIPs bool `json:"avoidBuggyIPs,omitempty"`
}

// IPAddressPoolStatus defines the observed state of IPAddressPool.
type IPAddressPoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Auto Assign",type=boolean,JSONPath=`.spec.autoAssign`
// +kubebuilder:printcolumn:name="Avoid Buggy IPs",type=boolean,JSONPath=`.spec.avoidBuggyIPs`
// +kubebuilder:printcolumn:name="Addresses",type=string,JSONPath=`.spec.addresses`

// IPAddressPool represents a pool of IP addresses that can be allocated
// to LoadBalancer services.
type IPAddressPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPAddressPoolSpec   `json:"spec"`
	Status IPAddressPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPAddressPoolList contains a list of IPAddressPool.
type IPAddressPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPAddressPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPAddressPool{}, &IPAddressPoolList{})
}
