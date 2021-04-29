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
	"time"
)

type Peer struct {
	// AS number to use for the local end of the session.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	MyASN uint32 `json:"my-asn"`

	// AS number to expect from the remote end of the session.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	ASN uint32 `json:"peer-asn"`

	// Address to dial when establishing the session.
	Addr string `json:"peer-address"`

	// Port to dial when establishing the session.
	// +optional
	Port uint16 `json:"peer-port,omitempty"`

	// Requested BGP hold time, per RFC4271.
	// +optional
	HoldTime time.Duration `json:"hold-time,omitempty"`

	// BGP router ID to advertise to the peer
	// +optional
	RouterID string `json:"router-id,omitempty"`

	// Only connect to this peer on nodes that match one of these
	// selectors.
	// +optional
	NodeSelectors []NodeSelector `json:"node-selectors,omitempty"`

	// Authentication password for routers enforcing TCP MD5 authenticated sessions
	// +optional
	Password string `json:"password,omitempty"`

	// Add  future BGP configuration here
}

type MatchExpression struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	// +kubebuilder:validation:MinItems:=1
	Values []string `json:"values"`
}

type NodeSelector struct {
	// +optional
	MatchLabels map[string]string `json:"match-labels,omitempty"`

	// +optional
	MatchExpressions []MatchExpression `json:"match-expressions,omitempty"`
}

// PeersSpec defines the desired state of Peers
type PeersSpec struct {
	Peers []Peer `json:"peers,omitempty"`
}

// PeersStatus defines the observed state of Peers
type PeersStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// Peers is the Schema for the peers API
type Peers struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PeersSpec   `json:"spec,omitempty"`
	Status PeersStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PeersList contains a list of Peers
type PeersList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Peers `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Peers{}, &PeersList{})
}
