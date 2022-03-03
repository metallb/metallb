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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.

// BGPAdvertisementSpec defines the desired state of BGPAdvertisement.
type BGPAdvertisementSpec struct {
	// The aggregation-length advertisement option lets you “roll up” the /32s into a larger prefix.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=32
	// +optional
	AggregationLength *int32 `json:"aggregationLength,omitempty"`

	// Optional, defaults to 128 (i.e. no aggregation) if not
	// specified.
	// +kubebuilder:default:=128
	// +optional
	AggregationLengthV6 *int32 `json:"aggregationLengthV6,omitempty"`

	// BGP LOCAL_PREF attribute which is used by BGP best path algorithm,
	// Path with higher localpref is preferred over one with lower localpref.
	LocalPref uint32 `json:"localPref,omitempty"`

	// BGP communities
	Communities []string `json:"communities,omitempty"`

	// IPPools is the list of ippools to advertise via this advertisement.
	IPPools []string `json:"ipPools,omitempty"`
}

// BGPAdvertisementStatus defines the observed state of BGPAdvertisement.
type BGPAdvertisementStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BGPAdvertisement is the Schema for the bgpadvertisements API.
type BGPAdvertisement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BGPAdvertisementSpec   `json:"spec,omitempty"`
	Status BGPAdvertisementStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BGPAdvertisementList contains a list of BGPAdvertisement.
type BGPAdvertisementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGPAdvertisement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BGPAdvertisement{}, &BGPAdvertisementList{})
}
