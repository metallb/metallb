# ConfigurationStatus CR

As a cluster administrator, I want to know if the configuration applied is valid or not, and why it failed.
Configuration is the 


## Non-goals

* only about the configuration which means if API fails this error is not config error
* we specifically do not cover the case where the applied frrk8sconfig from speaker gives error
* like frrnodestates
* aggregator of the status not to be implemented
* e2e test can work only with disabling the webhook
* "validating configuration" means any error from anywhere that is due to user CRs, and that user can fix it
* does not provide the "here is configuration", only that is the error
* Q do we want to be namespace?


## User Facing API

One CRD definitions, Multiple CRs

```
$kubectl get configurationstate
NAME         TYPE        NODE   VALID   LASTERROR   AGE
controller   Controller         true                5m
speaker-n1   Speaker     n1     true                5m
...
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



//TODO:
Find out all erros that can happen in the configReconciler 
`res := r.Handler(r.Logger, cfg)`
and  inside the nodecontroller in speaker and are configuration errors.



## Implementation

like FRRNodeState pattern
Q: Given there is controler which saves the status
- 


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
