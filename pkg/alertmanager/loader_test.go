package alertmanager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/client_golang/api"
)

// mockAlertmanagerAPI is a mock implementation of the Alertmanager Loader interface
type mockAlertmanagerAPI struct {
	getAlertsFunc   func(ctx context.Context, active, silenced, inhibited, unprocessed *bool, filter []string, receiver string) (models.GettableAlerts, error)
	getSilencesFunc func(ctx context.Context, filter []string) (models.GettableSilences, error)
}

func (m *mockAlertmanagerAPI) GetAlerts(ctx context.Context, active, silenced, inhibited, unprocessed *bool, filter []string, receiver string) (models.GettableAlerts, error) {
	if m.getAlertsFunc != nil {
		return m.getAlertsFunc(ctx, active, silenced, inhibited, unprocessed, filter, receiver)
	}
	return models.GettableAlerts{}, nil
}

func (m *mockAlertmanagerAPI) GetSilences(ctx context.Context, filter []string) (models.GettableSilences, error) {
	if m.getSilencesFunc != nil {
		return m.getSilencesFunc(ctx, filter)
	}
	return models.GettableSilences{}, nil
}

// Ensure mockAlertmanagerAPI implements Loader at compile time
var _ Loader = (*mockAlertmanagerAPI)(nil)

func TestGetAlerts(t *testing.T) {
	t.Run("Get all alerts", func(t *testing.T) {
		activeState := "active"

		expectedAlerts := models.GettableAlerts{
			&models.GettableAlert{
				Alert: models.Alert{
					Labels: models.LabelSet{
						"alertname": "HighCPU",
						"severity":  "warning",
					},
				},
				Status: &models.AlertStatus{
					State:       &activeState,
					SilencedBy:  []string{},
					InhibitedBy: []string{},
				},
			},
		}

		mock := &mockAlertmanagerAPI{
			getAlertsFunc: func(ctx context.Context, active, silenced, inhibited, unprocessed *bool, filter []string, receiver string) (models.GettableAlerts, error) {
				return expectedAlerts, nil
			},
		}

		alerts, err := mock.GetAlerts(context.TODO(), nil, nil, nil, nil, nil, "")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if len(alerts) != 1 {
			t.Errorf("expected 1 alert, got %d", len(alerts))
		}
	})

	t.Run("Get active alerts only", func(t *testing.T) {
		active := true
		activeState := "active"

		expectedAlerts := models.GettableAlerts{
			&models.GettableAlert{
				Alert: models.Alert{
					Labels: models.LabelSet{
						"alertname": "HighCPU",
					},
				},
				Status: &models.AlertStatus{
					State:       &activeState,
					SilencedBy:  []string{},
					InhibitedBy: []string{},
				},
			},
		}

		mock := &mockAlertmanagerAPI{
			getAlertsFunc: func(ctx context.Context, activeParam, silenced, inhibited, unprocessed *bool, filter []string, receiver string) (models.GettableAlerts, error) {
				if activeParam == nil || !*activeParam {
					t.Error("expected active parameter to be true")
				}
				return expectedAlerts, nil
			},
		}

		alerts, err := mock.GetAlerts(context.TODO(), &active, nil, nil, nil, nil, "")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if len(alerts) != 1 {
			t.Errorf("expected 1 alert, got %d", len(alerts))
		}
	})

	t.Run("Get alerts with filter", func(t *testing.T) {
		filter := []string{"alertname=HighCPU"}
		activeState := "active"

		expectedAlerts := models.GettableAlerts{
			&models.GettableAlert{
				Alert: models.Alert{
					Labels: models.LabelSet{
						"alertname": "HighCPU",
					},
				},
				Status: &models.AlertStatus{
					State:       &activeState,
					SilencedBy:  []string{},
					InhibitedBy: []string{},
				},
			},
		}

		mock := &mockAlertmanagerAPI{
			getAlertsFunc: func(ctx context.Context, active, silenced, inhibited, unprocessed *bool, filterParam []string, receiver string) (models.GettableAlerts, error) {
				if len(filterParam) != 1 || filterParam[0] != "alertname=HighCPU" {
					t.Errorf("expected filter 'alertname=HighCPU', got %v", filterParam)
				}
				return expectedAlerts, nil
			},
		}

		alerts, err := mock.GetAlerts(context.TODO(), nil, nil, nil, nil, filter, "")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if len(alerts) != 1 {
			t.Errorf("expected 1 alert, got %d", len(alerts))
		}
	})

	t.Run("Get alerts with receiver", func(t *testing.T) {
		receiver := "team-notifications"
		activeState := "active"

		expectedAlerts := models.GettableAlerts{
			&models.GettableAlert{
				Alert: models.Alert{
					Labels: models.LabelSet{
						"alertname": "HighCPU",
					},
				},
				Status: &models.AlertStatus{
					State:       &activeState,
					SilencedBy:  []string{},
					InhibitedBy: []string{},
				},
			},
		}

		mock := &mockAlertmanagerAPI{
			getAlertsFunc: func(ctx context.Context, active, silenced, inhibited, unprocessed *bool, filter []string, receiverParam string) (models.GettableAlerts, error) {
				if receiverParam != "team-notifications" {
					t.Errorf("expected receiver 'team-notifications', got %s", receiverParam)
				}
				return expectedAlerts, nil
			},
		}

		alerts, err := mock.GetAlerts(context.TODO(), nil, nil, nil, nil, nil, receiver)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if len(alerts) != 1 {
			t.Errorf("expected 1 alert, got %d", len(alerts))
		}
	})
}

