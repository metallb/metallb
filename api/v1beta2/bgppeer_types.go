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
	// ASN and DynamicASN are mutually exclusive and one of them must be specified.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	// +optional
	ASN uint32 `json:"peerASN,omitempty"`

	// DynamicASN detects the AS number to use for the remote end of the session
	// without explicitly setting it via the ASN field. Limited to:
	// internal - if the neighbor's ASN is different than MyASN connection is denied.
	// external - if the neighbor's ASN is the same as MyASN the connection is denied.
	// ASN and DynamicASN are mutually exclusive and one of them must be specified.
	// +kubebuilder:validation:Enum=internal;external
	// +optional
	DynamicASN DynamicASNMode `json:"dynamicASN,omitempty"`

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
	HoldTime *metav1.Duration `json:"holdTime,omitempty"`

	// Requested BGP keepalive time, per RFC4271.
	// +optional
	KeepaliveTime *metav1.Duration `json:"keepaliveTime,omitempty"`

	// Requested BGP connect time, controls how long BGP waits between connection attempts to a neighbor.
	// +kubebuilder:validation:XValidation:message="connect time should be between 1 seconds to 65535",rule="duration(self).getSeconds() >= 1 && duration(self).getSeconds() <= 65535"
	// +kubebuilder:validation:XValidation:message="connect time should contain a whole number of seconds",rule="duration(self).getMilliseconds() % 1000 == 0"
	// +optional
	ConnectTime *metav1.Duration `json:"connectTime,omitempty"`

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

	// EnableGracefulRestart allows BGP peer to continue to forward data packets
	// along known routes while the routing protocol information is being
	// restored. This field is immutable because it requires restart of the BGP
	// session. Supported for FRR mode only.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="EnableGracefulRestart cannot be changed after creation"
	EnableGracefulRestart bool `json:"enableGracefulRestart,omitempty"`

	// To set if the BGPPeer is multi-hops away. Needed for FRR mode only.
	// +optional
	EBGPMultiHop bool `json:"ebgpMultiHop,omitempty"`

	// To set if we want to peer with the BGPPeer using an interface belonging to
	// a host vrf
	// +optional
	VRFName string `json:"vrf,omitempty"`
	// Add future BGP configuration here

	// To set if we want to disable MP BGP that will separate IPv4 and IPv6 route exchanges into distinct BGP sessions.
	// +optional
	// +kubebuilder:default:=false
	DisableMP bool `json:"disableMP,omitempty"`
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

type DynamicASNMode string

const (
	InternalASNMode DynamicASNMode = "internal"
	ExternalASNMode DynamicASNMode = "external"
)
