# ConfigurationStatus CR

As a cluster user, I want to know if the MetalLB CRs applied with success to (no API/webhook error took place)
have been also successfully applied under the hood by looking a CRs of a CRD called ConfiguratonState.
That CRs should make configuration errors that have been visible only in logs.


## Configuration Errors

We define configuration errors as the errors that are created only due to user input and that
only user input changes make them go away. Versus infra errors (missing RBAC to watch a resource)
that are erros the admin/metallb deployment/code(bugs) method can fix.

### List of configuration errors

**Webhooks** are covering almost all errors but we can not rely because in corner cases (many
calls in parallel) could two conflicted CRs stored in the API.


//TODO



## Notes

* Only about the configuration errors, no other type of config error.
  Spefically do not cover the case where the applied frrk8sconfig from speaker gives error
* Aggregator of the status will not to be implemented, user will look into each one CR
* E2E test can work only with disabling the webhook
* QE/FT testing where disabling the webhook is not option, might not be able test the unhappy path
* API not provide the summary of the "input configuration", only the error
  which should clue which CR must be changed


## API

One CRD definitions, Multiple CRs, Namespaced, (maybe to configState)

```
$kubectl get configurationstate
NAME         TYPE        NODE   VALID   LASTERROR   AGE
controller   Controller         true                5m
speaker-n1   Speaker     n1     true                5m
...
```

```go
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

```


## On the Controller side

For Controller (Deployment - single instance), we only fetch status from PoolReconciler

    apiVersion: metallb.io/v1beta1
    kind: ConfigurationStatus
    metadata:
    name: controller
    namespace: metallb-system
    spec:
    type: Controller
    status:
    validConfig: true
    lastError: ""

    # Failed example
      apiVersion: metallb.io/v1beta1
      kind: ConfigurationState
      metadata:
        name: controller
        namespace: metallb-system
      spec:
        type: Controller
      status:
        validConfig: false
        lastError: "failed to parse configuration: CIDR \"192.168.10.100/32\" in pool \"client2-pool\" overlaps with already defined CIDR \"192.168.10.0/24\""


## On the Speaker side

For Speaker (DaemonSet - one pod per node) there are

ConfigReconciler - when config error, that error should appear in configState
NodeReconciler   -   >>
FRRK8sReconciler - ignore all errors
ServiceReconciler- ignore all errors
ServiceBGPStatusReconciler - no
Layer2StatusReconciler - no


configReconciler and NodeReconciler

    apiVersion: metallb.io/v1beta1
    kind: ConfigurationState
    metadata:
    name: speaker-kind-worker
    namespace: metallb-system
    spec:
    type: Speaker
    nodeName: kind-worker
    status:
    validConfig: true
    lastError: ""

    #Failed example:
    apiVersion: metallb.io/v1beta1
    kind: ConfigurationState
    metadata:
    name: speaker-kind-worker2
    namespace: metallb-system
    spec:
    type: Speaker
    nodeName: kind-worker2
    status:
    validConfig: false
    lastError: "failed to parse configuration: invalid BGP peer address"

## Implementation

Ideally like FRRNodeState pattern it metallb/frrk8s repo, but might not be
ideal because for ConfigurationStateReconciler needs references to ConfigReconciler
and NodeReconciler in-memory result. Alternative implementation based on
condition instead of channel can be evaluated. TBD and discussed during
the implementation PR.

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


### Config errors

  1. Webhook validates SUBSET of resources (validator.go:69-84)
  The webhook intentionally strips out fields that can cause TransientErrors:
  - Line 71: Peers[i].Spec.BFDProfile = "" - removes BFD profile references
  - Line 72: Peers[i].Spec.PasswordSecret = {} - removes password secret references
  - Lines 74-83: Removes community aliases (keeps only explicit values with :)

  2. Reconcilers validate FULL resources with ALL dependencies

  Specific Cases Where Webhook Passes but Reconciler Fails:

  A. BFD Profile Reference (config.go:288-291)
  - Webhook: Strips BFDProfile field → validation passes
  - Reconciler: Validates actual BFDProfile reference
    - Error: peer %s referencing non existing bfd profile %s (TransientError)
    - Happens when peer references a BFD profile that doesn't exist yet

  B. Password Secret Reference (config.go:456-459)
  - Webhook: Strips PasswordSecret field → validation passes
  - Reconciler: Tries to fetch actual secret from passwordSecrets map
    - Error: failed to parse peer %s password secret
    - Happens when secret doesn't exist or is malformed

  C. Community Aliases (config.go:364-379)
  - Webhook: Only validates explicit community values (with :)
  - Reconciler: Validates ALL communities including aliases
    - Error: parsing community %q: %s (line 370)
    - Error: duplicate definition of community %q (line 373)
    - Happens when community alias references don't exist

  D. Missing Resources in Reconciler Context
  - Webhook (bgppeer_webhook.go:108-114): Only fetches BGPPeers
  - ConfigReconciler: Fetches 9 resource types (Pools, Peers, BFDProfiles, L2Advs, BGPAdvs, Communities, Secrets, Nodes, Namespaces)
  - PoolReconciler (pool_controller.go:52-68): Fetches Pools, Communities, Namespaces

  Reconcilers can fail due to:
  - CIDR overlaps between pools (config.go:328-329)
  - Duplicate pool names (config.go:322)
  - Invalid namespace selectors (not checked by webhook)
  - Node label selector issues (not available in webhook)
  - Cross-resource dependencies

  E. Secrets Not Available to Webhook
  - Webhooks don't have access to PasswordSecrets map
  - Reconcilers list all secrets in namespace
  - Password validation only happens in reconciler


//TODO can happen in the configReconciler inside the handler `res := r.Handler(r.Logger, cfg)`
and  inside the nodecontroller in speaker and are configuration errors.


