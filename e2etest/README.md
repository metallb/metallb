# E2E Tests

The MetalLB E2E test suite can be run by creating a development cluster:

```
inv dev-env
```

and running the E2E tests against the development cluster:

```
inv e2etest
```

Run only BGP test suite:

```
inv e2etest --focus BGP
```

Run only L2 test suite:

```
inv e2etest --focus L2
```

The test suite will run the appropriate tests against the cluster.
Be sure to cleanup any previously created development clusters using `inv dev-env-cleanup`.


## BGP tests network topology

In order to test the multiple scenarios required for BGP, three different containers are required.
The following diagram describes the networks, the pods and the containers involved.

```
  ┌────────────────────────────────────────────┐
  │                exec network                │
  │ ┌─────────┐                                │
  │ │         │                                │
  │ │ Speaker ├────┐      ┌─────────────────┐  │
  │ │         │    │      │                 │  │
  │ └─────────┘    │      │ ibgp-single-hop │  │
  │                │      │                 │  │
  │ ┌─────────┐    │      └─────────────────┘  │
  │ │         │    │                           │
  │ │ Speaker ├────┤   ┌───────────────────────┼──────────────────────────┐
  │ │         │    │   │                       │                          │
  │ └─────────┘    │   │  ┌─────────────────┐  │       ┌────────────────┐ │
  │                │   │  │                 │  │       │                │ │
  │ ┌─────────┐    ├───┼──┤ ebgp-single-hop ├──┼───────┤ ibgp-multi-hop │ │
  │ │         │    │   │  │                 │  │       │                │ │
  │ │ Speaker │    │   │  └─────────────────┘  │       └────────────────┘ │
  │ │         ├────┘   │                       │                          │
  │ └─────────┘        │                       │       ┌────────────────┐ │
  │                    │                       │       │                │ │
  └────────────────────┼───────────────────────┘       │ ebgp-multi-hop │ │
                       │                               │                │ │
                       │       multi-hop-net           └────────────────┘ │
                       │       172.30.0.0/16                              │
                       │  fc00:f853:ccd:e798::/64                         │
                       └──────────────────────────────────────────────────┘
```

The above diagram is implemented in `infra_setup.go`.
