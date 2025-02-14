---
title: API reference docs
description: MetalLB API reference documentation
---
# API Reference

## Packages
- [metallb.io/v1beta1](#metallbiov1beta1)
- [metallb.io/v1beta2](#metallbiov1beta2)


## metallb.io/v1beta1



### Resource Types
- [BFDProfile](#bfdprofile)
- [BGPAdvertisement](#bgpadvertisement)
- [Community](#community)
- [IPAddressPool](#ipaddresspool)
- [L2Advertisement](#l2advertisement)



#### BFDProfile



BFDProfile represents the settings of the bfd session that can be
optionally associated with a BGP session.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `metallb.io/v1beta1`
| `kind` _string_ | `BFDProfile`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[BFDProfileSpec](#bfdprofilespec)_ |  |


#### BFDProfileSpec



BFDProfileSpec defines the desired state of BFDProfile.

_Appears in:_
- [BFDProfile](#bfdprofile)

| Field | Description |
| --- | --- |
| `receiveInterval` _integer_ | The minimum interval that this system is capable of<br />receiving control packets in milliseconds.<br />Defaults to 300ms. |
| `transmitInterval` _integer_ | The minimum transmission interval (less jitter)<br />that this system wants to use to send BFD control packets in<br />milliseconds. Defaults to 300ms |
| `detectMultiplier` _integer_ | Configures the detection multiplier to determine<br />packet loss. The remote transmission interval will be multiplied<br />by this value to determine the connection loss detection timer. |
| `echoInterval` _integer_ | Configures the minimal echo receive transmission<br />interval that this system is capable of handling in milliseconds.<br />Defaults to 50ms |
| `echoMode` _boolean_ | Enables or disables the echo transmission mode.<br />This mode is disabled by default, and not supported on multi<br />hops setups. |
| `passiveMode` _boolean_ | Mark session as passive: a passive session will not<br />attempt to start the connection and will wait for control packets<br />from peer before it begins replying. |
| `minimumTtl` _integer_ | For multi hop sessions only: configure the minimum<br />expected TTL for an incoming BFD control packet. |


#### BGPAdvertisement



BGPAdvertisement allows to advertise the IPs coming
from the selected IPAddressPools via BGP, setting the parameters of the
BGP Advertisement.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `metallb.io/v1beta1`
| `kind` _string_ | `BGPAdvertisement`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[BGPAdvertisementSpec](#bgpadvertisementspec)_ |  |


#### BGPAdvertisementSpec



BGPAdvertisementSpec defines the desired state of BGPAdvertisement.

_Appears in:_
- [BGPAdvertisement](#bgpadvertisement)

| Field | Description |
| --- | --- |
| `aggregationLength` _integer_ | The aggregation-length advertisement option lets you “roll up” the /32s into a larger prefix. Defaults to 32. Works for IPv4 addresses. |
| `aggregationLengthV6` _integer_ | The aggregation-length advertisement option lets you “roll up” the /128s into a larger prefix. Defaults to 128. Works for IPv6 addresses. |
| `localPref` _integer_ | The BGP LOCAL_PREF attribute which is used by BGP best path algorithm,<br />Path with higher localpref is preferred over one with lower localpref. |
| `communities` _string array_ | The BGP communities to be associated with the announcement. Each item can be a standard community of the<br />form 1234:1234, a large community of the form large:1234:1234:1234 or the name of an alias defined in the<br />Community CRD. |
| `ipAddressPools` _string array_ | The list of IPAddressPools to advertise via this advertisement, selected by name. |
| `ipAddressPoolSelectors` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta) array_ | A selector for the IPAddressPools which would get advertised via this advertisement.<br />If no IPAddressPool is selected by this or by the list, the advertisement is applied to all the IPAddressPools. |
| `nodeSelectors` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta) array_ | NodeSelectors allows to limit the nodes to announce as next hops for the LoadBalancer IP. When empty, all the nodes having  are announced as next hops. |
| `peers` _string array_ | Peers limits the bgppeer to advertise the ips of the selected pools to.<br />When empty, the loadbalancer IP is announced to all the BGPPeers configured. |


#### Community



Community is a collection of aliases for communities.
Users can define named aliases to be used in the BGPPeer CRD.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `metallb.io/v1beta1`
| `kind` _string_ | `Community`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[CommunitySpec](#communityspec)_ |  |


#### CommunityAlias





_Appears in:_
- [CommunitySpec](#communityspec)

| Field | Description |
| --- | --- |
| `name` _string_ | The name of the alias for the community. |
| `value` _string_ | The BGP community value corresponding to the given name. Can be a standard community of the form 1234:1234<br />or a large community of the form large:1234:1234:1234. |


#### CommunitySpec



CommunitySpec defines the desired state of Community.

_Appears in:_
- [Community](#community)

| Field | Description |
| --- | --- |
| `communities` _[CommunityAlias](#communityalias) array_ |  |


#### IPAddressPool



IPAddressPool represents a pool of IP addresses that can be allocated
to LoadBalancer services.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `metallb.io/v1beta1`
| `kind` _string_ | `IPAddressPool`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[IPAddressPoolSpec](#ipaddresspoolspec)_ |  |


#### IPAddressPoolSpec



IPAddressPoolSpec defines the desired state of IPAddressPool.

_Appears in:_
- [IPAddressPool](#ipaddresspool)

| Field | Description |
| --- | --- |
| `addresses` _string array_ | A list of IP address ranges over which MetalLB has authority.<br />You can list multiple ranges in a single pool, they will all share the<br />same settings. Each range can be either a CIDR prefix, or an explicit<br />start-end range of IPs. |
| `autoAssign` _boolean_ | AutoAssign flag used to prevent MetallB from automatic allocation<br />for a pool. |
| `avoidBuggyIPs` _boolean_ | AvoidBuggyIPs prevents addresses ending with .0 and .255<br />to be used by a pool. |
| `serviceAllocation` _[ServiceAllocation](#serviceallocation)_ | AllocateTo makes ip pool allocation to specific namespace and/or service.<br />The controller will use the pool with lowest value of priority in case of<br />multiple matches. A pool with no priority set will be used only if the<br />pools with priority can't be used. If multiple matching IPAddressPools are<br />available it will check for the availability of IPs sorting the matching<br />IPAddressPools by priority, starting from the highest to the lowest. If<br />multiple IPAddressPools have the same priority, choice will be random. |




#### L2Advertisement



L2Advertisement allows to advertise the LoadBalancer IPs provided
by the selected pools via L2.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `metallb.io/v1beta1`
| `kind` _string_ | `L2Advertisement`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[L2AdvertisementSpec](#l2advertisementspec)_ |  |


#### L2AdvertisementSpec



L2AdvertisementSpec defines the desired state of L2Advertisement.

_Appears in:_
- [L2Advertisement](#l2advertisement)

| Field | Description |
| --- | --- |
| `ipAddressPools` _string array_ | The list of IPAddressPools to advertise via this advertisement, selected by name. |
| `ipAddressPoolSelectors` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta) array_ | A selector for the IPAddressPools which would get advertised via this advertisement.<br />If no IPAddressPool is selected by this or by the list, the advertisement is applied to all the IPAddressPools. |
| `nodeSelectors` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta) array_ | NodeSelectors allows to limit the nodes to announce as next hops for the LoadBalancer IP. When empty, all the nodes having  are announced as next hops. |
| `interfaces` _string array_ | A list of interfaces to announce from. The LB IP will be announced only from these interfaces.<br />If the field is not set, we advertise from all the interfaces on the host. |


#### ServiceAllocation



ServiceAllocation defines ip pool allocation to namespace and/or service.

_Appears in:_
- [IPAddressPoolSpec](#ipaddresspoolspec)

| Field | Description |
| --- | --- |
| `priority` _integer_ | Priority priority given for ip pool while ip allocation on a service. |
| `namespaces` _string array_ | Namespaces list of namespace(s) on which ip pool can be attached. |
| `namespaceSelectors` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta) array_ | NamespaceSelectors list of label selectors to select namespace(s) for ip pool,<br />an alternative to using namespace list. |
| `serviceSelectors` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta) array_ | ServiceSelectors list of label selector to select service(s) for which ip pool<br />can be used for ip allocation. |







## metallb.io/v1beta2



### Resource Types
- [BGPPeer](#bgppeer)



#### BGPPeer



BGPPeer is the Schema for the peers API.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `metallb.io/v1beta2`
| `kind` _string_ | `BGPPeer`
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[BGPPeerSpec](#bgppeerspec)_ |  |


#### BGPPeerSpec



BGPPeerSpec defines the desired state of Peer.

_Appears in:_
- [BGPPeer](#bgppeer)

| Field | Description |
| --- | --- |
| `myASN` _integer_ | AS number to use for the local end of the session. |
| `peerASN` _integer_ | AS number to expect from the remote end of the session.<br />ASN and DynamicASN are mutually exclusive and one of them must be specified. |
| `dynamicASN` _[DynamicASNMode](#dynamicasnmode)_ | DynamicASN detects the AS number to use for the remote end of the session<br />without explicitly setting it via the ASN field. Limited to:<br />internal - if the neighbor's ASN is different than MyASN connection is denied.<br />external - if the neighbor's ASN is the same as MyASN the connection is denied.<br />ASN and DynamicASN are mutually exclusive and one of them must be specified. |
| `peerAddress` _string_ | Address to dial when establishing the session. |
| `sourceAddress` _string_ | Source address to use when establishing the session. |
| `peerPort` _integer_ | Port to dial when establishing the session. |
| `holdTime` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#duration-v1-meta)_ | Requested BGP hold time, per RFC4271. |
| `keepaliveTime` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#duration-v1-meta)_ | Requested BGP keepalive time, per RFC4271. |
| `connectTime` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#duration-v1-meta)_ | Requested BGP connect time, controls how long BGP waits between connection attempts to a neighbor. |
| `routerID` _string_ | BGP router ID to advertise to the peer |
| `nodeSelectors` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta) array_ | Only connect to this peer on nodes that match one of these<br />selectors. |
| `password` _string_ | Authentication password for routers enforcing TCP MD5 authenticated sessions |
| `passwordSecret` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#secretreference-v1-core)_ | passwordSecret is name of the authentication secret for BGP Peer.<br />the secret must be of type "kubernetes.io/basic-auth", and created in the<br />same namespace as the MetalLB deployment. The password is stored in the<br />secret as the key "password". |
| `bfdProfile` _string_ | The name of the BFD Profile to be used for the BFD session associated to the BGP session. If not set, the BFD session won't be set up. |
| `enableGracefulRestart` _boolean_ | EnableGracefulRestart allows BGP peer to continue to forward data packets<br />along known routes while the routing protocol information is being<br />restored. This field is immutable because it requires restart of the BGP<br />session. Supported for FRR mode only. |
| `ebgpMultiHop` _boolean_ | To set if the BGPPeer is multi-hops away. Needed for FRR mode only. |
| `vrf` _string_ | To set if we want to peer with the BGPPeer using an interface belonging to<br />a host vrf |
| `disableMP` _boolean_ | To set if we want to disable MP BGP that will separate IPv4 and IPv6 route exchanges into distinct BGP sessions. |


#### DynamicASNMode

_Underlying type:_ _string_



_Appears in:_
- [BGPPeerSpec](#bgppeerspec)



