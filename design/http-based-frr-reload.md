# HTTP-Based FRR Reload and Debouncer Improvements

## Summary

This proposal aims to improve MetalLB's BGP advertisement convergence time by replacing the timer-based FRR configuration reload mechanism with an HTTP-based approach, and eliminating fixed debounce delays in favor of immediate config application with intelligent batching.

## Motivation

Currently, when MetalLB is deployed in FRR mode, there is a consistent 3-4 second delay between a Service LoadBalancer IP being assigned (with endpoints becoming Ready) and the IP being advertised to BGP peers. This delay is caused by the design of the FRR configuration reload mechanism, which uses a hardcoded 3-second debounce timeout to batch multiple configuration changes.

While this debouncing mechanism was introduced to prevent excessive FRR reloads that could cause memory accumulation and performance issues, the fixed 3-second delay impacts critical use cases such as:

1. **Service startup convergence**: New services experience unnecessary delays before being accessible via BGP
2. **Pod deletion scenarios**: When the last endpoint on a node is deleted, BGP withdrawal messages are delayed by 3+ seconds, causing traffic blackholing
3. **Production deployments requiring fast failover**: Applications that need sub-second convergence times for high availability

### Historical Context

The debouncer was originally introduced in commit `88a938e0` with a 500ms timeout to:
- Batch multiple configuration changes that occur in rapid succession
- Reduce the number of FRR reloads triggered by a single MetalLB configuration change

The timeout was later increased to 3 seconds in commit `1de7f14d` because:
- FRR reload operations typically take 2-3 seconds to complete
- Multiple reload requests in flight would accumulate, creating pending SIGHUP signals
- Each pending reload spawned a `frr_reloader` child process, consuming memory
- Fast service updates could cause memory accumulation faster than reloads could be processed

### Current Issues

The current timer-based debouncer has several limitations:

1. **Fixed delay regardless of reload duration**: The 3-second timer starts when the first config change arrives, not when the previous reload completes
2. **PID-based signaling overhead**: Using SIGHUP signals and polling status files adds complexity and potential race conditions
3. **No feedback loop**: The debouncer doesn't know when a reload completes, leading to conservative timeout values
4. **Delayed convergence**: All configuration changes, even when FRR is idle, wait the full debounce period

### Goals

* Eliminate unnecessary fixed delays in the configuration reload path
* Provide immediate feedback when FRR reloads complete successfully
* Maintain protection against reload request accumulation
* Improve convergence times for service advertisements and withdrawals
* Allow future configurability of reload behavior without compromising safety

### Non-Goals

* Making the debounce timeout user-configurable (this would require deep understanding of internals)
* Changing the FRR reload mechanism itself (frr-reload.py)
* Providing different reload strategies for different types of configuration changes
* Supporting legacy FRR implementations

## Proposal

### User Stories

#### Story 1

As a cluster administrator deploying latency-sensitive services, I want my LoadBalancer IPs to be advertised to BGP peers within 1 second of endpoints becoming ready, so that service startup time is minimized.

#### Story 2

As a cluster administrator managing rolling updates, I want BGP withdrawals to occur immediately when the last endpoint on a node is deleted, to prevent traffic blackholing during pod migrations.

#### Story 3

As a platform engineer, I want the FRR reload mechanism to be efficient and predictable, batching configuration changes intelligently without introducing artificial delays when the system is idle.

## Design Details

### Architecture Overview

The new design consists of two major components:

1. **HTTP-based FRR Reloader**: A Go-based daemon that replaces the shell script wrapper around `frr-reload.py`
2. **Event-driven Debouncer**: A new debouncer implementation that eliminates fixed timers and provides immediate feedback

### Component 1: HTTP-Based FRR Reloader

#### Current Implementation (Shell Script + SIGHUP)

The current implementation:
- Runs a shell script that waits for SIGHUP signals
- Uses PID file-based communication
- Writes status to a file that is polled periodically
- No direct feedback to the caller

#### New Implementation (HTTP Server + Unix Socket)

The new `frr-reloader` daemon:
- Runs as a long-lived Go process
- Listens on a Unix domain socket for HTTP requests
- Accepts JSON-encoded reload requests with unique IDs
- Executes `frr-reload.py` and returns success/failure status
- Uses `flock` for mutual exclusion (same as shell script)
- Provides synchronous feedback to callers

**API Contract:**

Request:
```json
{
  "action": "reload",
  "id": 123
}
```

Response:
```json
{
  "id": 123,
  "result": true
}
```

**Key Features:**

