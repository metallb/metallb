# ConfigurationStatus Design

## Overview

> As a cluster administrator, I want to know if the configuration applied
> is valid or not, and why it failed.

The ConfigurationStatus feature provides centralized status reporting for
MetalLB controllers using the standard Kubernetes conditions pattern. This
enables users to monitor the health and validity of their MetalLB configuration
through a single CRD.


## ConfigurationStatus CRD

ConfigurationStatus uses [Kubernetes Server-Side
Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) to
handle concurrent status updates from multiple controllers without conflicts.

A singleton resource named `config-status` is created in the MetalLB namespace.
It uses the standard Kubernetes `metav1.Condition` pattern to report status
from multiple controllers.

```yaml
apiVersion: metallb.io/v1beta1
kind: ConfigurationStatus
metadata:
  name: config-status
  namespace: metallb-system
status:
  conditions:
  - message: ""
    observedGeneration: 1
    reason: AllComponentsReady
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-10-10T09:13:47Z"
    message: ""
    reason: SyncStateSuccess
    status: "True"
    type: speaker-kind-worker/frrk8sReconcilerValid
  - lastTransitionTime: "2025-10-10T09:13:50Z"
    message: ""
    reason: SyncStateSuccess
    status: "True"
    type: speaker-kind-worker/configReconcilerValid
  - lastTransitionTime: "2025-10-10T09:13:50Z"
    message: ""
    reason: SyncStateSuccess
    status: "True"
    type: controller/poolReconcilerValid
```

Instead of having this as in the original design document

```go
    type MetalLBConfigurationStatus struct {
        validConfig bool
        lastError   string
    }
```

We have the **Ready** condition among the rest of conditions reported by others.

```go
	condition := metav1.Condition{
		Type:               "Ready",
		Status:              metav1.ConditionFalse,
		Reason:             "",
		Message:             " here goes the lastError",
	}
```


