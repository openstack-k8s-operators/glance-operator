# Probes Defaults Configuration

The glance-operator provides flexible health check configuration through
Kubernetes probes. This document explains the probe configuration, timing
calculations, and best practices.

## Overview

The operator supports three types of probes:
- **Liveness Probes** - Determines if a container needs to be restarted
- **Readiness Probes** - Determines if a container can receive traffic
- **Startup Probes** - Handles initial container startup before other probes begin

All probes use the `/healthcheck` endpoint exposed by the Glance API service.

## Default Probe Configuration

### Dynamic Timing Based on APITimeout

The probe timings are dynamically calculated based on the `apiTimeout`
parameter to ensure alignment with HAProxy and Apache timeout settings:

```go
period = floor(apiTimeout * 0.3)          // ~1/3 of apiTimeout
timeout = max(period * 0.8, 5)            // 80% of period to prevent overlapping probes
startupPeriod = min(10, max(5, period/2)) // Capped for reasonable startup times
```

> **Note**:
> Using `0.3` instead of dividing by 3 keeps the entire calculation in `float64`,
> which avoids potential integer overflow when converting from `int` to `int32`
> (gosec G115). The slightly lower factor (`0.3` vs `0.333...`) results in tighter
> health checking, which is a deliberate safety margin.

### Default Values (apiTimeout=60s)

| Probe Type | Timeout | Period | Initial Delay | Failure Threshold | Total Unhealthy Time |
|------------|---------|--------|---------------|-------------------|---------------------|
| Liveness   | 14s     | 18s    | 5s            | 3                 | 54s (3 × 18s)       |
| Readiness  | 14s     | 18s    | 5s            | 3                 | 54s (3 × 18s)       |
| Startup    | 5s      | 9s     | 5s            | 12                | 108s (12 × 9s)      |

### Timing Examples for Different APITimeout Values

| apiTimeout | Period | Timeout | Startup Period | Max Startup Time | Total Unhealthy Time |
|-----------|--------|---------|----------------|------------------|---------------------|
| 30s       | 9s     | 7s      | 5s (min)       | 60s (1 min)      | 27s (3 × 9s)        |
| 60s       | 18s    | 14s     | 9s             | 108s (~2 min)    | 54s (3 × 18s)       |
| 120s      | 36s    | 28s     | 10s (capped)   | 120s (2 min)     | 108s (3 × 36s)      |
| 300s      | 90s    | 72s     | 10s (capped)   | 120s (2 min)     | 270s (3 × 90s)      |

## Design Decisions

### 1. Timeout < Period (80% Rule)

**Why**: Kubernetes best practice dictates that `timeoutSeconds` should be less
than `periodSeconds` to prevent overlapping probe executions.

**Problem**: If a probe takes the full timeout duration (e.g., 20s) and the
period is also 20s, the next probe execution starts immediately, potentially
causing:
- Resource contention
- Cascading failures
- False positives during high load

**Solution**: Set timeout to 80% of period, providing a 20% buffer between
probe executions.

### 2. Explicit Failure Threshold

**Why**: Makes the total unhealthy time calculation transparent.

`Total Unhealthy Time = periodSeconds × failureThreshold`

With `failureThreshold: 3` and `periodSeconds: 18` (apiTimeout=60s):
- First failure at t=0: Pod still healthy
- Second failure at t=18s: Pod still healthy
- Third failure at t=36s: Pod marked unhealthy
- Total time to failure: 54s

This stays slightly under the `apiTimeout` value (54s vs 60s), providing a
tighter health checking window that detects failures before the timeout
expires.

### 3. Capped Startup Period

**Why**: Prevents excessive startup detection times for large `apiTimeout`
values.

**Problem**: Without capping:
- `apiTimeout=300s` -> `startupPeriod=150s`
- Total startup time: `150s × 12 = 1800s` (30 minutes!)

**Solution**: Cap `startupPeriod` at 10 seconds maximum:
```go
startupPeriod = min(10, max(5, period/2))
```

This ensures:
- Minimum 5s period for fast iteration during startup
- Maximum 10s period even for large apiTimeout values
- Maximum startup detection time: `10s × 12 = 120s` (2 minutes)

### 4. Separate Startup Probes

**Why**: Allow containers to start slowly without affecting liveness/readiness
probes.

Startup probes run first with:
- Higher failure threshold (12 vs 3)
- Shorter period for faster detection
- After success, liveness/readiness probes take over

This prevents premature restarts during slow application initialization (e.g.,
database migrations, cache warming).

## Customizing Probes

### Via GlanceAPI CR Override

You can override default probe settings per GlanceAPI instance:

```yaml
apiVersion: glance.openstack.org/v1beta1
kind: GlanceAPI
metadata:
  name: glance-default
spec:
  apiTimeout: 120  # Affects default probe timing
  override:
    probes:
      livenessProbes:
        path: "/healthcheck"
        initialDelaySeconds: 10
        timeoutSeconds: 30
        periodSeconds: 40
        failureThreshold: 5
      readinessProbes:
        path: "/healthcheck"
        initialDelaySeconds: 10
        timeoutSeconds: 30
        periodSeconds: 40
        failureThreshold: 3
      startupProbes:
        timeoutSeconds: 10
        periodSeconds: 5
        initialDelaySeconds: 5
        failureThreshold: 20
```

## Probe Validation

The operator validates probe configurations via admission webhooks using
`lib-common`'s `ValidateProbes()` method. Invalid configurations are rejected
at CR creation/update time.

Validation checks:
- All timeout and period values are positive integers
- Failure thresholds are >= 1
- Path is non-empty for HTTP probes

## TLS Considerations

When TLS is enabled for the API endpoints:
- Probe scheme automatically switches to `HTTPS`
- Certificates are validated using Kubernetes service CA
- No configuration changes needed

The operator automatically configures the correct scheme based on the TLS settings:

```go
if instance.Spec.TLS.API.Enabled(endpoint) {
    scheme = corev1.URISchemeHTTPS
} else {
    scheme = corev1.URISchemeHTTP
}
```

## Best Practices

### 1. Align apiTimeout with Expected Response Times

Set `apiTimeout` based on your slowest expected Glance operation:
- Standard deployments: 60s (default)
- Large image operations: 120-300s
- High-latency backends: 180-300s

### 2. Monitor Probe Failures

Watch for probe failure patterns:

```bash
# Check probe failures in pod events
oc describe pod glance-default-api-0 -n openstack

# View probe timing in container spec
oc get pod glance-default-api-0 -n openstack -o jsonpath='{.spec.containers[0].livenessProbe}'
```

### 3. Adjust for Backend Performance

Slow storage backends may require:
- Higher `apiTimeout` (e.g., 180-300s)
- Increased `initialDelaySeconds` (e.g., 15-30s)
- Higher `failureThreshold` for readiness probes

## References

- [Kubernetes Probe Documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [lib-common Probes Module](https://github.com/openstack-k8s-operators/lib-common/tree/main/modules/common/probes)
- [GlanceAPI CRD Reference](../api/v1beta1/glanceapi_types.go)
- [Probes Defaults Implementation](../internal/glance/funcs.go)
