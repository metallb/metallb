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

// BgpCommunitiesSpec defines the desired state of BgpCommunities
type BgpCommunitiesSpec struct {
	BGPCommunities map[string]string `json:"bgp-communities,omitempty"`
}

// BgpCommunitiesStatus defines the observed state of BgpCommunities
type BgpCommunitiesStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// BgpCommunities is the Schema for the bgpcommunities API
type BgpCommunities struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BgpCommunitiesSpec   `json:"spec,omitempty"`
	Status BgpCommunitiesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BgpCommunitiesList contains a list of BgpCommunities
type BgpCommunitiesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BgpCommunities `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BgpCommunities{}, &BgpCommunitiesList{})
}