1. **Synchronous operation**: The HTTP request blocks until the reload completes
2. **Unique request IDs**: Prevents response mismatches using atomic counters
3. **Unix socket transport**: Secure, local-only communication
4. **Backward compatible**: Still supports SIGHUP signals for manual triggering
5. **Structured logging**: Maintains current logging behavior

#### Implementation Details

**File: `frr-tools/reloader/frr-reloader.go`**

```go
// Key structures
type reloadRequest struct {
    Action string `json:"action"`
    ID     int    `json:"id"`
}

type reloadResponse struct {
    ID     int  `json:"id"`
    Result bool `json:"result"`
}

// HTTP handler accepts requests and channels them to reload processor
func (r *reloader) handleReload(w http.ResponseWriter, req *http.Request) {
    // Decode request, send to channel, wait for response
    r.reloadReqChan <- reloadReq
    resp := <-r.reloadRespChan
    json.NewEncoder(w).Encode(resp)
}

// Main reload function uses flock for serialization
func (r *reloader) reloadFRR() bool {
    syscall.Flock(int(r.lockFd.Fd()), syscall.LOCK_EX)
    defer syscall.Flock(int(r.lockFd.Fd()), syscall.LOCK_UN)

    // Test config syntax
    r.runFRRReload("--test", startTime)
    // Apply config
    r.runFRRReload("--reload --overwrite", startTime)

    return success
}
```

**Environment Variables:**

- `SHARED_VOLUME`: Directory for socket and status files (default: `/etc/frr_reloader`)
- `FRR_RELOADER_SOCKET`: Override socket path (default: `$SHARED_VOLUME/frr-reloader.sock`)

### Component 2: Event-Driven Debouncer

#### Current Implementation (Timer-Based)

The current debouncer:
- Uses `time.After()` to create a 3-second delay
- Starts the timer on first config change
- Batches all changes arriving within the 3-second window
- No awareness of when reloads complete

**Pseudocode:**
```go
timer := nil
for {
    select {
    case config := <-reload:
        storeConfig(config)
        if timer == nil {
            timer = time.After(3 * time.Second)
        }
    case <-timer:
        reloadFRR(config)
        timer = nil
    }
}
```

#### New Implementation (Event-Driven)

The new debouncer:
- Uses atomic pointers for thread-safe config storage
- Employs a non-blocking trigger channel
- Separates config reception from reload execution
- Automatically retries on failure

**Architecture:**

```
Config Changes     Debouncer           Reload Worker
    (API)         (Main Loop)         (Goroutine)
       |               |                    |
   [config] --------> store config         |
       |               |                    |
       |           trigger reload --------> |
       |               |              <wait for trigger>
       |               |                    |
       |               |              load latest config
       |               |                    |
       |               |              execute reload
       |               |                    |
       |               |              <---- success/fail
       |               |                    |
   [config] --------> store config         |
       |               |                    |
       |           (no trigger - pending)  |
       |               |                    |
       |               |              reload complete
       |               |                    |
       |           trigger reload --------> |
```

**Key Characteristics:**

1. **Store-then-trigger ordering**: Ensures reload worker always sees latest config
2. **Non-blocking triggers**: If a trigger is pending, new configs update storage but don't send duplicate triggers
3. **Automatic batching**: Configs arriving during an active reload are automatically batched
4. **Retry mechanism**: Failed reloads sleep and then trigger retry with latest config
5. **Zero artificial delays**: Reloads start immediately when the system is idle

#### Implementation Details

**File: `internal/bgp/frr/config.go`**

```go
func debouncer(body func(config *frrConfig) error,
    reload <-chan reloadEvent,
    _ time.Duration,  // Ignored - kept for signature compatibility
    failureRetryInterval time.Duration,
    l log.Logger) {

    var configToUse atomic.Pointer[frrConfig]
    triggerReload := make(chan struct{}, 1)

    // Reload worker goroutine
    go func() {
        for {
            <-triggerReload  // Block until triggered
            cfg := configToUse.Load()  // Atomically load latest config
            err := body(cfg)  // Execute reload (blocks until complete)
            if err != nil {
                time.Sleep(failureRetryInterval)
                // Trigger retry if not already pending
                select {
                case triggerReload <- struct{}{}:
                default:
                }
            }
        }
    }()

    // Main debouncer loop
    for {
        select {
        case newCfg := <-reload:
            if newCfg.useOld && configToUse.Load() == nil {
                continue  // Ignore retry with no config
            }
            if !newCfg.useOld {
                configToUse.Store(newCfg.config)  // Store new config
            }
            // Trigger reload (non-blocking)
            select {
            case triggerReload <- struct{}{}:
            default:  // Already triggered, will use latest config
            }
        }
    }
}
```

