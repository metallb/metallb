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

	// AllocateTo makes ip pool allocation to specific namespace and/or service.
	// The controller will use the pool with lowest value of priority in case of
	// multiple matches. A pool with no priority set will be used only if the
	// pools with priority can't be used. If multiple matching IPAddressPools are
	// available it will check for the availability of IPs sorting the matching
	// IPAddressPools by priority, starting from the highest to the lowest. If
	// multiple IPAddressPools have the same priority, choice will be random.
	// +optional
	AllocateTo *ServiceAllocation `json:"serviceAllocation,omitempty"`
}

// ServiceAllocation defines ip pool allocation to namespace and/or service.
type ServiceAllocation struct {
	// Priority priority given for ip pool while ip allocation on a service.
	Priority int `json:"priority,omitempty"`
	// Namespaces list of namespace(s) on which ip pool can be attached.
	Namespaces []string `json:"namespaces,omitempty"`
	// NamespaceSelectors list of label selectors to select namespace(s) for ip pool,
	// an alternative to using namespace list.
	NamespaceSelectors []metav1.LabelSelector `json:"namespaceSelectors,omitempty"`
	// ServiceSelectors list of label selector to select service(s) for which ip pool
	// can be used for ip allocation.
	ServiceSelectors []metav1.LabelSelector `json:"serviceSelectors,omitempty"`
}

// IPAddressPoolStatus defines the observed state of IPAddressPool.
type IPAddressPoolStatus struct {
	// AssignedIPv4 is the number of assigned IPv4 addresses.
	AssignedIPv4 int64 `json:"assignedIPv4"`

	// AssignedIPv6 is the number of assigned IPv6 addresses.
	AssignedIPv6 int64 `json:"assignedIPv6"`

	// AvailableIPv4 is the number of available IPv4 addresses.
	AvailableIPv4 int64 `json:"availableIPv4"`

	// AvailableIPv6 is the number of available IPv6 addresses.
	AvailableIPv6 int64 `json:"availableIPv6"`
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
