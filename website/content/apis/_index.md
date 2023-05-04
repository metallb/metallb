---
title: API reference docs
description: MetalLB API reference documentation
---
{{< rawhtml >}}
<p>Packages:</p>
<ul>
<li>
<a href="#metallb.io%2fv1beta1">metallb.io/v1beta1</a>
</li>
<li>
<a href="#metallb.io%2fv1beta2">metallb.io/v1beta2</a>
</li>
</ul>
<h2 id="metallb.io/v1beta1">metallb.io/v1beta1</h2>
<div>
</div>
Resource Types:
<ul></ul>
<h3 id="metallb.io/v1beta1.BFDProfile">BFDProfile
</h3>
<div>
<p>BFDProfile represents the settings of the bfd session that can be
optionally associated with a BGP session.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#metallb.io/v1beta1.BFDProfileSpec">
BFDProfileSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>receiveInterval</code><br/>
<em>
uint32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The minimum interval that this system is capable of
receiving control packets in milliseconds.
Defaults to 300ms.</p>
</td>
</tr>
<tr>
<td>
<code>transmitInterval</code><br/>
<em>
uint32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The minimum transmission interval (less jitter)
that this system wants to use to send BFD control packets in
milliseconds. Defaults to 300ms</p>
</td>
</tr>
<tr>
<td>
<code>detectMultiplier</code><br/>
<em>
uint32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configures the detection multiplier to determine
packet loss. The remote transmission interval will be multiplied
by this value to determine the connection loss detection timer.</p>
</td>
</tr>
<tr>
<td>
<code>echoInterval</code><br/>
<em>
uint32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configures the minimal echo receive transmission
interval that this system is capable of handling in milliseconds.
Defaults to 50ms</p>
</td>
</tr>
<tr>
<td>
<code>echoMode</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enables or disables the echo transmission mode.
This mode is disabled by default, and not supported on multi
hops setups.</p>
</td>
</tr>
<tr>
<td>
<code>passiveMode</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Mark session as passive: a passive session will not
attempt to start the connection and will wait for control packets
from peer before it begins replying.</p>
</td>
</tr>
<tr>
<td>
<code>minimumTtl</code><br/>
<em>
uint32
</em>
</td>
<td>
<em>(Optional)</em>
<p>For multi hop sessions only: configure the minimum
expected TTL for an incoming BFD control packet.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#metallb.io/v1beta1.BFDProfileStatus">
BFDProfileStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="metallb.io/v1beta1.BGPAdvertisement">BGPAdvertisement
</h3>
<div>
<p>BGPAdvertisement allows to advertise the IPs coming
from the selected IPAddressPools via BGP, setting the parameters of the
BGP Advertisement.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#metallb.io/v1beta1.BGPAdvertisementSpec">
BGPAdvertisementSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>aggregationLength</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The aggregation-length advertisement option lets you “roll up” the /32s into a larger prefix. Defaults to 32. Works for IPv4 addresses.</p>
</td>
</tr>
<tr>
<td>
<code>aggregationLengthV6</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The aggregation-length advertisement option lets you “roll up” the /128s into a larger prefix. Defaults to 128. Works for IPv6 addresses.</p>
</td>
</tr>
<tr>
<td>
<code>localPref</code><br/>
<em>
uint32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The BGP LOCAL_PREF attribute which is used by BGP best path algorithm,
Path with higher localpref is preferred over one with lower localpref.</p>
</td>
</tr>
<tr>
<td>
<code>communities</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The BGP communities to be associated with the announcement. Each item can be a standard community of the
form 1234:1234, a large community of the form large:1234:1234:1234 or the name of an alias defined in the
Community CRD.</p>
</td>
</tr>
<tr>
<td>
<code>ipAddressPools</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The list of IPAddressPools to advertise via this advertisement, selected by name.</p>
</td>
</tr>
<tr>
<td>
<code>ipAddressPoolSelectors</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#labelselector-v1-meta">
[]Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A selector for the IPAddressPools which would get advertised via this advertisement.
If no IPAddressPool is selected by this or by the list, the advertisement is applied to all the IPAddressPools.</p>
</td>
</tr>
<tr>
<td>
<code>nodeSelectors</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#labelselector-v1-meta">
[]Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeSelectors allows to limit the nodes to announce as next hops for the LoadBalancer IP. When empty, all the nodes having  are announced as next hops.</p>
</td>
</tr>
<tr>
<td>
<code>peers</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Peers limits the bgppeer to advertise the ips of the selected pools to.
When empty, the loadbalancer IP is announced to all the BGPPeers configured.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#metallb.io/v1beta1.BGPAdvertisementStatus">
BGPAdvertisementStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="metallb.io/v1beta1.Community">Community
</h3>
<div>
<p>Community is a collection of aliases for communities.
Users can define named aliases to be used in the BGPPeer CRD.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#metallb.io/v1beta1.CommunitySpec">
CommunitySpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>communities</code><br/>
<em>
<a href="#metallb.io/v1beta1.CommunityAlias">
[]CommunityAlias
</a>
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#metallb.io/v1beta1.CommunityStatus">
CommunityStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="metallb.io/v1beta1.CommunityAlias">CommunityAlias
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the alias for the community.</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<p>The BGP community value corresponding to the given name. Can be a standard community of the form 1234:1234
or a large community of the form large:1234:1234:1234.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="metallb.io/v1beta1.IPAddressPool">IPAddressPool
</h3>
<div>
<p>IPAddressPool represents a pool of IP addresses that can be allocated
to LoadBalancer services.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#metallb.io/v1beta1.IPAddressPoolSpec">
IPAddressPoolSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>addresses</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>A list of IP address ranges over which MetalLB has authority.
You can list multiple ranges in a single pool, they will all share the
same settings. Each range can be either a CIDR prefix, or an explicit
start-end range of IPs.</p>
</td>
</tr>
<tr>
<td>
<code>autoAssign</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>AutoAssign flag used to prevent MetallB from automatic allocation
for a pool.</p>
</td>
</tr>
<tr>
<td>
<code>avoidBuggyIPs</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>AvoidBuggyIPs prevents addresses ending with .0 and .255
to be used by a pool.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAllocation</code><br/>
<em>
<a href="#metallb.io/v1beta1.ServiceAllocation">
ServiceAllocation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AllocateTo makes ip pool allocation to specific namespace and/or service.
The controller will use the pool with lowest value of priority in case of
multiple matches. A pool with no priority set will be used only if the
pools with priority can&rsquo;t be used. If multiple matching IPAddressPools are
available it will check for the availability of IPs sorting the matching
IPAddressPools by priority, starting from the highest to the lowest. If
multiple IPAddressPools have the same priority, choice will be random.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#metallb.io/v1beta1.IPAddressPoolStatus">
IPAddressPoolStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="metallb.io/v1beta1.L2Advertisement">L2Advertisement
</h3>
<div>
<p>L2Advertisement allows to advertise the LoadBalancer IPs provided
by the selected pools via L2.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#metallb.io/v1beta1.L2AdvertisementSpec">
L2AdvertisementSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>ipAddressPools</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The list of IPAddressPools to advertise via this advertisement, selected by name.</p>
</td>
</tr>
<tr>
<td>
<code>ipAddressPoolSelectors</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#labelselector-v1-meta">
[]Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A selector for the IPAddressPools which would get advertised via this advertisement.
If no IPAddressPool is selected by this or by the list, the advertisement is applied to all the IPAddressPools.</p>
</td>
</tr>
<tr>
<td>
<code>nodeSelectors</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#labelselector-v1-meta">
[]Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeSelectors allows to limit the nodes to announce as next hops for the LoadBalancer IP. When empty, all the nodes having  are announced as next hops.</p>
</td>
</tr>
<tr>
<td>
<code>interfaces</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of interfaces to announce from. The LB IP will be announced only from these interfaces.
If the field is not set, we advertise from all the interfaces on the host.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#metallb.io/v1beta1.L2AdvertisementStatus">
L2AdvertisementStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="metallb.io/v1beta1.ServiceAllocation">ServiceAllocation
</h3>
<div>
<p>ServiceAllocation defines ip pool allocation to namespace and/or service.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>priority</code><br/>
<em>
int
</em>
</td>
<td>
<p>Priority priority given for ip pool while ip allocation on a service.</p>
</td>
</tr>
<tr>
<td>
<code>namespaces</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Namespaces list of namespace(s) on which ip pool can be attached.</p>
</td>
</tr>
<tr>
<td>
<code>namespaceSelectors</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#labelselector-v1-meta">
[]Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>NamespaceSelectors list of label selectors to select namespace(s) for ip pool,
an alternative to using namespace list.</p>
</td>
</tr>
<tr>
<td>
<code>serviceSelectors</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#labelselector-v1-meta">
[]Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>ServiceSelectors list of label selector to select service(s) for which ip pool
can be used for ip allocation.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<h2 id="metallb.io/v1beta2">metallb.io/v1beta2</h2>
<div>
</div>
Resource Types:
<ul></ul>
<h3 id="metallb.io/v1beta2.BGPPeer">BGPPeer
</h3>
<div>
<p>BGPPeer is the Schema for the peers API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#metallb.io/v1beta2.BGPPeerSpec">
BGPPeerSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>myASN</code><br/>
<em>
uint32
</em>
</td>
<td>
<p>AS number to use for the local end of the session.</p>
</td>
</tr>
<tr>
<td>
<code>peerASN</code><br/>
<em>
uint32
</em>
</td>
<td>
<p>AS number to expect from the remote end of the session.</p>
</td>
</tr>
<tr>
<td>
<code>peerAddress</code><br/>
<em>
string
</em>
</td>
<td>
<p>Address to dial when establishing the session.</p>
</td>
</tr>
<tr>
<td>
<code>sourceAddress</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Source address to use when establishing the session.</p>
</td>
</tr>
<tr>
<td>
<code>peerPort</code><br/>
<em>
uint16
</em>
</td>
<td>
<em>(Optional)</em>
<p>Port to dial when establishing the session.</p>
</td>
</tr>
<tr>
<td>
<code>holdTime</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Requested BGP hold time, per RFC4271.</p>
</td>
</tr>
<tr>
<td>
<code>keepaliveTime</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Requested BGP keepalive time, per RFC4271.</p>
</td>
</tr>
<tr>
<td>
<code>routerID</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>BGP router ID to advertise to the peer</p>
</td>
</tr>
<tr>
<td>
<code>nodeSelectors</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#labelselector-v1-meta">
[]Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Only connect to this peer on nodes that match one of these
selectors.</p>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Authentication password for routers enforcing TCP MD5 authenticated sessions</p>
</td>
</tr>
<tr>
<td>
<code>passwordSecret</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>passwordSecret is name of the authentication secret for BGP Peer.
the secret must be of type &ldquo;kubernetes.io/basic-auth&rdquo;, and created in the
same namespace as the MetalLB deployment. The password is stored in the
secret as the key &ldquo;password&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>bfdProfile</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the BFD Profile to be used for the BFD session associated to the BGP session. If not set, the BFD session won&rsquo;t be set up.</p>
</td>
</tr>
<tr>
<td>
<code>ebgpMultiHop</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>To set if the BGPPeer is multi-hops away. Needed for FRR mode only.</p>
</td>
</tr>
<tr>
<td>
<code>vrf</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>To set if we want to peer with the BGPPeer using an interface belonging to
a host vrf</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#metallb.io/v1beta2.BGPPeerStatus">
BGPPeerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
.
</em></p>
