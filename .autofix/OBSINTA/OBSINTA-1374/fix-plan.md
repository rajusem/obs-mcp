## Fix Plan for OBSINTA-1374

### Version
Plan v1 | Audit skipped (simple fix — all confidence HIGH, ≤2 files, <20 lines)

### Root Cause
In `pkg/alertmanager/loader.go`, `NewAlertmanagerClient()` constructs a `client.TransportConfig` with only host/scheme settings and calls `client.NewHTTPClientWithConfig(nil, cfg)`. This function internally creates a default `go-openapi/runtime/client.Runtime` without a custom HTTP client, completely discarding `apiConfig.RoundTripper`.

The `RoundTripper` in `apiConfig` carries:
- TLS settings (`InsecureSkipVerify` for `--insecure` flag, or custom CA cert pool)
- Bearer token authentication (wrapped in `promcfg.NewAuthorizationCredentialsRoundTripper`)

The caller `getAlertmanagerClient()` in `pkg/mcp/auth.go` correctly builds an `apiConfig` with TLS/auth settings, but `NewAlertmanagerClient()` silently ignores `apiConfig.RoundTripper`. By contrast, `NewPrometheusClient()` uses `api.NewClient(apiConfig)` which correctly threads the `RoundTripper` through — explaining why Prometheus works but Alertmanager does not.

### Approach
Replace the `client.DefaultTransportConfig()` + `client.NewHTTPClientWithConfig()` pattern with:
1. Extract the `RoundTripper` from `apiConfig` (fall back to `http.DefaultTransport` if nil)
2. Wrap it in an `*http.Client{Transport: rt}`
3. Create the go-openapi transport with `httptransport.NewWithClient(host, client.DefaultBasePath, []string{scheme}, httpClient)`
4. Create the Alertmanager API client with `client.New(transport, nil)`

This is the correct go-openapi idiom for injecting a custom HTTP client, and is the approach used in the already-merged fix on branch `OBSINTA-1330`.

### Planned Files
- `pkg/alertmanager/loader.go` — Replace `client.DefaultTransportConfig()` + `client.NewHTTPClientWithConfig(nil, cfg)` with `httptransport.NewWithClient()` + `client.New()` using `apiConfig.RoundTripper`. Add imports for `net/http` and `httptransport "github.com/go-openapi/runtime/client"`.
- `pkg/alertmanager/loader_test.go` — Add regression test `TestNewAlertmanagerClientUsesRoundTripper` with `trackingRoundTripper` to verify the custom RoundTripper is actually called during HTTP requests.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Modify `TransportConfig` to include custom transport | Simple-looking | `TransportConfig` struct has no `RoundTripper` or `HTTPClient` field | Field doesn't exist in generated client |
| 2 | Pass `apiConfig.RoundTripper` as `formats` arg | — | `formats` is `strfmt.Registry`, unrelated type | Wrong API usage |
| 3 | Create separate auth path for Alertmanager | Separate concern | Duplicates auth logic already in `auth.go` | Unnecessarily complex |
| 4 | `httptransport.NewWithClient()` + `client.New()` (chosen) | Minimal, correct idiom, proven in OBSINTA-1330 | None | This is the fix |

### Dependencies & Side Effects
- [x] Public API change? **No** — `NewAlertmanagerClient` signature unchanged
- [ ] Config / env var change? **No**
- [ ] Database migration? **No**
- [ ] Downstream consumer impact? **No** — behavior is corrected (was broken before)
- [ ] Error handling / logging change? **No**
- [ ] Performance characteristics change? **No** — same HTTP call, correct transport

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Breaking behavior for plain HTTP | LOW | LOW | If `apiConfig.RoundTripper` is nil, fall back to `http.DefaultTransport` — equivalent to old behavior |
| go-openapi/runtime API incompatibility | VERY LOW | MEDIUM | `NewWithClient` is the documented API; same approach works in OBSINTA-1330 branch |
| Test flakiness | VERY LOW | LOW | Test uses `httptest.Server` (local, deterministic) |

### Test Strategy
- **Existing tests**: `TestGetAlerts`, `TestGetSilences` should remain green
- **New regression test**: `TestNewAlertmanagerClientUsesRoundTripper` — verifies the custom RoundTripper is invoked during `GetAlerts`; fails if bug is reintroduced

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Direct code inspection: `NewHTTPClientWithConfig` calls `httptransport.New()` (not `NewWithClient()`), discarding custom transport |
| Approach correctness | HIGH | Identical fix already merged in branch `OBSINTA-1330` (commit `ca6aeb7`) |
| Scope completeness | HIGH | Only one construction site (`NewAlertmanagerClient`) creates the Alertmanager HTTP client |

### Audit Trail
- Audit skipped — simple fix (all confidence HIGH, ≤2 files, <20 lines, clear regression with single root cause)
