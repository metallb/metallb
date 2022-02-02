# MetalLB test env

This scripts installs MetalLB into OCP, for test environment.

## Prerequisites

- Succeed to deploy OCP by dev-scripts (https://github.com/openshift-metal3/dev-scripts)
- IPv4 is enabled (Currently we targets IPv4. v4v6 case, we only support IPv4 for now)

## Quickstart

Make sure that OCP is deployed by dev-scripts.

To configure MetalLB clone this repo to dev-scripts and run:

```
$ cd <dev-scripts>/metallb/openshift-ci/
$ ./deploy_metallb.sh
```

### Check MetalLB pod status

```
$ export KUBECONIFG=<dev-scripts>/ocp/<cluster name>/auth/kubeconfig
$ oc get pod -n metallb-system
```

### Run E2E tests against development cluster

The test suite will run the appropriate tests against the cluster.

To run the E2E tests:

```
$ cd <dev-scripts>/metallb/openshift-ci/
$ ./run_e2e.sh
```
