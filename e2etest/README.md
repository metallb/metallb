# E2E Tests

To run the MetalLB E2E test suite, you first need to create a development cluster:

```
inv dev-env
```

The above command will create a cluster with backend bgp `frr`. To deploy development cluster
with other types of bgp backend, you specify that with `--bgp-type` parameter. For example:

```
inv dev-env --bgp-type native
```

Running the E2E tests against the development cluster requires mandatory field `bgp-mode`, which needs
to match the backend bgp the dev-env was created with.

```
inv e2etest --bgp-mode frr
```

Run only BGP test suite:

```
inv e2etest --bgp-mode frr --focus BGP
```

Run only L2 test suite:

```
inv e2etest --bgp-mode frr --focus L2
```

Run with additional ginkgo parameters for example:

```
inv e2etest --bgp-mode frr --ginkgo-params="--until-it-fails -v"
```

The test suite will run the appropriate tests against the cluster.
Be sure to cleanup any previously created development clusters using `inv dev-env-cleanup`.


# E2E Tests auto detection

The recommended E2E tests execution is a **two-step process**:

## Step 1: Auto-Detect Environment and Get Recommendations

First, use auto-detection to see what configuration is detected and get the recommended test command:

```bash
# Auto-detect environment and show recommended command (does NOT run tests)
inv e2etest --auto-detect-env
```

This will:
- Detect your environment configuration (BGP type, IP family, protocol, etc.)
- Generate appropriate skip patterns from `test-filters.yaml`
- Display the recommended test execution command
- **Exit without running any tests**

Example output:
```
📊 Auto-detected environment: BGP Type: frr, IP Family: ipv4, Protocol: bgp, Prometheus: True
⏭️  Auto-skip patterns: IPV6|DUALSTACK|L2

The recommended test execution command is:
$ inv e2etest --skip "IPV6|DUALSTACK|L2" --bgp-mode frr

Exiting without running tests.
```

## Step 2: Run Tests with Recommended Parameters

Copy and run the recommended command from Step 1:

```bash
# Run the recommended command (this actually runs the tests)
inv e2etest --skip "IPV6|DUALSTACK|L2" --bgp-mode frr
```

# E2E Tests auto detection in CI

This auto detection mechanism has been integrated in CI to generate appropriate skip patterns:

```yaml
- name: Run E2E Tests
  run: |
    SKIP_PATTERNS=$(inv generate-ci-skip-patterns "${{ matrix.bgp-type }}" "${{ matrix.ip-family }}")
    sudo -E env "PATH=$PATH" inv e2etest --skip "$SKIP_PATTERNS" --bgp-mode ${{ matrix.bgp-type }}
```

## BGP tests network topology

In order to test the multiple scenarios required for BGP, three different containers are required.
The following diagram describes the network connectivity using the regular kind network, the pods
and the containers involved.

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

The same layout is replicated to validate external routers reacheable via a VRF.

So, on top of what's described above, there is another set of 4 containers meant
to be reached using an interface hosted inside a vrf created on the node:

```none
                                                                                           ┌─────────────────────┐
                                                                                           │                     │
                                                                                           │                     │
                                                                                        ┌──┤  ibgp-vrf-multi-hop │
                                           ┌──────────────────────┐                     │  │                     │
                                           │                      │                     │  │                     │
                                           │                      │                     │  └─────────────────────┘
┌───────────────────────┐               ┌──┤  ebgp-vrf-single-hop ├─────────────────────┤
│                       │               │  │                      │                     │  ┌─────────────────────┐
│                       │               │  │                      │   vrf-multihop-net  │  │                     │
│   ┌─────────┐         │               │  └──────────────────────┘                     │  │                     │
│   │         │  ┌──────┤               │                                               └──┤  ebgp-vrf-multi-hop │
│   │ Speaker │  │      │               │                                                  │                     │
│   │         │  │   ───┼───────────────┤                                                  │                     │
│   └─────────┘  │   VRF│  vrf-net      │  ┌──────────────────────┐                        └─────────────────────┘
│                │      │               │  │                      │
│                └──────┤               │  │                      │
│                       │               └──┤  ibgp-vrf-single-hop │
│                  Node │                  │                      │
│                       │                  │                      │
└───────────────────────┘                  └──────────────────────┘

```


The above diagram is implemented in `infra_setup.go`.

## Use Existing Containers

The E2E tests can run while using existing FRR containers that act as the single/multi-hop BGP routers.

To do so, pass the flag `external-containers` with the value of a comma-separated list of containers names.
The valid names are: `ibgp-single-hop` / `ibgp-multi-hop` / `ebgp-single-hop` / `ebgp-multi-hop`.

The test setup will use them instead of creating the external frr containers on its own.

The requirements for this are:
- The external FRR containers must be named as `ibgp-single-hop` / `ibgp-multi-hop` / `ebgp-single-hop` / `ebgp-multi-hop`.
- When running with an multi-hop container, the `ebgp-single-hop` container must be present.
Other than that, the multi-hop containers must be connected to a docker network named
`multi-hop-net`. The test suite will take care of connecting the `ebgp-single-hop` to the multi-hop-net and creating the required static routes between the speakers and the containers, as well as configuring the external FRR containers.
- When using existing container, i.e. `ibgp_single_hop`, you have to mount a directory named `ibgp-single-hop`
that has the initial frr configurations files (vtysh.conf, zebra.conf, daemons, bgpd.conf, bfdd.conf).
See `metallb/e2etest/config/frr` for example.
Note that the test's teardown will delete the directory, so it's recommended to keep a copy of the directory in a different place.
