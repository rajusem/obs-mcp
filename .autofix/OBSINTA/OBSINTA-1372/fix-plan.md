# Fix Plan for OBSINTA-1372

**Ticket**: [OBSINTA-1372] Alertmanager ignoring TLS and auth transport config
**Plan Version**: v1 (audit skipped — all confidence HIGH, 1 file, <10 lines, regression signal)
**Branch**: OBSINTA-1372/alertmanager-ignoring-tls-and-auth-transport-co

---

## Root Cause

In `pkg/alertmanager/loader.go` (line 53), `NewAlertmanagerClient` calls:
```go
c := client.NewHTTPClientWithConfig(nil, cfg)
```

This internally calls `httptransport.New(cfg.Host, cfg.BasePath, cfg.Schemes)` from
`github.com/go-openapi/runtime/client`, which hardcodes `rt.Transport = http.DefaultTransport`
(go-openapi/runtime@v0.29.2/client/runtime.go:254).

The `apiConfig.RoundTripper` — which carries TLS settings (`--insecure` flag via
`TLSClientConfig{Insecure: true}`) and Bearer token authentication — is completely
discarded. Both callers properly construct a `api.Config` with a configured `RoundTripper`
(`pkg/mcp/auth.go:110` and `pkg/toolset/tools/prometheus_client.go:110-113`) but the
Alertmanager client silently ignores it.

By contrast, the Prometheus client (`pkg/prometheus/loader.go:42`) correctly calls
`api.NewClient(apiConfig)` which respects the `RoundTripper` field.

---

## Approach

Replace the `client.NewHTTPClientWithConfig(nil, cfg)` call with a transport construction
that wires in `apiConfig.RoundTripper`:

```go
// Before (broken):
c := client.NewHTTPClientWithConfig(nil, cfg)

// After (fixed):
rt := apiConfig.RoundTripper
if rt == nil {
    rt = http.DefaultTransport
}
transport := httptransport.NewWithClient(cfg.Host, client.DefaultBasePath, cfg.Schemes, &http.Client{Transport: rt})
c := client.New(transport, nil)
```

Additional import changes needed in `pkg/alertmanager/loader.go`:
- Add: `httptransport "github.com/go-openapi/runtime/client"`
- Add: `"net/http"` 
- The existing `"github.com/prometheus/alertmanager/api/v2/client"` import stays (used by `client.DefaultBasePath` and `client.New`)

This uses only exported APIs and falls back safely to `http.DefaultTransport` when no
custom transport is provided.

---

## Planned Files

- `pkg/alertmanager/loader.go` — Replace lines 49-53 with RoundTripper-aware transport
  construction using `httptransport.NewWithClient` + `client.New`; add imports for
  `net/http` and `github.com/go-openapi/runtime/client`

- `pkg/alertmanager/loader_test.go` — Add `TestNewAlertmanagerClientUsesRoundTripper`
  test that verifies a custom RoundTripper is wired through (optional but strongly
  recommended for regression prevention)

---

## Audit Trail

- **Audit skipped**: Simple fix — 1 file, ~10 lines, all confidence HIGH, clear regression signal
- Architecture: N/A (skipped)
- PE: N/A (skipped)
- Language Expert: N/A (skipped)

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `client.DefaultBasePath` not exported | LOW | HIGH | Verified exported in alertmanager_api_client.go |
| `httptransport.NewWithClient` not available | LOW | HIGH | Verified in go-openapi/runtime@v0.29.2 line 275 |
| Nil RoundTripper crash | LOW | HIGH | Explicitly handled: fallback to `http.DefaultTransport` |
| Regression for HTTP (non-TLS) endpoints | LOW | MEDIUM | Fallback preserves existing behavior |

---

## Test Strategy

- All existing mock-based tests in `pkg/alertmanager/loader_test.go` continue to pass
  (they use the `Loader` interface, not `NewAlertmanagerClient` directly)
- New test: `TestNewAlertmanagerClientUsesRoundTripper` validates the RoundTripper is
  correctly wired into the created client transport
- Run: `go test ./pkg/alertmanager/...`

---

## Session Telemetry

| Metric | Value |
|--------|-------|
| Model | claude-sonnet-4-6@default |
| Session started | 2026-07-01 |
| Audit | Skipped (simple fix, all HIGH confidence) |
