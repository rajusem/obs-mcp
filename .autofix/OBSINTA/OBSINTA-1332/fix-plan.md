## Fix Plan for OBSINTA-1332

### Version
Plan v1 | Iteration 0 (initial draft)

### Root Cause
In `pkg/alertmanager/loader.go`, `NewAlertmanagerClient` calls `client.NewHTTPClientWithConfig(nil, cfg)` where `nil` causes the library to create a brand-new default `httptransport.New(host, basePath, schemes)` HTTP transport. This ignores `apiConfig.RoundTripper` entirely — the field that carries TLS settings (InsecureSkipVerify for `--insecure` flag) and bearer token authentication configured by callers in `pkg/mcp/auth.go` and `pkg/toolset/tools/prometheus_client.go`. As a result, connecting to a secured (HTTPS + auth) Alertmanager endpoint fails with x509 certificate errors or 401 Unauthorized.

### Approach
Replace `client.NewHTTPClientWithConfig(nil, cfg)` with:
1. Create an `http.Client` that uses `apiConfig.RoundTripper` (falling back to `http.DefaultTransport` if nil).
2. Use `httptransport.NewWithClient(host, basePath, schemes, httpClient)` to create a transport that respects the configured RoundTripper.
3. Use `client.New(transport, nil)` to create the AlertmanagerAPI client.

This approach is the correct pattern for injecting a custom `http.RoundTripper` into the go-openapi generated client (see `NewWithClient` in the `go-openapi/runtime/client` package). It's minimal — changes only the client construction in `NewAlertmanagerClient`, preserving all the surrounding URL-parsing logic. Prometheus' own client handles this via `api.NewClient(apiConfig)` which internally uses the RoundTripper, so we're aligning Alertmanager with the same pattern.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | **Use `httptransport.NewWithClient`** (chosen) | Minimal change, uses existing RoundTripper, aligns with go-openapi API design | Requires importing `httptransport` package | Selected |
| 2 | Change `NewAlertmanagerClient` signature to accept `*http.Client` instead of `api.Config` | Simpler type | Breaking API change; callers already pass `api.Config` | More invasive, changes public API |
| 3 | Wrap TLS config manually inside `NewAlertmanagerClient` | Doesn't require RoundTripper | Duplicates TLS/auth logic already done in callers; TLS settings become inconsistent | Wrong abstraction layer |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `pkg/alertmanager/loader.go` | Replace `client.NewHTTPClientWithConfig(nil, cfg)` with `httptransport.NewWithClient` + `client.New`; add `net/http` import | Core fix — propagate `apiConfig.RoundTripper` to the go-openapi HTTP transport |
| `pkg/alertmanager/loader_test.go` | Add regression test for `NewAlertmanagerClient` that verifies a custom RoundTripper is used | Prevent recurrence |

### Dependencies & Side Effects
- [ ] Public API change? No — `NewAlertmanagerClient(api.Config)` signature unchanged
- [ ] Config / env var change? No
- [ ] Database migration? No
- [ ] Downstream consumer impact? No — internal behavior change only (correctly uses RoundTripper now)
- [ ] Error handling / logging change? No
- [ ] Performance characteristics change? No

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| RoundTripper is nil for non-HTTPS callers | Medium | Low — fallback to `http.DefaultTransport` handles it | Add nil check: `if apiConfig.RoundTripper == nil { apiConfig.RoundTripper = http.DefaultTransport }` |
| go-openapi version compatibility | Low | Medium | `httptransport.NewWithClient` is present in all versions in go.mod; verified via `go doc` |

### Test Strategy
- Existing tests to verify: `pkg/alertmanager/loader_test.go` (existing mock tests pass)
- New regression test: `TestNewAlertmanagerClientUsesRoundTripper` — creates a custom `RoundTripper` that records whether it was called, calls `NewAlertmanagerClient`, makes a test HTTP request, and asserts the custom RoundTripper was invoked.

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | `loader.go:53` — `client.NewHTTPClientWithConfig(nil, cfg)` creates default transport, confirmed by reading `alertmanager_api_client.go:NewHTTPClientWithConfig` source which calls `httptransport.New` (no RoundTripper param) |
| Approach correctness | HIGH | `go doc github.com/go-openapi/runtime/client.NewWithClient` confirms the function exists and accepts `*http.Client`, allowing custom RoundTripper injection |
| Scope completeness | HIGH | All call sites for `NewAlertmanagerClient` examined (`auth.go:104`, `toolset/tools/prometheus_client.go:116`) — both correctly configure RoundTripper in `apiConfig` before calling; the bug is entirely in the callee |

### Investigation Strategy
**Signals detected**: default (clear code bug — transport config ignored)
**Strategy used**: Standard code path tracing
**Key findings from strategy**:
  - `NewAlertmanagerClient` receives `api.Config` with a populated `RoundTripper` from both callers
  - The go-openapi generated `NewHTTPClientWithConfig` always creates a new `httptransport.New(...)` transport, ignoring any RoundTripper
  - `httptransport.NewWithClient` exists in the same package and accepts `*http.Client` for custom transport injection
  - Prometheus client (`NewPrometheusClient`) correctly uses `api.NewClient(apiConfig)` which respects the RoundTripper — Alertmanager should match this pattern

### Audit Trail
- Audit disabled by configuration (simple fix, all confidence HIGH)