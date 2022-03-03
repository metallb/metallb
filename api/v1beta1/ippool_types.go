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

// IPPoolSpec defines the desired state of IPPool.
type IPPoolSpec struct {
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

// IPPoolStatus defines the observed state of IPPool.
type IPPoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// IPPool is the Schema for the IPPools API.
type IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolSpec   `json:"spec"`
	Status IPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPPoolList contains a list of IPPool.
type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPPool{}, &IPPoolList{})
}
