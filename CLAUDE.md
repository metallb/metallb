# MetalLB Project Guide

## Project Overview
- Go project (check go.mod for version and toolchain)
- Load balancer implementation for bare metal Kubernetes clusters
- Uses Python invoke for task automation (prefix commands with `inv`, read tasks.py for available tasks)
- Testing framework: Ginkgo (tests are called "specs", not "tests")

## Key Components
- **Binaries**: controller, speaker
- **BGP modes**: native, frr, frr-k8s, frr-k8s-external
- **IP families**: ipv4, ipv6, dual
- **Deployment methods**: manifests, helm

## Common Tasks (via invoke)
```bash
inv test              # Run unit tests
inv lint              # Run linter
inv dev-env           # Build and run MetalLB in local Kind cluster
inv e2etest           # Run E2E tests
inv build             # Build docker images
inv generatemanifests # Regenerate manifests
```

## Development Environment

- Unit tests: standard Go tests with kubebuilder assets

```
inv test 
# how to run test in isolation
KUBEBUILDER_ASSETS=/home/kka/wkts/metallb/configstate/dev-env/unittest/bin/k8s/1.27.1-linux-amd64 go test ./internal/k8s/controllers -coverprofile=coverage.out -coverpkg=./internal/k8s/controllers -v -run=TestPoolController/handler_returns
```
- Uses Kind for local Kubernetes clusters e.g 

`inv dev-env-cleanup; inv dev-env -i ipv4 -b frr-k8s   -l all`

- E2E tests use Ginkgo with focus/skip patterns e.g.

`inv e2etest --bgp-mode frr --skip "IPV6|DUALSTACK|metrics|L2-interface selector|FRRK8S-MODE" --focus="Networkpolicies" -e /tmp/kind_logs --ginkgo-params "-v --dry-run"`

**Important** review .github/workflows/ci.yaml before answering
