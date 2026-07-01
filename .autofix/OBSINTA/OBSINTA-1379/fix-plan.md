## Fix Plan for OBSINTA-1379

### Version
Plan v1 | Iteration 0 (initial draft) | Audit skipped — simple fix

### Root Cause
`NewAlertmanagerClient` in `pkg/alertmanager/loader.go` (line 53) creates a default HTTP client
via `client.NewHTTPClientWithConfig(nil, cfg)`, which internally calls `httptransport.New(host, basePath, schemes)`.
This creates a new `http.Client` with default settings, completely ignoring the `apiConfig.RoundTripper`
that carries:
- TLS settings (including `InsecureSkipVerify` from the `--insecure` flag)
- Bearer token authentication (from kubeconfig or service account)

Both callers (`pkg/mcp/auth.go:104` and `pkg/toolset/tools/prometheus_client.go:116`) pass an
`api.Config` with a properly configured `RoundTripper`, but the function discards it.

In contrast, the Prometheus client (`pkg/prometheus/loader.go:42`) correctly passes the full
`apiConfig` (including `RoundTripper`) via `api.NewClient(apiConfig)`.

### Approach
Replace `client.NewHTTPClientWithConfig(nil, cfg)` with a manual construction that:
1. Creates an `*http.Client` with `apiConfig.RoundTripper` as its `Transport`
2. Uses `httptransport.NewWithClient(host, basePath, schemes, httpClient)` from
   `github.com/go-openapi/runtime/client` to create the go-openapi runtime transport
3. Passes that runtime transport to `client.New(transport, nil)` to create the AlertmanagerAPI client

This approach uses the same `go-openapi/runtime/client` package already imported transitively,
and mirrors how the go-openapi library itself creates clients but with a custom HTTP transport.

If `apiConfig.RoundTripper` is nil (no auth/TLS needed), fall back to `http.DefaultTransport`
for backward compatibility.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Wrap Alertmanager behind the Prometheus `api.Client` interface | Reuses exact Prometheus pattern | Alertmanager uses go-swagger generated client (different library), not prometheus/client_golang; would require major refactoring | Architectural mismatch |
| 2 | Create custom `http.Client` and use `SetTransport()` after creation | Simple post-creation injection | `SetTransport` sets the go-openapi `runtime.ClientTransport`, not the `http.RoundTripper`; would still use default HTTP client for actual connections | Doesn't solve the problem |
| 3 | Use `httptransport.NewWithClient` with custom `http.Client` (chosen) | Direct, minimal change, uses existing library API, preserves full RoundTripper chain | Adds one import | Best fit — minimal, correct |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `pkg/alertmanager/loader.go` | Replace `client.NewHTTPClientWithConfig(nil, cfg)` with `httptransport.NewWithClient` + `client.New` pattern using `apiConfig.RoundTripper` | Core fix: pipe RoundTripper into the Alertmanager HTTP client |
| `pkg/alertmanager/loader_test.go` | Add `TestNewAlertmanagerClientUsesRoundTripper` test | Regression test: verify that custom RoundTripper is actually used |

### Dependencies & Side Effects
- [ ] Public API change? — No (function signature unchanged)
- [ ] Config / env var change? — No
- [ ] Database migration? — No
- [ ] Downstream consumer impact? — No (callers unchanged)
- [ ] Error handling / logging change? — No
- [ ] Performance characteristics change? — No

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| nil RoundTripper when no auth configured | LOW | MEDIUM | Fallback to `http.DefaultTransport` |
| BasePath mismatch | LOW | LOW | Use `client.DefaultTransportConfig()` constants for BasePath |
| Breaking existing mock tests | LOW | LOW | Existing tests use mock interface, not `NewAlertmanagerClient` directly |

### Test Strategy
- Existing tests to verify: `pkg/alertmanager/loader_test.go` (uses mock interface, should pass unchanged)
- New regression test: `TestNewAlertmanagerClientUsesRoundTripper` — creates a custom RoundTripper, passes it via `api.Config`, calls `NewAlertmanagerClient`, and verifies the resulting client uses the custom transport (e.g., by checking the transport chain or making a request to a test server that verifies headers/TLS)

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Line 53 of `loader.go` creates default client; `apiConfig.RoundTripper` is never referenced anywhere in the function |
| Approach correctness | HIGH | `httptransport.NewWithClient` exists in `go-openapi/runtime@v0.29.2` (verified at line 275 of `client/runtime.go`); `client.New(transport, nil)` is the AlertmanagerAPI constructor (verified in alertmanager client source) |
| Scope completeness | HIGH | Only 1 file needs the core fix + 1 test file; both callers (`auth.go:104`, `prometheus_client.go:116`) already pass configured RoundTripper and need no changes |

### Investigation Strategy
**Signals detected**: default
**Strategy used**: Standard investigation (grep, file reads, code path tracing)
**Key findings from strategy**:
  - Traced the full call chain: `createAPIConfig` → `apiConfig.RoundTripper` set → `NewAlertmanagerClient` → `client.NewHTTPClientWithConfig` (RoundTripper discarded)
  - Compared with working Prometheus client path: `createAPIConfig` → `api.NewClient(apiConfig)` (RoundTripper preserved)
  - Verified `go-openapi/runtime/client.NewWithClient` API exists and accepts `*http.Client` parameter
