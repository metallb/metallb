/*
Copyright 2022.

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

type CommunityAlias struct {
	// The name of the alias for the community.
	Name string `json:"name,omitempty"`
	// The BGP community value corresponding to the given name. Can be a standard community of the form 1234:1234
	// or a large community of the form large:1234:1234:1234.
	Value string `json:"value,omitempty"`
}

// CommunitySpec defines the desired state of Community.
type CommunitySpec struct {
	Communities []CommunityAlias `json:"communities,omitempty"`
}

// CommunityStatus defines the observed state of Community.
type CommunityStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Community is a collection of aliases for communities.
// Users can define named aliases to be used in the BGPPeer CRD.
type Community struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CommunitySpec   `json:"spec,omitempty"`
	Status CommunityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CommunityList contains a list of Community.
type CommunityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Community `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Community{}, &CommunityList{})
}
