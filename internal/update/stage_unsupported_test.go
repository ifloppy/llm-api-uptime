//go:build !linux && !darwin

package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStageUnsupported(t *testing.T) {
	checker := NewChecker(WithCurrentVersion("v1.0.0"))
	status := checker.Stage(context.Background())
	if status.State != StateUnsupported {
		t.Fatalf("Stage().State = %q, want %q", status.State, StateUnsupported)
	}
	if status.Error == "" {
		t.Fatal("Stage() did not include an unsupported notice")
	}
}

func TestAutomaticCheckPreservesAvailableReleaseOnUnsupportedPlatform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"tag_name":"v1.1.0","html_url":"https://github.com/ifloppy/llm-api-uptime/releases/tag/v1.1.0","published_at":"2026-07-01T12:00:00Z","assets":[]}`))
	}))
	defer server.Close()

	checker := NewChecker(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithCurrentVersion("v1.0.0"),
		WithAutoStage(true),
	)
	checker.checkAndStage(context.Background())
	status := checker.Status()
	if status.State != StateAvailable || status.Latest != "v1.1.0" || status.ReleaseURL == "" {
		t.Fatalf("available release metadata was lost: %#v", status)
	}
	if status.Error == "" {
		t.Fatal("unsupported auto-stage notice was not retained")
	}
}
