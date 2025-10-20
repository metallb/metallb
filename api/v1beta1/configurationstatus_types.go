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

// MetalLBConfigurationStatus defines the observed state of ConfigurationStatus.
type MetalLBConfigurationStatus struct {
	// Conditions contains the status conditions from each controller.
	// Each condition reports the controller's reconciliation state:
	//   - Status: True (valid) or False (invalid)
	//   - Reason: The SyncState of the reconciler
	//   - Message: Error message during reconcile, if any (empty if no error)
	//
	// Example valid condition:
	//   - type: speaker-<node-name>/frrk8sReconcilerValid
	//     status: "True"
	//     reason: SyncStateSuccess
	//     message: ""
	//
	// Example invalid condition:
	//   - type: speaker-<node-name>/frrk8sReconcilerValid
	//     status: "False"
	//     reason: ConfigError
	//     message: 'failed to create or update frr configuration: admission webhook denied the request: different asns specified for same vrf'
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
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`

// ConfigurationStatus exposes the validation status of the overall MetalLB configuration.
type ConfigurationStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Status MetalLBConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigurationStatusList contains a list of ConfigurationStatus.
type ConfigurationStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ConfigurationStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigurationStatus{}, &ConfigurationStatusList{})
}
