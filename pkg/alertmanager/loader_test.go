package alertmanager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/alertmanager/api/v2/client/alert"
	"github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/client_golang/api"
	alertmanagerClient "github.com/prometheus/alertmanager/api/v2/client"
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

// Helper functions to create pointers
func ptrString(s string) *string {
	return &s
}

func ptrBool(b bool) *bool {
	return &b
}

// TestNewAlertmanagerClientUsesRoundTripper verifies that NewAlertmanagerClient
// correctly uses the RoundTripper from api.Config instead of ignoring it.
func TestNewAlertmanagerClientUsesRoundTripper(t *testing.T) {
	// Create a test server
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create a custom RoundTripper that tracks if it was called
	customRoundTripper := &trackingRoundTripper{
		base: testServer.Client().Transport,
	}

	// Create api.Config with our custom RoundTripper
	apiConfig := api.Config{
		Address:      testServer.URL,
		RoundTripper: customRoundTripper,
	}

	// Create the Alertmanager client
	loader, err := NewAlertmanagerClient(apiConfig)
	if err != nil {
		t.Fatalf("failed to create Alertmanager client: %v", err)
	}

	// Verify the client was created with our custom RoundTripper
	if loader == nil {
		t.Fatal("expected loader to be non-nil")
	}

	// Verify that the internal client uses our custom RoundTripper
	// We can check this by inspecting the transport chain
	// The fix should propagate our RoundTripper to the underlying HTTP client
	verifyRoundTripperUsed(loader.client, customRoundTripper, t)
}

// verifyRoundTripperUsed verifies the custom RoundTripper is used in the client
func verifyRoundTripperUsed(client *alertmanagerClient.AlertmanagerAPI, expected *trackingRoundTripper, t *testing.T) {
	// The go-openapi generated client embeds the transport
	// We need to verify that our custom RoundTripper is in the chain
	// Since we can't directly access the internal transport, we test behavior instead

	// Make a request that should use our custom RoundTripper
	// This will fail due to TLS but proves the RoundTripper was invoked
	ctx := context.Background()
	_, _ = client.Alert.GetAlerts(alert.NewGetAlertsParams().WithContext(ctx))

	// If the custom RoundTripper was called during request setup, the fix works
	if !expected.called {
		t.Error("expected custom RoundTripper to be used during HTTP client creation")
	}
}

// trackingRoundTripper is a RoundTripper that wraps another RoundTripper
// and tracks whether it was used.
type trackingRoundTripper struct {
	base    http.RoundTripper
	called bool
}

func (t *trackingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	t.called = true
	return t.base.RoundTrip(req)
}
