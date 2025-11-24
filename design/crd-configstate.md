# ConfigurationState CRD

As a cluster user, I want to verify that MetalLB CRs successfully applied to the API
are also successfully processed by MetalLB components. This CR surfaces configuration
errors currently hidden in logs, making them visible via standard kubectl commands.


## Configuration Errors

We define configuration errors as errors caused solely by user input that can only be
resolved by correcting that input. This is in contrast to infrastructure errors (such as
missing RBAC permissions to watch resources) which can be fixed by administrators through
deployment changes or code bug fixes.

**Note:** While webhooks cover most configuration errors, they cannot be solely relied upon.
In corner cases (e.g., many concurrent API calls), two conflicting CRs could be stored in the
API. Another common case is that the webhook explicitly ignores transient errors related to
resource ordering by resetting fields that could cause such errors (via the
`resetTransientErrorsFields` function).

**Note:** This CRD specifically does not cover errors that arise when the configuration
applied by the speaker to the `frr-k8s` daemon conflicts with other external configurations
within that daemon.

## API

One CRD definition, Multiple CRs, Namespaced.

```bash
$ kubectl get configurationstates -n metallb-system
NAME                    RESULT    LASTERROR   AGE
controller              Valid                 5m
speaker-kind-worker     Valid                 5m
speaker-kind-worker2    Valid                 5m
...

# Query by component type using labels
$ kubectl get configurationstates -n metallb-system -l metallb.io/component-type=speaker

# Query by node using labels
$ kubectl get configurationstates -n metallb-system -l metallb.io/node-name=kind-worker
```

```go
// ConfigurationState is a status-only CRD that reports configuration validation results
// from MetalLB components. The type and node information are conveyed through labels
// rather than spec fields, following Kubernetes best practices for status resources.
// Labels:
//   - metallb.io/component-type: "controller" or "speaker"
//   - metallb.io/node-name: node name (only for speaker)

// ConfigurationStateStatus defines the observed state of ConfigurationState.
type ConfigurationStateStatus struct {
	// Result indicates the configuration validation result.
	// Possible values:
	//   - "Valid": Configuration is successfully validated.
	//   - "Invalid": Configuration has errors.
	//   - "Unknown": Component has not reported state (e.g., during initialization or after a crash).
	// +optional
	// +kubebuilder:validation:Enum=Valid;Invalid;Unknown
	Result string `json:"result,omitempty"`

	// LastError contains the error message from the last reconciliation failure.
	// This field is empty when Result is "Valid".
	// +optional
	LastError string `json:"lastError,omitempty"`
}

```

* Initially, users will query individual ConfigurationState CRs per component
* Aggregation can be addressed in a future proposal/issue if needed
* Users can leverage label selectors to query specific subsets (e.g., all speakers, specific node)
* API does not provide the summary of the "input configuration", only the error
  which should indicate to the user which CR must be changed

## Controller side

For Controller (Deployment - single instance) there are three reconcilers

- PoolReconciler - yes, but only config errors, that error should appear in configState
- PoolStatusReconciler - no config errors
- ServiceReconciler - no config errors

```yaml
apiVersion: metallb.io/v1beta1
kind: ConfigurationState
metadata:
    name: controller
    namespace: metallb-system
    labels:
        metallb.io/component-type: controller
status:
    result: "Valid"
    lastError: ""

# Failed example
apiVersion: metallb.io/v1beta1
kind: ConfigurationState
metadata:
    name: controller
    namespace: metallb-system
    labels:
        metallb.io/component-type: controller
status:
    result: "Invalid"
    lastError: "failed to parse configuration: CIDR \"192.168.10.100/32\" in pool \"client2-pool\" overlaps with already defined CIDR \"192.168.10.0/24\""
```

## Speaker side

For Speaker (DaemonSet - one pod per node) there are

- ConfigReconciler - yes, but only config errors, that error should appear in configState
- NodeReconciler - no config errors
- FRRK8sReconciler - no config errors
- ServiceReconciler - no config errors
- ServiceBGPStatusReconciler - no config errors
- Layer2StatusReconciler - no config errors

```yaml
apiVersion: metallb.io/v1beta1
kind: ConfigurationState
metadata:
    name: speaker-kind-worker
    namespace: metallb-system
    labels:
        metallb.io/component-type: speaker
        metallb.io/node-name: kind-worker
status:
    result: "Valid"
    lastError: ""

# Failed example:
apiVersion: metallb.io/v1beta1
kind: ConfigurationState
metadata:
    name: speaker-kind-worker2
    namespace: metallb-system
    labels:
        metallb.io/component-type: speaker
        metallb.io/node-name: kind-worker2
status:
    result: "Invalid"
    lastError: "peer peer1 referencing non existing bfd profile my-bfd-profile"
```

## Implementation

Ideally like FRRNodeState pattern in metallb/frrk8s repo, but might not be
ideal because the ConfigurationStateReconciler needs references to ConfigReconciler
and NodeReconciler in-memory result. Alternative implementation based on
condition instead of channel can be evaluated. TBD and discussed during
the implementation PR.

