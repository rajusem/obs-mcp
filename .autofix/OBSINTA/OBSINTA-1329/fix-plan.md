## Fix Plan for OBSINTA-1329

### Version
Plan v1 | Iteration 0 (initial draft)

### Root Cause
`NewAlertmanagerClient` in `pkg/alertmanager/loader.go` (line 32-57) receives an `api.Config` containing a properly configured `RoundTripper` with TLS settings and bearer token authentication, but **completely ignores it**. On line 53, it calls `client.NewHTTPClientWithConfig(nil, cfg)` which creates a new default HTTP client with no custom transport configuration. This causes:
- **x509 certificate errors** when connecting to secured Alertmanager endpoints (TLS `InsecureSkipVerify` setting is lost)
- **401 Unauthorized errors** when bearer token authentication is required (the `RoundTripper` wrapping the token is discarded)

By contrast, the Prometheus client path (`pkg/prometheus/loader.go:41-42`) correctly passes the entire `apiConfig` (including `RoundTripper`) to `api.NewClient(apiConfig)`, which uses the custom transport.

### Approach
Replace `client.NewHTTPClientWithConfig(nil, cfg)` with a construction path that uses the `apiConfig.RoundTripper`:

1. Build an `*http.Client` using the provided `apiConfig.RoundTripper` (falling back to `http.DefaultTransport` if nil)
2. Use `runtimeclient.NewWithClient(host, client.DefaultBasePath, schemes, httpClient)` from `github.com/go-openapi/runtime/client` to create a `Runtime` transport that carries the custom HTTP client
3. Use `amclient.New(transport, nil)` to create the `AlertmanagerAPI` with the custom transport

This ensures TLS settings and bearer token auth flow through to all Alertmanager HTTP requests.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Create custom `http.Client` and use `NewWithClient` from `go-openapi/runtime/client` | Uses existing API, clean, passes transport through | Requires importing `go-openapi/runtime/client` | **Selected approach** |
| 2 | Modify `NewHTTPClientWithConfig` upstream | Would fix it at the source | We don't control the alertmanager library; upstream change scope | Not feasible for this fix |
| 3 | Create a separate HTTP client outside `NewAlertmanagerClient` and replace its transport after construction | Simpler code change | Fragile, depends on internal implementation of `NewHTTPClientWithConfig` | Breaks encapsulation |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `pkg/alertmanager/loader.go` | Replace `client.NewHTTPClientWithConfig(nil, cfg)` with `amclient.New(runtimeclient.NewWithClient(..., httpClient), nil)` using `apiConfig.RoundTripper` | Core bug fix: pass the configured RoundTripper through to the Alertmanager HTTP client |
| `pkg/alertmanager/loader_test.go` | Add `TestNewAlertmanagerClient_UsesRoundTripper` test | Verify the RoundTripper from apiConfig is actually used by the created client |

### Specific Change Details

**`pkg/alertmanager/loader.go`:**

Add imports:
```go
"net/http"
runtimeclient "github.com/go-openapi/runtime/client"
```

Replace lines 49-53:
```go
// BEFORE (buggy):
cfg := client.DefaultTransportConfig().
    WithHost(host).
    WithSchemes([]string{scheme})
c := client.NewHTTPClientWithConfig(nil, cfg)

// AFTER (fixed):
transport := apiConfig.RoundTripper
if transport == nil {
    transport = http.DefaultTransport
}
httpClient := &http.Client{Transport: transport}
runtimeTransport := runtimeclient.NewWithClient(host, client.DefaultBasePath, []string{scheme}, httpClient)
c := client.New(runtimeTransport, nil)
```

**`pkg/alertmanager/loader_test.go`:**

Add a test that:
1. Creates a custom `RoundTripper` that records whether it was called
2. Passes it via `api.Config{Address: "https://localhost:9093", RoundTripper: customRT}`
3. Calls `NewAlertmanagerClient`
4. Verifies the client's underlying transport uses the custom `RoundTripper`

### Dependencies & Side Effects
- [ ] Public API change? **No** - `NewAlertmanagerClient` signature unchanged
- [ ] Config / env var change? **No**
- [ ] Database migration? **No**
- [ ] Downstream consumer impact? **No** - all callers already pass `RoundTripper` in `apiConfig`
- [ ] Error handling / logging change? **No**
- [ ] Performance characteristics change? **No**

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `go-openapi/runtime/client` API mismatch | LOW | HIGH | Verified `NewWithClient` exists in `go-openapi/runtime v0.29.2` via `go doc` |
| `client.DefaultBasePath` mismatch | LOW | MEDIUM | Verified it's `/api/v2/` which is the standard Alertmanager API path |
| `nil` strfmt.Registry in `client.New` | LOW | LOW | `NewHTTPClientWithConfig` also passes `nil` for formats — library handles it |

### Test Strategy
- Existing tests to verify: `pkg/alertmanager/loader_test.go` (existing mock tests confirm interface compliance)
- New regression test: `TestNewAlertmanagerClient_UsesRoundTripper` — validates that a custom `RoundTripper` passed in `apiConfig` is used by the resulting client, preventing future regressions

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Line 53 of `loader.go` calls `NewHTTPClientWithConfig` which creates a default HTTP client, ignoring `apiConfig.RoundTripper`. Both callers set `RoundTripper` (auth.go:150, prometheus_client.go:112). Prometheus path works because `api.NewClient` uses it. |
| Approach correctness | HIGH | `go-openapi/runtime/client.NewWithClient` is the documented API for custom HTTP clients. `client.New()` accepts a `runtime.ClientTransport`. Verified via `go doc`. |
| Scope completeness | HIGH | Only 1 file to fix (`loader.go`), 1 test file to add test to. All callers already pass correct `apiConfig`. |

### Investigation Strategy
**Signals detected**: default (straightforward code bug — transport config not being forwarded)
**Strategy used**: Standard investigation (grep, code path tracing)
**Key findings from strategy**:
- The `apiConfig` is correctly constructed with `RoundTripper` in both callers (`pkg/mcp/auth.go` and `pkg/toolset/tools/prometheus_client.go`)
- The Prometheus client path (`NewPrometheusClient`) correctly uses `api.NewClient(apiConfig)` which respects `RoundTripper`
- The Alertmanager client path (`NewAlertmanagerClient`) only uses `apiConfig.Address` and discards `apiConfig.RoundTripper`
- The `go-openapi/runtime/client` package provides `NewWithClient` which accepts a custom `*http.Client`
