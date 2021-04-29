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

const (
	Layer2 = "layer2"
	BGP    = "bgp"
)

type BgpAdvertisement struct {
	// The aggregation-length advertisement option lets you “roll up” the /32s into a larger prefix.
	// +kubebuilder:validation:Minimum=1
	AggregationLength int `json:"aggregation-length,omitempty"`

	// BGP LOCAL_PREF attribute which is used by BGP best path algorithm,
	// Path with higher localpref is preferred over one with lower localpref.
	LocalPref uint32 `json:"localpref,omitempty"`

	// BGP communities
	Communities []string `json:"communities,omitempty"`
}

type AddressPool struct {
	// Address Pool Name
	Name string `json:"name"`

	// Protocol can be used to select how the announcement is done,
	// +kubebuilder:validation:Enum:=layer2; bgp
	Protocol string `json:"protocol"`

	// BGP Advertisement allow user to customize BGP advertisements, by default
	// BGP advertise /32 with localpref set to 0 and no BGP communities.
	BgpAdvertisements []BgpAdvertisement `json:"bgp-advertisements,omitempty"`

	// A list of IP address ranges over which MetalLB has authority.
	// You can list multiple ranges in a single pool, they will all share the
	// same settings. Each range can be either a CIDR prefix, or an explicit
	// start-end range of IPs.
	Addresses []string `json:"addresses"`

	// auto-assign flag used to prevent MetallB from automatic allocation
	// for a pool.
	// +optional
	AutoAssign bool `json:"auto-assign,omitempty"`

	// Avoid buggy ips is used to mark .0 and .255 as unusable.
	// +optional
	AvoidBuggyIPs bool `json:"avoid-buggy-ips,omitempty"`
}

// AddressPoolsSpec defines the desired state of AddressPools
type AddressPoolsSpec struct {
	AddressPools []AddressPool `json:"address-pools"`
}

// AddressPoolsStatus defines the observed state of AddressPools
type AddressPoolsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// AddressPools is the Schema for the addresspools API
type AddressPools struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddressPoolsSpec   `json:"spec,omitempty"`
	Status AddressPoolsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AddressPoolsList contains a list of AddressPools
type AddressPoolsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AddressPools `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AddressPools{}, &AddressPoolsList{})
}
