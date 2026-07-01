package alertmanager

import (
	"context"
	"net/http"
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

// roundTripperFunc is a simple adapter to use a function as an http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewAlertmanagerClient_UsesRoundTripper(t *testing.T) {
	t.Run("custom RoundTripper is wired into client", func(t *testing.T) {
		called := false
		customRT := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			return &http.Response{StatusCode: http.StatusOK}, nil
		})

		cfg := api.Config{
			Address:      "https://alertmanager.example.com",
			RoundTripper: customRT,
		}

		loader, err := NewAlertmanagerClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if loader == nil {
			t.Fatal("expected non-nil loader")
		}
		if loader.client == nil {
			t.Fatal("expected non-nil client")
		}

		// Attempt a request to verify the custom RoundTripper is used.
		// The request will go through our custom RoundTripper which sets called=true.
		// We don't care about the result — just that our transport was invoked.
		_, _ = loader.GetAlerts(context.Background(), nil, nil, nil, nil, nil, "")
		if !called {
			t.Error("expected custom RoundTripper to be called, but it was not")
		}
	})

	t.Run("nil RoundTripper falls back to default client", func(t *testing.T) {
		cfg := api.Config{
			Address:      "http://alertmanager.example.com",
			RoundTripper: nil,
		}

		loader, err := NewAlertmanagerClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if loader == nil {
			t.Fatal("expected non-nil loader")
		}
		if loader.client == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("invalid URL returns error", func(t *testing.T) {
		cfg := api.Config{
			Address: "://invalid",
		}

		_, err := NewAlertmanagerClient(cfg)
		if err == nil {
			t.Fatal("expected error for invalid URL")
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
