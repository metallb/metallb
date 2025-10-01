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

// ConfigurationStateSpec defines the desired state of ConfigurationState.
type ConfigurationStateSpec struct {
	// Type identifies whether this is a controller or speaker instance.
	// +kubebuilder:validation:Enum=Controller;Speaker
	Type string `json:"type"`

	// NodeName is set when Type is "Speaker" to identify which node this speaker is running on.
	// +optional
	NodeName string `json:"nodeName,omitempty"`
}

// ConfigurationStateStatus defines the observed state of ConfigurationState.
type ConfigurationStateStatus struct {
	// ValidConfig indicates whether the configuration is valid.
	// True when all reconcilers report success, False otherwise.
	// +optional
	ValidConfig bool `json:"validConfig,omitempty"`

	// LastError contains the error message from the last reconciliation failure.
	// Empty when ValidConfig is true.
	// +optional
	LastError string `json:"lastError,omitempty"`

	// Conditions contains the status conditions from the reconcilers running in this component.
	// Each condition reports a reconciler's state:
	//   - Status: True (valid) or False (invalid)
	//   - Reason: The SyncState of the reconciler
	//   - Message: Error message during reconcile, if any (empty if no error)
	//
	// Controller example (valid condition):
	//   - type: poolReconcilerValid
	//     status: "True"
	//     reason: SyncStateSuccess
	//     message: ""
	//
	// Controller example (invalid condition):
	//   - type: poolReconcilerValid
	//     status: "False"
	//     reason: ConfigError
	//     message: 'failed to parse configuration: CIDR overlaps'
	//
	// Speaker example (valid condition):
	//   - type: configReconcilerValid
	//     status: "True"
	//     reason: SyncStateSuccess
	//     message: ""
	//
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
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.spec.nodeName`
// +kubebuilder:printcolumn:name="Valid",type=boolean,JSONPath=`.status.validConfig`
// +kubebuilder:printcolumn:name="LastError",type=string,JSONPath=`.status.lastError`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ConfigurationState exposes the validation status of MetalLB components.
// One instance is created by the controller, and one instance per speaker pod.
type ConfigurationState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationStateSpec   `json:"spec,omitempty"`
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
