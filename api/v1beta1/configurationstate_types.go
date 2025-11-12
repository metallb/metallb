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

// ConfigurationStateResult represents the validation result of a MetalLB configuration.
// +kubebuilder:validation:Enum=Valid;Invalid;Unknown
type ConfigurationStateResult string

const (
	// ConfigurationStateResultValid indicates configuration is successfully validated.
	ConfigurationStateResultValid ConfigurationStateResult = "Valid"
	// ConfigurationStateResultInvalid indicates configuration has errors.
	ConfigurationStateResultInvalid ConfigurationStateResult = "Invalid"
	// ConfigurationStateResultUnknown indicates component has not reported state.
	ConfigurationStateResultUnknown ConfigurationStateResult = "Unknown"
)

// ConfigurationStateStatus defines the observed state of ConfigurationState.
type ConfigurationStateStatus struct {
	// Result indicates the configuration validation result.
	// +optional
	Result ConfigurationStateResult `json:"result,omitempty"`

	// LastError contains the error message from the last reconciliation failure.
	// This field is empty when Result is "Valid".
	// +optional
	LastError string `json:"lastError,omitempty"`

	// Conditions contains the status conditions from the reconcilers running in this component.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Result",type=string,JSONPath=`.status.result`
// +kubebuilder:printcolumn:name="LastError",type=string,JSONPath=`.status.lastError`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ConfigurationState is a status-only CRD that reports configuration validation results from MetalLB components.
// Labels:
//   - metallb.io/component-type: "controller" or "speaker"
//   - metallb.io/node-name: node name (only for speaker)
type ConfigurationState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status ConfigurationStateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigurationStateList contains a list of ConfigurationState.
type ConfigurationStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ConfigurationState `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigurationState{}, &ConfigurationStateList{})
}
