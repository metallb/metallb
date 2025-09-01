# MetalLB GitHub Issue #2203: Automated Test Focus/Skip Implementation Guide

This document provides a comprehensive analysis and implementation guide for [GitHub issue #2203](https://github.com/metallb/metallb/issues/2203) - implementing automated test focus/skip functionality for MetalLB's e2e tests.

## Table of Contents
- [Understanding the Issue](#understanding-the-issue)
- [Current Problem](#current-problem)
- [Test Organization Structure](#test-organization-structure)
- [Proposed Solution](#proposed-solution)
- [Implementation Approach](#implementation-approach)
- [Example Usage](#example-usage)
- [Implementation Steps](#implementation-steps)
- [Key Considerations](#key-considerations)

## Understanding the Issue

### Current Problem
When developers run e2e tests locally with `inv e2etest`, **ALL tests run** regardless of their development environment configuration. This causes:

1. **Unnecessary test execution**: Tests for features not configured in the environment
2. **Longer feedback cycles**: Waiting for irrelevant tests to complete
3. **Confusing failures**: Tests failing because features aren't configured

### Example of Current Behavior
```bash
# Developer sets up BGP environment with IPv4 only and FRR mode
inv dev-env --protocol bgp --ip-family ipv4 --bgp-type frr

# But running tests executes ALL tests including irrelevant ones:
inv e2etest --bgp-mode frr
# ^ Runs IPv6 tests, Layer2 tests, FRRK8S-MODE tests, BFD tests, etc.
```

### What the CI Does (That We Want to Automate)
Looking at the CI configuration, there's already sophisticated logic that determines what to skip based on environment configuration:

```bash
# From .github/workflows/ci.yaml
SKIP="none"
if [ "${{ matrix.bgp-type }}" == "native" ]; then 
    SKIP="$SKIP|FRR|FRR-MODE|FRRK8S-MODE|BFD|VRF|DUALSTACK"; 
fi
if [ "${{ matrix.ip-family }}" == "ipv4" ]; then 
    SKIP="$SKIP|IPV6|DUALSTACK"; 
fi
if [ "${{ matrix.bgp-type }}" == "frr-k8s" ]; then 
    SKIP="$SKIP|FRR-MODE"; 
fi
# ... etc
```

## Test Organization Structure

The e2e tests use specific naming patterns that correspond to these skip patterns:

**Test Categories Found:**
- `FRR-MODE` - Tests specific to FRR BGP implementation
- `FRRK8S-MODE` - Tests specific to FRR-K8s BGP implementation  
- `IPV4` / `IPV6` / `DUALSTACK` - IP family specific tests
- `BFD` - BFD (Bidirectional Forwarding Detection) tests
- `VRF` - VRF (Virtual Routing and Forwarding) tests
- `L2` - Layer2 tests
- `BGP` - BGP-related tests
- `metrics` - Prometheus metrics tests

## Proposed Solution & Implementation Approach

### 1. Add New Parameter to `e2etest` Function

Add an `--auto-focus` parameter that automatically determines focus/skip patterns:

```python
@task(help={
    # ... existing parameters ...
    "auto_focus": "Automatically determine test focus/skip based on current dev-env configuration. Default: False",
})
def e2etest(
    ctx,
    # ... existing parameters ...
    auto_focus=False,
):
```

### 2. Create Environment Detection Logic

Add a function to detect the current environment configuration:

```python
def detect_dev_env_config(cluster_name="kind"):
    """Detect the configuration of the current dev environment."""
    config = {
        'bgp_type': 'unknown',
        'ip_family': 'unknown', 
        'with_prometheus': False,
        'protocol': 'unknown'
    }
    
    try:
        # Check what's deployed in metallb-system namespace
        result = run(f"{kubectl_path} get deployment -n metallb-system -o yaml", hide=True)
        
        # Parse deployment to detect BGP type
        if "frr-k8s" in result.stdout:
            config['bgp_type'] = 'frr-k8s'
        elif "frr" in result.stdout:
            config['bgp_type'] = 'frr'  
        else:
            config['bgp_type'] = 'native'
            
        # Check for prometheus namespace
        prom_result = run(f"{kubectl_path} get namespace monitoring", hide=True, warn=True)
        config['with_prometheus'] = prom_result.ok
        
        # Detect IP family from service CIDRs
        cidr_result = run(f"{kubectl_path} get ipaddresspool -n metallb-system -o yaml", hide=True, warn=True)
        if cidr_result.ok:
            if "fc00:" in cidr_result.stdout and "192.168" in cidr_result.stdout:
                config['ip_family'] = 'dual'
            elif "fc00:" in cidr_result.stdout:
                config['ip_family'] = 'ipv6'
            else:
                config['ip_family'] = 'ipv4'
                
        # Detect protocol (BGP vs Layer2)
        bgppeer_result = run(f"{kubectl_path} get bgppeer -n metallb-system", hide=True, warn=True)
        l2adv_result = run(f"{kubectl_path} get l2advertisement -n metallb-system", hide=True, warn=True) 
        
        if bgppeer_result.ok and "No resources found" not in bgppeer_result.stdout:
            config['protocol'] = 'bgp'
        elif l2adv_result.ok and "No resources found" not in l2adv_result.stdout:
            config['protocol'] = 'layer2'
            
    except Exception as e:
        print(f"Warning: Could not auto-detect environment config: {e}")
        
    return config
```

### 3. Create Focus/Skip Logic

```python
def generate_test_filters(config):
    """Generate ginkgo focus/skip patterns based on environment config."""
    skip_patterns = []
    
    # BGP type filtering
    if config['bgp_type'] == 'native':
        skip_patterns.extend(['FRR', 'FRR-MODE', 'FRRK8S-MODE', 'BFD', 'VRF'])
    elif config['bgp_type'] == 'frr':
        skip_patterns.append('FRRK8S-MODE')
    elif config['bgp_type'] in ['frr-k8s', 'frr-k8s-external']:
        skip_patterns.append('FRR-MODE')
        
    # IP family filtering  
    if config['ip_family'] == 'ipv4':
        skip_patterns.extend(['IPV6', 'DUALSTACK'])
    elif config['ip_family'] == 'ipv6':
        skip_patterns.extend(['IPV4', 'DUALSTACK'])
        if config['bgp_type'] == 'native':
            skip_patterns.append('BGP')  # Native BGP doesn't support IPv6
    elif config['ip_family'] == 'dual':
        skip_patterns.append('IPV6')  # Skip IPv6-only tests in dual stack
        
    # Protocol filtering
    if config['protocol'] == 'layer2':
        skip_patterns.append('BGP')
    elif config['protocol'] == 'bgp':
        skip_patterns.append('L2')
        
    # Prometheus filtering
    if not config['with_prometheus']:
        skip_patterns.append('metrics')
        
    return {
        'skip': '|'.join(skip_patterns) if skip_patterns else '',
        'focus': ''  # Could add focus logic later
    }
```

### 4. Integration into e2etest Function

```python
def e2etest(ctx, ..., auto_focus=False):
    """Run E2E tests against development cluster."""
    
    # ... existing setup code ...
    
    if auto_focus:
        print("🔍 Auto-detecting development environment configuration...")
        env_config = detect_dev_env_config(name)
        filters = generate_test_filters(env_config)
        
        print(f"📊 Detected configuration:")
        print(f"  - BGP Type: {env_config['bgp_type']}")
        print(f"  - IP Family: {env_config['ip_family']}")
        print(f"  - Protocol: {env_config['protocol']}")
        print(f"  - Prometheus: {env_config['with_prometheus']}")
        
        if filters['skip']:
            print(f"⏭️  Auto-skip patterns: {filters['skip']}")
            if not skip:  # Don't override manually provided skip
                skip = filters['skip']
        else:
            print("✅ No tests will be skipped")
    
    # ... rest of existing e2etest logic ...
```

## Example Usage After Implementation

### Development Workflow Examples

```bash
# 1. Set up BGP development environment
inv dev-env --protocol bgp --bgp-type frr --ip-family ipv4

# 2. Run tests with auto-focus (NEW!)
inv e2etest --auto-focus --bgp-mode frr
# Output:
# 🔍 Auto-detecting development environment configuration...
# 📊 Detected configuration:
#   - BGP Type: frr
#   - IP Family: ipv4  
#   - Protocol: bgp
#   - Prometheus: False
# ⏭️  Auto-skip patterns: FRRK8S-MODE|IPV6|DUALSTACK|L2|metrics

# 3. Still allow manual override for advanced users
inv e2etest --auto-focus --skip "BGP|metrics" --focus "L2"
```

### Different Environment Examples

```bash
# Layer2 + IPv6 environment
inv dev-env --protocol layer2 --ip-family ipv6
inv e2etest --auto-focus
# Would skip: BGP|IPV4|DUALSTACK|metrics

# FRR-K8s + Dual stack + Prometheus
inv dev-env --protocol bgp --bgp-type frr-k8s --ip-family dual --with-prometheus
inv e2etest --auto-focus  
# Would skip: FRR-MODE|IPV6 (keeps BGP, metrics, DUALSTACK tests)
```

## Implementation Steps & Workflow

### Phase 1: Basic Implementation
1. **Add the detection functions** to `tasks.py`
2. **Add `auto_focus` parameter** to `e2etest` task
3. **Test with simple scenarios** (BGP vs Layer2)

### Phase 2: Enhanced Detection  
1. **Improve environment detection** robustness
2. **Add support for more configuration options** (VRF, external containers)
3. **Add validation** to ensure detected config matches reality

### Phase 3: Documentation & Polish
1. **Update README and documentation** to promote this as the default
2. **Add helpful output** showing what tests will run
3. **Add `--dry-run` option** to show what would be skipped without running

### Testing Your Implementation

```bash
# Test detection logic
inv dev-env --protocol bgp --bgp-type frr --ip-family ipv4
# Manually verify what's deployed
kubectl get deployment,ipaddresspool,bgppeer -n metallb-system

# Test auto-focus
inv e2etest --auto-focus --dry-run  # (if you implement dry-run)

# Compare with manual approach
inv e2etest --skip "FRRK8S-MODE|IPV6|DUALSTACK" --bgp-mode frr
inv e2etest --auto-focus --bgp-mode frr  # Should be equivalent
```

## Key Considerations

1. **Opt-in Design**: Must be explicitly enabled with `--auto-focus`
2. **CI Safety**: CI continues using hardcoded patterns to avoid "volkswagen tests"
3. **Fallback Behavior**: If detection fails, proceed with current behavior  
4. **User Feedback**: Clear output showing what was detected and what will be skipped
5. **Override Capability**: Manual focus/skip should still work and take precedence

### Benefits of This Implementation

1. **Improved Developer Experience**: New contributors get relevant test feedback faster
2. **Faster Iteration**: Developers spend less time waiting for irrelevant tests
3. **Better Focus**: Developers can focus on tests relevant to their changes
4. **Maintained Flexibility**: Advanced users can still use manual focus/skip
5. **CI Compatibility**: Doesn't affect existing CI workflows

### Example Complete Workflow

```bash
# Developer workflow with auto-focus
inv dev-env --protocol bgp --bgp-type frr --ip-family ipv4
inv e2etest --auto-focus --bgp-mode frr

# What gets executed:
# ✅ BGP tests (FRR-MODE)
# ✅ IPv4 tests  
# ✅ BGP-specific tests
# ❌ FRRK8S-MODE tests (skipped)
# ❌ IPv6 tests (skipped)
# ❌ Layer2 tests (skipped)
# ❌ Metrics tests (skipped - no Prometheus)

# Result: ~50% fewer tests, 2x faster feedback
```

This implementation would significantly improve the developer experience for newcomers while maintaining the reliability and explicitness that the CI requires. The auto-detection logic mirrors what's already proven to work in CI, making it a low-risk, high-value addition to the MetalLB development workflow.