**Configuration Storage:**
- `atomic.Pointer[frrConfig]`: Thread-safe config pointer
- Updated before sending trigger signal
- Loaded atomically by reload worker

**Trigger Channel:**
- Buffered channel with size 1
- Non-blocking sends prevent duplicate triggers
- Acts as a semaphore for "reload pending" state

### Component 3: HTTP-Based Config Reload

#### Current Implementation

```go
func reloadConfig() error {
    pid := readPidFile()
    syscall.Kill(pid, syscall.SIGHUP)
    return nil
}
```

No feedback, fire-and-forget.

#### New Implementation

**File: `internal/bgp/frr/config.go`**

```go
func reloadConfig() error {
    // Create HTTP client with Unix socket transport
    client := &http.Client{
        Transport: &http.Transport{
            DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
                return net.Dial("unix", reloaderSocketName)
            },
        },
        Timeout: 30 * time.Second,
    }

    // Generate unique request ID
    requestID := int(reloadRequestIDCounter.Add(1))
    request := reloadRequest{Action: "reload", ID: requestID}

    // Make HTTP POST request (blocks until reload completes)
    resp, err := client.Post("http://unix/", "application/json",
                             bytes.NewReader(requestBody))
    if err != nil {
        return fmt.Errorf("failed to send reload request: %w", err)
    }
    defer resp.Body.Close()

    // Parse and validate response
    var response reloadResponse
    json.NewDecoder(resp.Body).Decode(&response)

    if response.ID != requestID {
        return fmt.Errorf("response ID mismatch")
    }
    if !response.Result {
        return fmt.Errorf("FRR reload failed")
    }

    return nil
}
```

**Key Changes:**

1. **Synchronous feedback**: Caller knows if reload succeeded or failed
2. **Request/response matching**: Unique IDs prevent cross-contamination
3. **Error propagation**: Failures are returned to caller for retry logic
4. **Timeout protection**: 30-second timeout prevents indefinite blocking

### Removed Component: Reload Validator

The previous implementation included a validator that polled `.status` file every 30 seconds to detect failed reloads and trigger retries. This is **removed** because:

1. **Synchronous feedback**: HTTP-based reload provides immediate status
2. **Debouncer retry logic**: Failed reloads are retried automatically within the debouncer
3. **Reduced complexity**: No need for polling and timestamp tracking
4. **Lower API load**: No periodic status file reads or retry events

### Configuration Changes

**Deployment Changes:**

1. **Speaker Dockerfile**: Updated to build and include `frr-reloader` binary
2. **Environment variables**: `FRR_RELOADER_SOCKET` replaces `FRR_RELOADER_PID_FILE`
3. **Container init**: Start `frr-reloader` daemon instead of shell script

Example:
```yaml
env:
- name: FRR_RELOADER_SOCKET
  value: "/etc/frr_reloader/frr-reloader.sock"
```

### Migration Path

The migration is **backward compatible** in deployment:

1. Old reloader script can coexist with new binary
2. Socket path is configurable via environment variable
3. SIGHUP signaling still supported for manual operations
4. No CRD or API changes required

### Performance Characteristics

#### Convergence Time Improvements

**Before:**
- Minimum delay: 3 seconds (debounce timeout)
- Reload duration: 2-3 seconds
- Total time: 5-6 seconds minimum

**After:**
- Minimum delay: 0 seconds (immediate trigger when idle)
- Reload duration: 2-3 seconds (unchanged)
- Total time: 2-3 seconds minimum

**Scenario: Single service creation**
- Before: Config change → 3s wait → 2-3s reload = **5-6s total**
- After: Config change → immediate → 2-3s reload = **2-3s total**
- **Improvement: ~50% reduction**

**Scenario: Rapid service changes (5 services in 1 second)**
- Before: First change → 3s wait → 2-3s reload = **5-6s total**
- After: First change → immediate → 2-3s reload → (4 more batched) → 0s wait → 2-3s reload = **4-6s total**
- **Improvement: Handles bursts better**

**Scenario: Service deletion (last endpoint)**
- Before: Delete → 3s wait → 2-3s reload = **5-6s blackhole**
- After: Delete → immediate → 2-3s reload = **2-3s blackhole**
- **Improvement: ~50% reduction in traffic loss**

#### Resource Utilization

**Before:**
- Timer goroutines: 1 per debouncer
- Periodic polling: Every 30 seconds for validation
- Child processes: Potentially many if reloads accumulate
- Memory: Could grow if reload rate > completion rate

