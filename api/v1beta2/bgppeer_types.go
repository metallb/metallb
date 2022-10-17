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

package v1beta2

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BGPPeerSpec defines the desired state of Peer.
type BGPPeerSpec struct {
	// AS number to use for the local end of the session.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	MyASN uint32 `json:"myASN"`

	// AS number to expect from the remote end of the session.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	ASN uint32 `json:"peerASN"`

	// Address to dial when establishing the session.
	Address string `json:"peerAddress"`

	// Source address to use when establishing the session.
	// +optional
	SrcAddress string `json:"sourceAddress,omitempty"`

	// Port to dial when establishing the session.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=16384
	// +kubebuilder:default:=179
	Port uint16 `json:"peerPort,omitempty"`

	// Requested BGP hold time, per RFC4271.
	// +optional
	HoldTime metav1.Duration `json:"holdTime,omitempty"`

	// Requested BGP keepalive time, per RFC4271.
	// +optional
	KeepaliveTime metav1.Duration `json:"keepaliveTime,omitempty"`

	// BGP router ID to advertise to the peer
	// +optional
	RouterID string `json:"routerID,omitempty"`

	// Only connect to this peer on nodes that match one of these
	// selectors.
	// +optional
	NodeSelectors []metav1.LabelSelector `json:"nodeSelectors,omitempty"`

	// Authentication password for routers enforcing TCP MD5 authenticated sessions
	// +optional
	Password string `json:"password,omitempty"`

	// passwordSecret is name of the authentication secret for BGP Peer.
	// the secret must be of type "kubernetes.io/basic-auth", and created in the
	// same namespace as the MetalLB deployment. The password is stored in the
	// secret as the key "password".
	// +optional
	PasswordSecret v1.SecretReference `json:"passwordSecret,omitempty"`

	// The name of the BFD Profile to be used for the BFD session associated to the BGP session. If not set, the BFD session won't be set up.
	// +optional
	BFDProfile string `json:"bfdProfile,omitempty"`

	// To set if the BGPPeer is multi-hops away. Needed for FRR mode only.
	// +optional
	EBGPMultiHop bool `json:"ebgpMultiHop,omitempty"`
	// Add future BGP configuration here
}

// BGPPeerStatus defines the observed state of Peer.
type BGPPeerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.spec.peerAddress`
//+kubebuilder:printcolumn:name="ASN",type=string,JSONPath=`.spec.peerASN`
//+kubebuilder:printcolumn:name="BFD Profile",type=string,JSONPath=`.spec.bfdProfile`
//+kubebuilder:printcolumn:name="Multi Hops",type=string,JSONPath=`.spec.ebgpMultiHop`

// BGPPeer is the Schema for the peers API.
type BGPPeer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BGPPeerSpec   `json:"spec,omitempty"`
	Status BGPPeerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PeerList contains a list of Peer.
type BGPPeerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGPPeer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BGPPeer{}, &BGPPeerList{})
}
