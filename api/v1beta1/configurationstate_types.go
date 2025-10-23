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

// ConfigurationResult represents the validation result of a MetalLB configuration.
// +kubebuilder:validation:Enum=Valid;Invalid;Unknown
type ConfigurationResult string

const (
	// ConfigurationResultValid indicates configuration is successfully validated.
	ConfigurationResultValid ConfigurationResult = "Valid"
	// ConfigurationResultInvalid indicates configuration has errors.
	ConfigurationResultInvalid ConfigurationResult = "Invalid"
	// ConfigurationResultUnknown indicates component has not reported state.
	ConfigurationResultUnknown ConfigurationResult = "Unknown"
)

// ConfigurationStateStatus defines the observed state of ConfigurationState.
type ConfigurationStateStatus struct {
	// Result indicates the configuration validation result.
	// +optional
	Result ConfigurationResult `json:"result,omitempty"`

	// ErrorSummary contains the aggregated error messages from reconciliation failures.
	// This field is empty when Result is "Valid".
	// +optional
	ErrorSummary string `json:"errorSummary,omitempty"`

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
// +kubebuilder:printcolumn:name="ErrorSummary",type=string,JSONPath=`.status.errorSummary`
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