**After:**
- Worker goroutines: 2 per debouncer (receiver + worker)
- HTTP overhead: Minimal (Unix socket, local only)
- Child processes: Serialized by flock (maximum 1 at a time)
- Memory: Bounded by single config in flight

#### Reload Request Handling

**Scenario: Multiple configs during reload**

Before (3-second timer):
```
t=0.0s: Config A arrives → start 3s timer
t=0.5s: Config B arrives → replace A, timer still running
t=1.0s: Config C arrives → replace B, timer still running
t=3.0s: Timer expires → reload C
```

After (event-driven):
```
t=0.0s: Config A arrives → immediate trigger → start reload
t=0.5s: Config B arrives → store B, trigger already pending
t=1.0s: Config C arrives → store C (overwrites B), trigger already pending
t=2.5s: Reload A completes → trigger fires → load C → start reload
t=5.0s: Reload C completes
```

Both approaches batch B and C, but the new approach:
- Starts first reload immediately (saves 3 seconds)
- Processes second batch immediately after first completes (no extra delay)

**Scenario: Reload failure**

Before:
- Failure detected after 30 seconds by validator
- Retry triggered with 3-second debounce
- Total recovery: 33+ seconds

After:
- Failure detected immediately when HTTP returns
- Retry after 5-second sleep (configurable)
- Total recovery: 5-8 seconds

### Testing Strategy

#### Unit Tests

**Debouncer Tests:**
- Config batching during active reload
- Retry behavior on failure
- Atomic config pointer thread safety
- Non-blocking trigger channel behavior

**HTTP Reloader Tests:**
- Request/response ID matching
- Error handling and propagation
- Timeout behavior
- Unix socket communication

**Integration Tests:**
- Full reload cycle with FRR mock
- Concurrent config updates
- Failure and retry scenarios

#### E2E Tests

**Convergence Time Tests:**
1. Measure time from service creation to BGP advertisement
2. Measure time from endpoint deletion to BGP withdrawal
3. Verify improvements vs. baseline

**Stress Tests:**
1. Rapid service creation/deletion (100+ services)
2. Verify no process accumulation
3. Verify memory stability

**Failure Recovery Tests:**
1. Simulate FRR reload failures
2. Verify automatic retry
3. Verify config consistency after recovery

### Observability

#### Metrics (Existing)

No new metrics required; existing reload metrics still apply:
- `metallb_frr_reloads_total`: Counter of reload attempts
- `metallb_frr_reload_errors_total`: Counter of failed reloads

#### Logs

Enhanced logging includes:
- Reload trigger events
- HTTP request/response IDs
- Reload duration
- Batch detection (when configs are replaced during reload)

Example:
```
op=reload action=start
op=reload duration=2.1s result=success
op=reload action=retry reason=failure
```

### Security Considerations

#### Unix Socket Permissions

- Socket created with `0600` permissions (owner read/write only)
- Located in `/etc/frr_reloader` (not world-accessible)
- No network exposure (Unix domain socket only)

#### Process Isolation

- `frr-reloader` runs as same user as speaker
- Uses `flock` for mutual exclusion (same as before)
- No elevation of privileges required

#### Attack Surface

**Removed:**
- PID file parsing vulnerabilities
- Status file race conditions
- Signal handling edge cases

**Added:**
- HTTP server (local only, minimal attack surface)
- JSON parsing (Go stdlib, well-tested)

**Net Impact:** Slightly reduced attack surface, more robust error handling

### Rollout and Compatibility

#### Compatibility Matrix

| Speaker Version | Reloader Type | Status |
|----------------|---------------|---------|
| Old | Shell Script | ✅ Supported (current) |
| New | Shell Script | ✅ Compatible (fallback) |
| New | HTTP Daemon | ✅ Supported (new default) |

#### Migration Steps

1. **Phase 1**: Deploy HTTP reloader alongside existing mechanism
2. **Phase 2**: Update speaker to use HTTP-based reload
3. **Phase 3**: Remove shell script and PID-based logic (future)

#### Rollback Plan

If issues arise:
1. Set `FRR_RELOADER_SOCKET` to empty/unset
2. Revert to PID-based signaling
3. Restart speaker pods

### Open Questions and Future Work

#### Make Retry Interval Configurable

Currently `failureRetryInterval` is hardcoded to 5 seconds. Consider making this configurable via:
- Environment variable
- ConfigMap
- FRRConfiguration CRD

**Rationale:** Different environments may have different tolerance for retry delays.

#### Metrics for Batching Efficiency

Add metrics to track:
- Number of configs batched per reload
- Average time between reloads
- Reload queue depth

**Rationale:** Helps operators understand batching behavior and tune if needed.

#### Support for Priority Reloads