func TestGetSilences(t *testing.T) {
	t.Run("Get all silences", func(t *testing.T) {
		silenceID := "test-silence-id"
		silenceState := models.SilenceStatusStateActive

		expectedSilences := models.GettableSilences{
			&models.GettableSilence{
				ID: &silenceID,
				Status: &models.SilenceStatus{
					State: &silenceState,
				},
				Silence: models.Silence{
					Matchers: models.Matchers{
						&models.Matcher{
							Name:    ptrString("alertname"),
							Value:   ptrString("HighCPU"),
							IsRegex: ptrBool(false),
							IsEqual: ptrBool(true),
						},
					},
					CreatedBy: ptrString("admin"),
					Comment:   ptrString("Maintenance window"),
				},
			},
		}

		mock := &mockAlertmanagerAPI{
			getSilencesFunc: func(ctx context.Context, filter []string) (models.GettableSilences, error) {
				return expectedSilences, nil
			},
		}

		silences, err := mock.GetSilences(context.TODO(), nil)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if len(silences) != 1 {
			t.Errorf("expected 1 silence, got %d", len(silences))
		}
	})

	t.Run("Get silences with filter", func(t *testing.T) {
		filter := []string{"alertname=HighCPU"}
		silenceID := "test-silence-id"
		silenceState := models.SilenceStatusStateActive

		expectedSilences := models.GettableSilences{
			&models.GettableSilence{
				ID: &silenceID,
				Status: &models.SilenceStatus{
					State: &silenceState,
				},
				Silence: models.Silence{
					Matchers: models.Matchers{
						&models.Matcher{
							Name:    ptrString("alertname"),
							Value:   ptrString("HighCPU"),
							IsRegex: ptrBool(false),
							IsEqual: ptrBool(true),
						},
					},
					CreatedBy: ptrString("admin"),
					Comment:   ptrString("Planned maintenance"),
				},
			},
		}

		mock := &mockAlertmanagerAPI{
			getSilencesFunc: func(ctx context.Context, filterParam []string) (models.GettableSilences, error) {
				if len(filterParam) != 1 || filterParam[0] != "alertname=HighCPU" {
					t.Errorf("expected filter 'alertname=HighCPU', got %v", filterParam)
				}
				return expectedSilences, nil
			},
		}

		silences, err := mock.GetSilences(context.TODO(), filter)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if len(silences) != 1 {
			t.Errorf("expected 1 silence, got %d", len(silences))
		}
	})

	t.Run("Get empty silences list", func(t *testing.T) {
		mock := &mockAlertmanagerAPI{
			getSilencesFunc: func(ctx context.Context, filter []string) (models.GettableSilences, error) {
				return models.GettableSilences{}, nil
			},
		}

		silences, err := mock.GetSilences(context.TODO(), nil)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if len(silences) != 0 {
			t.Errorf("expected 0 silences, got %d", len(silences))
		}
	})
}

// TestNewAlertmanagerClientTransport verifies that NewAlertmanagerClient injects
// the RoundTripper from apiConfig into the underlying HTTP transport. This is a
// regression test for the bug where client.NewHTTPClientWithConfig discarded the
// configured RoundTripper (TLS settings and bearer token auth).
func TestNewAlertmanagerClientTransport(t *testing.T) {
	const wantToken = "test-bearer-token"
	transportInvoked := false

	// Create a test server that requires a bearer token
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+wantToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Respond with a valid (empty) alerts JSON payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	// Custom RoundTripper that adds the bearer token and records invocation
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		transportInvoked = true
		req.Header.Set("Authorization", "Bearer "+wantToken)
		return http.DefaultTransport.RoundTrip(req)
	})

	loader, err := NewAlertmanagerClient(api.Config{
		Address:      srv.URL,
		RoundTripper: rt,
	})
	if err != nil {
		t.Fatalf("NewAlertmanagerClient returned error: %v", err)
	}

	// Call GetAlerts — if the RoundTripper is ignored we get a 401 error
	_, err = loader.GetAlerts(context.Background(), nil, nil, nil, nil, nil, "")
	if err != nil {
		t.Errorf("GetAlerts returned unexpected error (RoundTripper may not be applied): %v", err)
	}
	if !transportInvoked {
		t.Error("custom RoundTripper was never invoked — transport injection is broken")
	}
}

// roundTripperFunc allows using a plain function as an http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Helper functions to create pointers
func ptrString(s string) *string {
	return &s
}

func ptrBool(b bool) *bool {
	return &b
}
