# Fix Plan for OBSINTA-1370

## Summary
`NewAlertmanagerClient` creates a default HTTP client, ignoring the `RoundTripper` from `apiConfig`.

## Root Cause
`NewAlertmanagerClient` in `pkg/alertmanager/loader.go` (line 53) accepts an `api.Config`
parameter that contains a `RoundTripper` set by all callers with Kubernetes authentication
(bearer tokens, kubeconfig credentials, TLS configuration). However, the function ignores
`apiConfig.RoundTripper` entirely and calls `client.NewHTTPClientWithConfig(nil, cfg)` which
creates a new HTTP client using the default transport from `go-openapi/runtime`. This means
all authentication and TLS configuration is silently discarded, causing Alertmanager API calls
to fail or make unauthenticated requests.

**Evidence**:
- `pkg/alertmanager/loader.go:53`: `c := client.NewHTTPClientWithConfig(nil, cfg)` — ignores `apiConfig.RoundTripper`
- `pkg/mcp/auth.go:104`: `alertmanager.NewAlertmanagerClient(apiConfig)` — caller sets `apiConfig.RoundTripper` with auth
- `pkg/toolset/tools/prometheus_client.go:110-116`: caller sets `apiConfig.RoundTripper: rt` then passes to `NewAlertmanagerClient`
- `pkg/prometheus/loader.go:41-42`: contrast — `NewPrometheusClient` correctly passes `apiConfig` directly to `api.NewClient(apiConfig)`, which uses `apiConfig.RoundTripper`

## Approach
Use `httptransport.NewWithClient` (from `github.com/go-openapi/runtime/client`) to create the
Alertmanager transport with an `http.Client` that wraps `apiConfig.RoundTripper`. This is the
idiomatic go-openapi pattern for injecting a custom transport.

**Specific changes in `NewAlertmanagerClient`**:
1. After building `cfg` (host/scheme), if `apiConfig.RoundTripper != nil`:
   - Create `httpClient := &http.Client{Transport: apiConfig.RoundTripper}`
   - Build transport via `httptransport.NewWithClient(host, cfg.BasePath, []string{scheme}, httpClient)`
   - Call `client.New(transport, nil)` to create the `AlertmanagerAPI`
2. Else: fall back to existing `client.NewHTTPClientWithConfig(nil, cfg)` (no regression)

This approach:
- Reuses the same `go-openapi/runtime` dependency already used by `NewHTTPClientWithConfig`
- Requires no new dependencies
- Preserves backward compatibility (when `RoundTripper` is nil, falls back to existing behavior)

## Planned Files
- `pkg/alertmanager/loader.go` — Add `net/http` import and `httptransport "github.com/go-openapi/runtime/client"` import; update `NewAlertmanagerClient` to use `apiConfig.RoundTripper` when non-nil via `httptransport.NewWithClient` + `client.New`
- `pkg/alertmanager/loader_test.go` — Add `TestNewAlertmanagerClientUsesRoundTripper` regression test that verifies the provided RoundTripper is actually called when the client makes requests

## Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Wrap RoundTripper in http.Client + use httptransport.NewWithClient (CHOSEN) | Uses existing deps, idiomatic go-openapi pattern, minimal change | Requires adding imports | Best fit |
| 2 | Change callers to not pass RoundTripper; build auth into alertmanager package | Avoids parameter threading issue | Major refactor, breaks consistency with Prometheus approach | Too invasive |
| 3 | Add RoundTripper field to alertmanager.TransportConfig | Custom wrapper, flexible | Non-standard, extra wrapper type, more code | Unnecessary complexity |

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Backward compat break when RoundTripper is nil | Low | Low | Code checks for nil, falls back to existing default path |
| `httptransport` import version mismatch | Low | Medium | Already used transitively by alertmanager client, same module |

## Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Line 53 in loader.go clearly ignores apiConfig.RoundTripper; callers in mcp/auth.go:102-104 and toolset/tools/prometheus_client.go:110-116 explicitly set it |
| Approach correctness | HIGH | `httptransport.NewWithClient` is the idiomatic go-openapi pattern; confirmed via `go doc github.com/go-openapi/runtime/client.NewWithClient` |
| Scope completeness | HIGH | Single root cause in one file; both callers already correctly set `apiConfig.RoundTripper` — only loader.go needs the fix |

## Audit Trail
- Complexity gate: Rule 5 matched — ≤2 files, <20 lines, all confidence HIGH, default signal
- Audit: SKIPPED (AUDIT_SKIP_SIMPLE=true, all dimensions HIGH, simple fix)