* Number of conditions is fixed. Controller has 3, and each speaker has 4
  (total number is 3 + 4 x #Nodes).

```
    type: Ready
    type: controller/poolReconcilerValid
    type: controller/serviceReconcilerValid
    type: speaker-kind-worker/serviceReconcilerValid
    type: speaker-kind-worker/nodeReconcilerValid
    type: speaker-kind-worker/configReconcilerValid
    type: speaker-kind-worker/frrk8sReconcilerValid
```

* Type "Ready" Condition instead of custom field was picked to comply with `kubectl wait --for=condition=Ready configurationstatus/config-status -n metallb-system`
* There is a unique controller that owns the CRD (creates it) and it responsible to decide based on the rest condition if the overal config is valid. This update the Ready condition
* Each controller can efficiently report each status with minimum RBAC (no conflicts due to ServerSideApply)
* Speaker pods use node-based condition naming to avoid stale conditions when pods restart, and to keep the number of condition fixed and to make debugging easier.
* Only becomes stale when node is removed (rare event)
* Can be extended to any other controller/Reconciler, even the webhook pod or the frr-k8s pods could report a condition and have a logic for that (e.g. webhook failed last time
but Ready condition is not false).

## User Integration

The aggregate Ready condition provides a simple way to check MetalLB's to check if there is no error:

```bash
kubectl get configurationstatus config-status -n metallb-system
# NAME            READY   REASON
# config-status   True    AllComponentsReady

# NAME            READY   REASON
# config-status   False   ComponentFailing (summary which components failed and how)
```

Secondary interaction

```bash
# Wait for MetalLB to be ready (common in automation/CI)
kubectl wait --for=condition=Ready configurationstatus/config-status -n metallb-system --timeout=60s

# Wait for Ready=False (useful in negative tests)
kubectl wait --for=condition=Ready=False configurationstatus/config-status -n metallb-system --timeout=10s

kubectl get configurationstatus config-status -n metallb-system -o yaml | grep -A 10 "controller/poolReconcilerValid"
# Shows the actual error message in the condition 

# 3. Fix the configuration and wait for recovery
kubectl apply -f fixed-config.yaml
kubectl wait --for=condition=Ready configurationstatus/config-status -n metallb-system --timeout=30s
```

### Condition Structure per Ready (User facing)

This is calculated centrally after any other condition changes:

- **Type**:  Ready ( to make kubectl wait works)
- **Status**: `True` (valid) or `False` (invalid)
- **Reason**:  join list of which condition failed
- **Message**: Error message  summary

### Condition Structure per Controller (Visible to user, but internal implementation)

Each controller reports its status as a separate condition:

- **Type**: Controller identifier with "Valid" suffix (e.g., "controller/poolReconcilerValid", "speaker-node1/configReconcilerValid")
- **Status**: `True` (valid) or `False` (invalid)
- **Reason**: Determined by the following hierarchy:
  1. If configuration error occurred: `ConfigError`
  2. Otherwise: The SyncState value
     - `SyncStateSuccess`: Configuration is valid and applied (Status=True)
     - `SyncStateReprocessAll`: Configuration applied, requires service reload (Status=True)
     - `SyncStateError`: Sync operation failed, transient error, will retry (Status=False)
     - `SyncStateErrorNoRetry`: Sync operation failed, non-transient error, no retry (Status=False)
- **Message**: Error message during reconcile, if any (empty if no error)

### Which Controller(Reconcilers) Reports

Any Reconciler can report each condition every time reconciler loop runs as long as it
knows the global config and has patch RBAC. This can be achieved through a
defer that should never fail. There is no watch on any ConfigStatus resource,
**only patches its own condition**. 

Even we can add all reconcilers (which should be ok), we must have

* Controller (Deployment - single instance)
    * PoolReconciler:  Condition: `controller/poolReconcilerValid`
    * ServiceReconciler:  Condition: `controller/serviceReconcilerValid`

* Speaker (DaemonSet - one pod per node)
    * ConfigReconciler: Condition: `speaker-<nodename>/configReconcilerValid`
    * ServiceReconciler: Condition: `speaker-<nodename>/serviceReconcilerValid`
    * NodeReconciler: Condition: `speaker-<nodename>/nodeReconcilerValid`
    * FRRK8sReconciler Condition: `speaker-<nodename>/frrk8sReconcilerValid`

#### Code Integration

Controllers integrate via defer pattern to ensure status is always reported:

**Example: PoolReconciler**
```go
type PoolReconciler struct {
    client.Client
    ...
	ConfigStatusRef types.NamespacedName
    NodeName       string  // Used to construct speaker-<node>/configReconciler ID
    ...
}

func (r *PoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
 ...
    var syncResult SyncState
    var syncError error

    // Defer status reporting - runs after reconcile completes
    defer func() {
        if err := r.reportCondition(ctx, syncError, syncResult); err != nil {
            level.Error(r.Logger).Log("controller", "PoolReconciler", "error", err)
        }
    }()

    // Reconcile logic...
    cfg, err := toConfig(resources, r.ValidateConfig)
    if err != nil {
        syncResult = SyncStateError
        syncError = err
        return ctrl.Result{}, nil
    }

    syncResult = r.Handler(r.Logger, cfg.Pools)
    return ctrl.Result{}, nil
}
```

### Server-Side Apply for Concurrent Updates

ConfigurationStatus uses [Kubernetes Server-Side
Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) to
handle concurrent status updates from multiple controllers without conflicts.

Without server-side apply, concurrent updates cause **resource version conflicts**:
```
Operation cannot be fulfilled on configurationstatuses.metallb.io "config-status":
the object has been modified; please apply your changes to the latest version and try again
```

Each controller becomes a unique **field manager** and owns its specific condition:
- `controller/poolReconciler` owns the `controller/poolReconcilerValid` condition
- `speaker-node1/configReconciler` owns the `speaker-node1/configReconcilerValid` condition
- `speaker-node2/configReconciler` owns the `speaker-node2/configReconcilerValid` condition

When a controller updates its condition:
1. Creates a partial ConfigurationStatus object with only its condition
2. Uses `client.Status().Patch()` with `client.Apply` patch type
3. Specifies field owner via `client.FieldOwner(owner)`
4. Kubernetes API server merges the condition into the existing conditions array
5. No conflicts - each field manager owns different fields


**Benefits:**
- No resource version conflicts
- Controllers can update concurrently
- Clear field ownership tracking
- Automatic conflict resolution
- Kubernetes-native approach

To see which field manager owns which fields:

```bash
kubectl get configurationstatus config-status -n metallb-system --show-managed-fields -o yaml
```
## Resource Lifecycle and Cleanup

The ConfigurationStatus resource is **created at runtime** by the ConfigurationStatusReconciler, not included in
installation manifests. This means:

- Created automatically when the unique controller configstatus starts
- Not deleted when running `kubectl delete -f metallb.yaml` or `helm uninstall metallb`
- Requires manual cleanup: `kubectl delete configurationstatus config-status -n metallb-system`
- Other components never fail if they cannot patch their condition
- Must review stale conditions, are they removed?

# FEEDBACK from Fede

https://github.com/metallb/metallb/pull/2856#issuecomment-3388777537

> I am not sure I like this approach, and this is not exactly what was drafted
> in the related design proposal.

We need to clarify the drafted proposal, questions on the other doc

> What the proposal said was either to cut corners making one single global
> configuration status managed by the controller (because the controller is
> responsible of the majority of the configuration) OR a per speaker
> configuration (which might make sense now with all the node selectors we have).

This proposal does have "single global config status" AND each speaker adds each status.

Q: how the "node selectors" are important, AFAIC if there speaker it posts its status,
if speaker stops existing condition becomes stale and removed (need some double checking
on that)

> With one singleton status, we may have a gigantic status, fed by all the
> controllers of all the speakers, becoming a lot of data to carry around and to
> redistribute to clients. A per node / per speaker instance is more scalable
> imo.

Status size is fixed (3 condition for the controller +
N times 3 conditions per speaker) and the user should only check the summary.
A solution which would list per node status and the user must "grep" the not valid
is less ideal

Q: What is "redistribute to clients"?

> Regarding having one condition per controller, what we are giving away here is
> the knowledge of the real error. A configuration is the result of many things:
> * the nodes
> * the configuration bits
> * possibly, the result of applying the frrk8s configuration

Q: what is "real" error example?

Q: do you mean by the configuration bits the configuration of  all `kubectl api-resources --api-group=metallb.io -o name`
AFAIC The validation if overall config is ok or not, it means if there is any
error somewhere or do you mean to actively check (e.g. vtysh show bgp config)?

> By hooking more or less blindly in each controller (even some that are
> unnecessary, he service controller and the bgp status ones are the first two I
> noticed), we are giving away the knowledge that the lower levels have, so the
> same error might float differently because a node event was processed before of
> a config event or viceversa.

'hooking blindly' is not "unnecessary", e.g if the poolreconciler loses rbac to
list something, that error will be shown, in all other case the condition will be ok.
It is for free to post condition.

Q: Can we define the "giving away the lower levels have? and "float differently"?

At the end what would be the description of an e2e test for that case you have in mind?

> All in all, we should consider if we prefer to stick to the structure of the
> other "status" controllers, which are waken by a signal and they go fetch the
> current status. This would allow us to retry whenever writing the new
> configurations status fails, and would have a single place where we write the
> configuration, instead of having multiple scattered writers.

Q: Can we define "fetch the current status"? How can controller that wakes up from signal find out the status? ask frrk8s?

Q: "When the configuration status fails", what is that in code? Reconciler loop runs?

"instead of having multiple scattered writers"  this sounds very bad, but is not
due to condition ServerSide condition patching.


Q: Channel approach is the following?

```
Having speaker code have a controller ConfigStatus which create
`config-status-speaker-on-node1`, and it wakes up on a channel event.
Other controller (only frrk8s?) when runs it always send an event.
Then the ConfigStatus will "fetch the current status"
//TODO on karampok check the type MetalLBServiceBGPStatus implementation
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
