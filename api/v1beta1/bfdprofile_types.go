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

// BFDProfileSpec defines the desired state of BFDProfile.
type BFDProfileSpec struct {
	// The minimum interval that this system is capable of
	// receiving control packets in milliseconds.
	// Defaults to 300ms.
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	// +optional
	ReceiveInterval *uint32 `json:"receiveInterval,omitempty"`
	// The minimum transmission interval (less jitter)
	// that this system wants to use to send BFD control packets in
	// milliseconds. Defaults to 300ms
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	// +optional
	TransmitInterval *uint32 `json:"transmitInterval,omitempty"`
	// Configures the detection multiplier to determine
	// packet loss. The remote transmission interval will be multiplied
	// by this value to determine the connection loss detection timer.
	// +kubebuilder:validation:Maximum:=255
	// +kubebuilder:validation:Minimum:=2
	// +optional
	DetectMultiplier *uint32 `json:"detectMultiplier,omitempty"`
	// Configures the minimal echo receive transmission
	// interval that this system is capable of handling in milliseconds.
	// Defaults to 50ms
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	// +optional
	EchoInterval *uint32 `json:"echoInterval,omitempty"`
	// Enables or disables the echo transmission mode.
	// This mode is disabled by default, and not supported on multi
	// hops setups.
	// +optional
	EchoMode *bool `json:"echoMode,omitempty"`
	// Mark session as passive: a passive session will not
	// attempt to start the connection and will wait for control packets
	// from peer before it begins replying.
	// +optional
	PassiveMode *bool `json:"passiveMode,omitempty"`
	// For multi hop sessions only: configure the minimum
	// expected TTL for an incoming BFD control packet.
	// +kubebuilder:validation:Maximum:=254
	// +kubebuilder:validation:Minimum:=1
	// +optional
	MinimumTTL *uint32 `json:"minimumTtl,omitempty"`
}

// BFDProfileStatus defines the observed state of BFDProfile.
type BFDProfileStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BFDProfile represents the settings of the bfd session that can be
// optionally associated with a BGP session.
type BFDProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BFDProfileSpec   `json:"spec,omitempty"`
	Status BFDProfileStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BFDProfileList contains a list of BFDProfile.
type BFDProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BFDProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BFDProfile{}, &BFDProfileList{})
}
