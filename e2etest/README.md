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

# E2E Tests auto detection

**Auto-detection is enabled by default** to make testing easier for newcomers.

## Default Behavior: Auto-Detection with Required Protocol

Auto-detection **always requires** the `--protocol` parameter because kind clusters are typically unconfigured:

```bash
# Auto-detect environment and run tests automatically
inv e2etest --protocol bgp      # For BGP testing
inv e2etest --protocol layer2   # For Layer2 testing
```

This will:
- Detect your environment configuration (BGP type, IP family, Prometheus presence, etc.)
- Generate appropriate skip patterns from `test-filters.yaml`
- **Warn if overriding any manual parameters** (--skip, --bgp-mode)
- **Automatically run the tests** with the detected configuration

Example output:
```
Auto-detected environment: BGP Type: frr, IP Family: ipv4, Protocol: bgp, Prometheus: True
Auto-skip patterns: IPV6|DUALSTACK|L2

Running tests with auto-detected configuration...
[Test execution begins...]
```

## **Manual Mode**
To use manual parameters exclusively, disable auto-detection:

```bash
# Disable auto-detection and run with explicit parameters
inv e2etest --auto-detect-env=false --skip "IPV6|DUALSTACK" --bgp-mode frr
```

### **Recommendation**
- **Use auto-detection** for quick, environment-matched testing
- **Use manual mode** for specific test scenarios or CI environments

Running the E2E tests against the development cluster can be done in those ways. Examples:

Run only BGP test suite:

```
# Auto-detection approach
inv e2etest --protocol bgp --focus BGP

# Manual approach
inv e2etest --auto-detect-env=false --bgp-mode frr --focus BGP
```

Run only L2 test suite:

```
# Auto-detection approach
inv e2etest --protocol layer2 --focus L2

# Manual approach
inv e2etest --auto-detect-env=false --bgp-mode frr --focus L2
```

Run with additional ginkgo parameters:

```
# Auto-detection approach
inv e2etest --protocol bgp --ginkgo-params="--until-it-fails -v"

# Manual approach
inv e2etest --auto-detect-env=false --bgp-mode frr --ginkgo-params="--until-it-fails -v"
```

The test suite will run the appropriate tests against the cluster.
Be sure to cleanup any previously created development clusters using `inv dev-env-cleanup`.


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