Some config changes (e.g., withdrawals) may be more urgent than others (e.g., new advertisements). Consider:
- Priority levels for reload requests
- Fast path for critical changes

**Rationale:** Could further reduce blackhole windows for deletions.

#### HTTP Reloader Health Checks

Add health check endpoint to HTTP server:
- `GET /health` returns 200 if ready
- Kubernetes liveness/readiness probes

**Rationale:** Better visibility into reloader daemon health.

### Alternatives Considered

#### Alternative 1: Configurable Debounce Timeout

**Approach:** Keep timer-based debouncer but make timeout configurable.

**Pros:**
- Simpler implementation
- Users can tune to their needs

**Cons:**
- Requires deep understanding of FRR internals
- Still has artificial delays even when idle
- Doesn't solve reload completion feedback problem
- Risk of users setting too-low values and causing issues

**Decision:** Rejected in favor of event-driven approach that eliminates delay when possible.

#### Alternative 2: Remove Debouncing Entirely

**Approach:** Reload FRR on every config change immediately.

**Pros:**
- Absolute minimum latency
- Simplest implementation

**Cons:**
- High risk of reload accumulation
- Memory issues under rapid changes (proven in commit `1de7f14d`)
- CPU waste from redundant reloads
- Potential FRR instability

**Decision:** Rejected due to safety concerns and historical evidence of problems.

#### Alternative 3: WebSocket-Based Communication

**Approach:** Use WebSocket instead of HTTP for bidirectional communication.

**Pros:**
- Could support streaming updates
- Slightly lower overhead for repeated requests

**Cons:**
- More complex than HTTP POST
- No clear benefit for request/response pattern
- Additional dependencies

**Decision:** Rejected; HTTP POST is sufficient and simpler.

#### Alternative 4: gRPC-Based Communication

**Approach:** Use gRPC for speaker-to-reloader communication.

**Pros:**
- Type-safe protocol
- Built-in streaming support

**Cons:**
- Heavier dependency footprint
- Overkill for simple request/response
- Protobuf compilation complexity

**Decision:** Rejected; HTTP with JSON is simpler and adequate.

### References

- GitHub Issue: [#2900 - FRR BGP advertisement delayed by ~3-4s](https://github.com/metallb/metallb/issues/2900)
- Original debouncer commit: `88a938e0abaeef87201b4fc5a2ed7e5a43f9e36d`
- Timeout increase commit: `1de7f14dbf25b0844ddaa815b2e7006da7d59d6c`
- FRR reload documentation: https://docs.frrouting.org/en/latest/setup.html

### Development Phases

The implementation can be developed in the following phases:

#### Phase 1: HTTP Reloader Daemon ✅ (Complete in prototype)
- Implement `frr-tools/reloader/frr-reloader.go`
- Add HTTP server with Unix socket
- Port shell script logic to Go
- Add unit tests
- Update Dockerfile to build binary

#### Phase 2: Event-Driven Debouncer ✅ (Complete in prototype)
- Replace timer-based debouncer with event-driven version
- Implement atomic config storage
- Add non-blocking trigger channel
- Add retry logic
- Update unit tests

#### Phase 3: HTTP-Based Reload Client ✅ (Complete in prototype)
- Update `reloadConfig()` to use HTTP
- Add request ID generation
- Implement response validation
- Remove `reloadValidator` function

#### Phase 4: E2E Testing and Documentation (Future)
- Add convergence time tests
- Add stress tests
- Update user documentation
- Create migration guide

#### Phase 5: Cleanup (Future)
- Remove shell script fallback
- Remove PID file logic
- Remove status file polling
- Simplify environment variables

### Success Criteria

The implementation will be considered successful when:

1. ✅ **Convergence time reduced**: Service advertisements occur within 3 seconds of endpoint ready (vs. 5-6 seconds before)
2. ✅ **No reload accumulation**: Memory remains stable under rapid service changes
3. ✅ **Immediate retry on failure**: Failed reloads retry within 5 seconds (vs. 30+ seconds before)
4. **E2E tests pass**: All existing FRR mode tests continue to pass
5. **Backward compatible**: Can deploy without breaking existing installations
6. **Production validated**: Runs successfully in production for 30+ days without issues

### Conclusion

The HTTP-based FRR reload mechanism and event-driven debouncer provide significant improvements to MetalLB's BGP convergence time while maintaining safety and reliability. By eliminating fixed timer delays and providing synchronous reload feedback, the system can respond to configuration changes in near real-time while still batching rapid updates to prevent overload.

The prototype implementation demonstrates the feasibility of the approach and provides a foundation for full production deployment.
