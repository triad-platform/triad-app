package main

import "testing"

func TestNATSURL_Default(t *testing.T) {
	t.Setenv("NATS_URL", "")
	if got, want := natsURL(), "nats://localhost:4222"; got != want {
		t.Fatalf("default nats url mismatch: got=%q want=%q", got, want)
	}
}

func TestNATSURL_Override(t *testing.T) {
	t.Setenv("NATS_URL", "nats://example:4222")
	if got, want := natsURL(), "nats://example:4222"; got != want {
		t.Fatalf("override nats url mismatch: got=%q want=%q", got, want)
	}
}

func TestNotificationsURL_Default(t *testing.T) {
	t.Setenv("NOTIFICATIONS_URL", "")
	if got, want := notificationsURL(), "http://localhost:8082"; got != want {
		t.Fatalf("default notifications url mismatch: got=%q want=%q", got, want)
	}
}

func TestNotificationsURL_Override(t *testing.T) {
	t.Setenv("NOTIFICATIONS_URL", "http://notifications:8082")
	if got, want := notificationsURL(), "http://notifications:8082"; got != want {
		t.Fatalf("override notifications url mismatch: got=%q want=%q", got, want)
	}
}

func TestMetricsPort_Default(t *testing.T) {
	t.Setenv("WORKER_METRICS_PORT", "")
	if got, want := metricsPort(), "9091"; got != want {
		t.Fatalf("default metrics port mismatch: got=%q want=%q", got, want)
	}
}

func TestMetricsPort_Override(t *testing.T) {
	t.Setenv("WORKER_METRICS_PORT", "9999")
	if got, want := metricsPort(), "9999"; got != want {
		t.Fatalf("override metrics port mismatch: got=%q want=%q", got, want)
	}
}

func TestWorker_SubscriptionAndIdempotency_Pending(t *testing.T) {
	t.Skip("TODO(phase-1): add subscription, retry, and idempotency tests once worker processing is implemented")
}
