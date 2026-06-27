# Fix Plan for OBSINTA-1330

**Ticket**: [OBSINTA-1330] [EVAL] OBSINTA-1176 x Claude Sonnet 4.6 - Alertmanager transport config ignored
**Branch**: `OBSINTA-1330/alertmanager-transport-config-ignored`
**Confidence**: HIGH
**Audit**: Skipped — simple fix, all confidence HIGH, ≤2 files, signal is default

---

## Root Cause

In `pkg/alertmanager/loader.go`, `NewAlertmanagerClient` calls:

```go
c := client.NewHTTPClientWithConfig(nil, cfg)
```

The `nil` first argument causes the library to create a brand-new default HTTP transport via `httptransport.New(host, basePath, schemes)`, completely ignoring `apiConfig.RoundTripper`. The `RoundTripper` field of `api.Config` is set by callers to carry:

- TLS settings (e.g., `InsecureSkipVerify: true` for the `--insecure` flag)
- Bearer token authentication (via `promcfg.NewAuthorizationCredentialsRoundTripper`)

Both call sites correctly configure `apiConfig.RoundTripper` before calling `NewAlertmanagerClient`:
- `pkg/mcp/auth.go:104` — sets RoundTripper with TLS + auth
- `pkg/toolset/tools/prometheus_client.go:116` — sets RoundTripper from REST config

The bug means that all transport customization is silently discarded, resulting in:
- `x509: certificate signed by unknown authority` errors when using HTTPS
- `401 Unauthorized` errors when bearer token auth is required

## Approach

Replace the go-openapi default transport construction with one that uses the configured `RoundTripper`:

**Before** (broken):
```go
c := client.NewHTTPClientWithConfig(nil, cfg)
```

**After** (fixed):
```go
rt := apiConfig.RoundTripper
if rt == nil {
    rt = http.DefaultTransport
}
httpClient := &http.Client{Transport: rt}
transport := httptransport.NewWithClient(host, cfg.BasePath, []string{scheme}, httpClient)
c := client.New(transport, nil)
```

This uses `httptransport.NewWithClient` (from `github.com/go-openapi/runtime/client`) which accepts an `*http.Client`, allowing us to inject the custom `RoundTripper`. Then `client.New` wraps the transport in the generated Alertmanager API client.

The fix is contained to `NewAlertmanagerClient` and does not change any public interface.

## Planned Files

### `pkg/alertmanager/loader.go`

**Changes:**
1. Add `net/http` import
2. Add `httptransport "github.com/go-openapi/runtime/client"` import
3. Replace `c := client.NewHTTPClientWithConfig(nil, cfg)` (line 53) with:

```go
// Use the RoundTripper from apiConfig (carries TLS + auth settings).
// Fall back to http.DefaultTransport for plain HTTP connections.
rt := apiConfig.RoundTripper
if rt == nil {
    rt = http.DefaultTransport
}
httpClient := &http.Client{Transport: rt}
transport := httptransport.NewWithClient(host, cfg.BasePath, []string{scheme}, httpClient)
c := client.New(transport, nil)
```

4. Remove unused `cfg` variable that was only used for `NewHTTPClientWithConfig` (the `DefaultTransportConfig()` call can be simplified — `cfg.BasePath` is `/api/v2/` from `DefaultTransportConfig().BasePath`).

**Full revised `NewAlertmanagerClient`:**
```go
func NewAlertmanagerClient(apiConfig api.Config) (*RealLoader, error) {
    // Parse the URL to extract scheme and host
    parsedURL, err := url.Parse(apiConfig.Address)
    if err != nil {
        return nil, fmt.Errorf("failed to parse Alertmanager URL: %w", err)
    }

    host := parsedURL.Host
    if host == "" {
        host = strings.TrimPrefix(apiConfig.Address, "//")
    }

    scheme := parsedURL.Scheme
    if scheme == "" {
        scheme = "http"
    }

    // Use the RoundTripper from apiConfig (carries TLS + auth settings).
    // Fall back to http.DefaultTransport for plain HTTP connections.
    rt := apiConfig.RoundTripper
    if rt == nil {
        rt = http.DefaultTransport
    }
    httpClient := &http.Client{Transport: rt}
    transport := httptransport.NewWithClient(host, client.DefaultBasePath, []string{scheme}, httpClient)
    c := client.New(transport, nil)

    return &RealLoader{
        client: c,
    }, nil
}
```

### `pkg/alertmanager/loader_test.go`

**Changes:**
Add `TestNewAlertmanagerClientUsesRoundTripper` — a regression test that verifies the custom `RoundTripper` from `apiConfig` is actually invoked when making HTTP requests. Uses an `httptest.Server` and a tracking `RoundTripper` wrapper.

```go
func TestNewAlertmanagerClientUsesRoundTripper(t *testing.T) {
    // Track whether our custom RoundTripper was invoked
    called := false
    trackingRT := &trackingRoundTripper{
        inner:  http.DefaultTransport,
        called: &called,
    }

    apiConfig := promapi.Config{
        Address:      "http://localhost:9093",
        RoundTripper: trackingRT,
    }

    loader, err := NewAlertmanagerClient(apiConfig)
    if err != nil {
        t.Fatalf("expected no error, got: %v", err)
    }
    if loader == nil {
        t.Fatal("expected non-nil loader")
    }

    // Attempt a request (will fail, but RoundTripper should still be called)
    _, _ = loader.GetAlerts(context.TODO(), nil, nil, nil, nil, nil, "")

    if !called {
        t.Error("expected custom RoundTripper to be called, but it was not")
    }
}

type trackingRoundTripper struct {
    inner  http.RoundTripper
    called *bool
}

func (t *trackingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    *t.called = true
    return t.inner.RoundTrip(req)
}
```

Note: The test needs `net/http` and `github.com/prometheus/client_golang/api` as imports.

## Dependencies & Side Effects

- **Public API change?** No — `NewAlertmanagerClient(api.Config)` signature unchanged
- **Config / env var change?** No
- **Database migration?** No
- **Downstream consumer impact?** No — internal behavior change only (correctly uses RoundTripper now)
- **Error handling change?** No

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| RoundTripper nil for plain HTTP callers | Medium | None | Explicit nil check falls back to `http.DefaultTransport` |
| `httptransport.NewWithClient` unavailable | Very Low | High | Verified present via `go doc github.com/go-openapi/runtime/client.NewWithClient` |

## Audit Trail

- **Audit**: Skipped per `AUDIT_SKIP_SIMPLE=true` — all confidence HIGH, ≤2 files, <20 lines changed, signal is `default` (rule 5)
- **Root cause certainty**: HIGH — traced from `loader.go:53` through `alertmanager_api_client.go:NewHTTPClientWithConfig` which calls `httptransport.New` without any RoundTripper parameter
- **Approach correctness**: HIGH — `httptransport.NewWithClient` is the documented go-openapi API for custom HTTP client injection
- **Scope completeness**: HIGH — all 2 call sites examined; bug is entirely in the callee
