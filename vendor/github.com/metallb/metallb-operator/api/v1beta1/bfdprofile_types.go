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

// BFDProfileSpec defines the desired state of BFDProfile
type BFDProfileSpec struct {
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	ReceiveInterval *uint32 `json:"receiveInterval,omitempty"`
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	TransmitInterval *uint32 `json:"transmitInterval,omitempty"`
	// +kubebuilder:validation:Maximum:=255
	// +kubebuilder:validation:Minimum:=2
	DetectMultiplier *uint32 `json:"detectMultiplier,omitempty"`
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	EchoInterval *uint32 `json:"echoInterval,omitempty"`
	EchoMode     *bool   `json:"echoMode,omitempty"`
	PassiveMode  *bool   `json:"passiveMode,omitempty"`
	// +kubebuilder:validation:Maximum:=254
	// +kubebuilder:validation:Minimum:=1
	MinimumTTL *uint32 `json:"minimumTtl,omitempty"`
}

// BFDProfileStatus defines the observed state of BFDProfile
type BFDProfileStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BFDProfile is the Schema for the bfdprofiles API
type BFDProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BFDProfileSpec   `json:"spec,omitempty"`
	Status BFDProfileStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BFDProfileList contains a list of BFDProfile
type BFDProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BFDProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BFDProfile{}, &BFDProfileList{})
}