The `res := r.Handler(r.Logger, cfg)` will not be refactored to return
an error. When handler returns SyncStateNoRetry result, that will be reported
in the ConfigurationState status.

E2E tests for transient errors can run without disabling webhooks. Tests for
other validation errors might require disabling the webhook.

## List of Configuration Errors that pass Webhooks

### Transient Errors

Transient errors occur due to interdependencies between CRDs where resources
reference other resources that don't exist yet. These errors are temporary and
resolve automatically when the missing resource is created. The webhook strips
fields that can cause transient errors to avoid making assumptions based on
object creation ordering.

- BFD Profile Reference
  - When: BGPPeer references a BFD profile that doesn't exist
  - Error: `peer %s referencing non existing bfd profile %s`
  - Code: [internal/config/config.go:290](https://github.com/metallb/metallb/blob/main/internal/config/config.go#L290)

- Password Secret Reference
  - When: BGPPeer references a password secret that doesn't exist
  - Error: `secret ref not found for peer config %q/%q`
  - Code: [internal/config/config.go:496](https://github.com/metallb/metallb/blob/main/internal/config/config.go#L496)

- Community Alias Reference
  - When: BGPAdvertisement references a community alias that doesn't exist
  - Error: Invalid community format
  - Code: [internal/config/config.go](https://github.com/metallb/metallb/blob/main/internal/config/config.go)

**Given** a MetalLB installation in namespace `metallb-system`

**When** user applies a BGPPeer that references a non-existent BFD profile:
```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: peer1
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64513
  peerAddress: 192.168.1.1
  bfdProfile: my-bfd-profile  # References profile that doesn't exist
```

**Then** speaker/configReconciler fails to load configuration

   ```bash
   kubectl apply -f bgppeer.yaml
   # Output: bgppeer.metallb.io/peer1 created

   kubectl logs -n metallb-system -l component=speaker -c speaker --tail=50 | grep "error"
   # Output: {"caller":"config_controller.go:140","controller":"ConfigReconciler","error":"peer peer1 referencing non existing bfd profile my-bfd-profile","level":"error"}
   ```

### Other Validation Errors

Other validation errors can occur for example when the secret exists
but has incorrect content. The user must fix the secret to resolve these
errors. The webhook strips the `passwordSecret` field to avoid accessing secret
content during validation. The reconciler validates the actual secret content.

- Secret Type Mismatch
  - When: BGPPeer references a secret that exists but has wrong type (not `kubernetes.io/basic-auth`)
  - Code: [internal/config/config.go:500](https://github.com/metallb/metallb/blob/main/internal/config/config.go#L500)

- Password Field Missing
  - When: BGPPeer references a secret that exists but doesn't have a `password` field
  - Code: [internal/config/config.go:505](https://github.com/metallb/metallb/blob/main/internal/config/config.go#L505)

- IPv6 Pool with BFD Echo
  - When: Pool has IPv6 CIDR, BGPAdvertisement references BGPPeer with BFDProfile that has echo mode enabled
  - Code: [internal/config/validation.go](https://github.com/metallb/metallb/blob/main/internal/config/validation.go)

**Given** a MetalLB installation in namespace `metallb-system`

**When** user applies a Secret with wrong type and BGPPeer that references it:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bgp-password
  namespace: metallb-system
type: Opaque  # Wrong type - should be kubernetes.io/basic-auth
stringData:
  password: "mypassword123"
---
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: peer-with-secret
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64513
  peerAddress: 192.168.1.2
  passwordSecret:
    name: bgp-password
```

**Then** speaker/configReconciler fails to load configuration

   ```bash
   kubectl apply -f secret-and-peer.yaml
   # Output:
   # secret/bgp-password created
   # bgppeer.metallb.io/peer-with-secret created

   kubectl logs -n metallb-system -l component=speaker -c speaker --tail=50 | grep "error"
   # Output: {"caller":"config_controller.go:140","controller":"ConfigReconciler","error":"parsing peer peer-with-secret secret type mismatch on \"metallb-system\"/\"bgp-password\", type \"kubernetes.io/basic-auth\" is expected \nfailed to parse peer peer-with-secret password secret","level":"error"}
   ```

## References

- https://github.com/k8snetworkplumbingwg/sriov-network-operator/pull/918/files
- Openshift Operators are doing a lot conditions to report overall state (Degraded) https://github.com/openshift/library-go/blob/91376e1b394e6eddd36c4421c44e2bb00503e3bf/pkg/operator/apiserver/controller/workload/workload.go#L181
- **[Kubernetes API Conventions - Status Fields](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status)**:
  Official guidelines for status subresources and conditions
- **[Knative Conditions](https://github.com/knative/pkg/blob/main/apis/condition_types.go)**:
  Standard condition types and structures used across Knative projects
- **[Knative Condition Set Manager](https://github.com/knative/pkg/blob/main/apis/condition_set.go)**:
  High-level condition management with automatic Ready condition calculation
- **[Controller Pitfalls: Report Status and Conditions](https://ahmet.im/blog/controller-pitfalls/#report-status-and-conditions)**:
  Real-world guidance on implementing conditions correctly, emphasizing observedGeneration importance
