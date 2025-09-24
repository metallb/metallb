# E2E Tests

This directory contains End-to-End tests for MetalLB.

## Test Organization

The tests are split between:
- **BGP tests** (`bgptests/`): Tests focused on BGP protocol functionality
- **Layer2 tests** (`l2tests/`): Tests focused on Layer2 protocol functionality
- **Network Policy tests** (`netpoltests/`): Tests focused on network policy enforcement
- **Webhook tests** (`webhookstests/`): Tests focused on validation and mutation webhooks
- **Common functionality** (`pkg/`): Shared utilities and helper functions

## Running Tests

### Prerequisites

1. **Development environment**: Set up a MetalLB development environment:
   ```bash
   inv dev-env --bgp-type frr --ip-family ipv4 --protocol bgp
   ```

2. **Required dependencies**: The test suite requires several utilities installed on the host system. These are automatically installed in CI environments.

### Test Execution

The recommended approach is a **two-step process**:

#### Step 1: Auto-Detect Environment and Get Recommendations

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

#### Step 2: Run Tests with Recommended Parameters

Copy and run the recommended command from Step 1:

```bash
# Run the recommended command (this actually runs the tests)
inv e2etest --skip "IPV6|DUALSTACK|L2" --bgp-mode frr
```

#### Alternative: Manual Configuration

If you prefer to skip auto-detection and configure manually:

```bash
# Direct manual configuration
inv e2etest --bgp-mode frr --focus "BGP"

# See all available options
inv e2etest --help
```

**Important Notes:**
- `--auto-detect-env` is a **discovery tool** - it shows you what to run but doesn't run tests itself
- The two-step approach ensures you understand what tests will be skipped before running

This matches the actual implementation behavior and guides users through the intended workflow.

### Test Filtering System

E2E tests use an intelligent filtering system to skip irrelevant tests based on your environment configuration. This system is defined in:

- **`test-filters.yaml`**: Central configuration defining skip patterns for different environments
- **`tasks.py`**: Contains filtering functions that process the YAML configuration

#### Configuration-Based Skipping

The filtering system skips tests based on:

1. **BGP Type** (`native`, `frr`, `frr-k8s`, `frr-k8s-external`):
   - Native BGP skips FRR-specific tests
   - FRR skips FRR-K8s-specific tests
   - etc.

2. **IP Family** (`ipv4`, `ipv6`, `dual`):
   - IPv4 environments skip IPv6-only tests
   - IPv6 environments skip IPv4-only tests
   - Dual-stack has its own skip patterns

3. **Protocol** (`bgp`, `layer2`):
   - BGP environments skip Layer2-only tests
   - Layer2 environments skip BGP-only tests

4. **Prometheus presence**: Tests requiring metrics are skipped when Prometheus is not available

Example configuration snippet from `test-filters.yaml`:
```yaml
bgp_type:
  native:
    skip: ["FRR", "FRR-MODE", "FRRK8S-MODE", "BFD", "VRF", "DUALSTACK"]
ip_family:
  ipv4:
    skip: ["IPV6", "DUALSTACK"]
```

#### CI Integration

The filtering system is integrated into CI workflows:

```yaml
- name: Run E2E Tests
  run: |
    SKIP_PATTERNS=$(inv generate-test-skip-patterns "${{ matrix.bgp-type }}" "${{ matrix.ip-family }}")
    sudo -E env "PATH=$PATH" inv e2etest --skip "$SKIP_PATTERNS" --bgp-mode ${{ matrix.bgp-type }}
```

### Components of the Filtering System

1. **`test-filters.yaml`**: Central configuration file defining all skip rules
2. **`tasks.py`**: Contains filtering functions (`load_test_filters`, `generate_skip_patterns`, `generate_skip_string`)

## Automatic Environment Detection

E2E tests can automatically detect your development environment configuration and provide recommended test commands. This means you no longer need to remember complex skip patterns for most scenarios.

### Auto-Detection Usage

Use auto-detection to see what configuration is detected and get a recommended command:

```bash
# Automatically detect environment and show recommended command
inv e2etest --auto-detect-env
```

Auto-detection will:
- Detect BGP type (native, frr, frr-k8s, frr-k8s-external)
- Detect IP family (ipv4, ipv6, dual)
- Detect protocol (BGP vs Layer2)
- Detect Prometheus availability
- Load `test-filters.yaml` and generate appropriate skip patterns
- Display the recommended test execution command
- Exit without running tests

### Manual Control

When you need full control over test selection:

```bash
# Manual configuration with specific parameters
inv e2etest --bgp-mode frr --focus "BGP"

# Using custom skip patterns (note the required quotes)
inv e2etest --skip "MyCustomPattern|AnotherPattern"
```

## Manual Test Selection

Run specific test suites:

```bash
# Run only BGP test suite
inv e2etest --focus "BGP"

# Run only L2 test suite
inv e2etest --focus "L2"

# Run with additional ginkgo parameters
inv e2etest --ginkgo-params="--until-it-fails -v"

# Skip specific patterns (always use quotes for pipe-separated values)
inv e2etest --skip "IPV6|FRRK8S-MODE|metrics|BGP"
```

**Important**: Always quote skip patterns that contain pipe characters (`|`) to prevent shell interpretation issues.

## Maintaining Test Filters

When adding new test patterns or changing environment configurations:

1. **Update `test-filters.yaml`**: Add new skip patterns or modify existing ones
2. **Test locally**: Use `inv e2etest --auto-detect-env` to verify your changes
3. **Test in CI**: The filtering system automatically applies to CI matrix runs

### Adding New Skip Patterns

To add skip patterns for a new test category:

1. **Identify the pattern**: Look at test names that should be skipped
2. **Update `test-filters.yaml`**: Add the pattern to appropriate sections
3. **Test the change**: Run tests with different configurations to verify

Example of adding a new special case:
```yaml
special_cases:
  - condition:
      bgp_type: "native"
      ip_family: "ipv6" 
    skip: ["BGP"]  # Native BGP doesn't support IPv6
```

## Test Development Guidelines

When writing new tests:

1. **Use descriptive test names** that clearly indicate what environment they target
2. **Tag tests appropriately** using patterns that match the filtering system
3. **Consider multi-environment compatibility** - write tests that work across configurations when possible
4. **Update `test-filters.yaml`** if adding environment-specific tests

Common test naming patterns:
- `BGP` prefix for BGP-specific tests
- `L2` prefix for Layer2-specific tests  
- `IPV4`, `IPV6`, `DUALSTACK` for IP family specific tests
- `FRR-MODE`, `FRRK8S-MODE` for implementation-specific tests
- `metrics` for Prometheus-dependent tests

## Troubleshooting

### Common Issues

1. **Shell parsing errors when using skip patterns**:
   - **Problem**: `bash: FRRK8S-MODE: command not found`
   - **Solution**: Always quote skip patterns: `--skip "IPV6|FRRK8S-MODE|metrics"`

2. **Tests failing due to environment mismatch**: 
   - Check detected configuration: `inv e2etest --auto-detect-env`
   - Verify your dev-env matches expected configuration

3. **Skip patterns not working**:
   - Validate `test-filters.yaml` syntax
   - Check that test names match skip patterns exactly

### Debugging Environment Detection

See what environment configuration is detected:

```bash
# Show detected configuration and recommended command
inv e2etest --auto-detect-env

# Run specific configuration manually
inv e2etest --bgp-mode frr --skip "IPV6|L2"
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
