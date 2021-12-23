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

type MatchExpression struct {
	Key      string `json:"key" yaml:"key"`
	Operator string `json:"operator" yaml:"operator"`
	// +kubebuilder:validation:MinItems:=1
	Values []string `json:"values" yaml:"values"`
}

type NodeSelector struct {
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty" yaml:"match-labels,omitempty"`

	// +optional
	MatchExpressions []MatchExpression `json:"matchExpressions,omitempty" yaml:"match-expressions,omitempty"`
}

// BGPPeerSpec defines the desired state of Peer
type BGPPeerSpec struct {
	// AS number to use for the local end of the session.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	MyASN uint32 `json:"myASN" yaml:"my-asn"`

	// AS number to expect from the remote end of the session.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	ASN uint32 `json:"peerASN" yaml:"peer-asn"`

	// Address to dial when establishing the session.
	Address string `json:"peerAddress" yaml:"peer-address"`

	// Source address to use when establishing the session.
	// +optional
	SrcAddress string `json:"sourceAddress,omitempty" yaml:"source-address,omitempty"`

	// Port to dial when establishing the session.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=16384
	Port uint16 `json:"peerPort,omitempty" yaml:"peer-port,omitempty"`

	// Requested BGP hold time, per RFC4271.
	// +optional
	HoldTime metav1.Duration `json:"holdTime,omitempty" yaml:"hold-time,omitempty"`

	// Requested BGP keepalive time, per RFC4271.
	// +optional
	KeepaliveTime metav1.Duration `json:"keepaliveTime,omitempty" yaml:"keepalive-time,omitempty"`

	// BGP router ID to advertise to the peer
	// +optional
	RouterID string `json:"routerID,omitempty" yaml:"router-id,omitempty"`

	// Only connect to this peer on nodes that match one of these
	// selectors.
	// +optional
	NodeSelectors []NodeSelector `json:"nodeSelectors,omitempty" yaml:"node-selectors,omitempty"`

	// Authentication password for routers enforcing TCP MD5 authenticated sessions
	// +optional
	Password string `json:"password,omitempty" yaml:"password,omitempty"`

	BFDProfile string `json:"bfdProfile,omitempty" yaml:"bfdprofile,omitempty"`

	// EBGP peer is multi-hops away
	EBGPMultiHop bool `json:"ebgpMultiHop,omitempty" yaml:"ebgp-multihop,omitempty"`
	// Add future BGP configuration here
}

// BGPPeerStatus defines the observed state of Peer
type BGPPeerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BGPPeer is the Schema for the peers API
type BGPPeer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BGPPeerSpec   `json:"spec,omitempty"`
	Status BGPPeerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PeerList contains a list of Peer
type BGPPeerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGPPeer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BGPPeer{}, &BGPPeerList{})
}
