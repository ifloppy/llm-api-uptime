package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckerCheck(t *testing.T) {
	tests := []struct {
		name    string
		current string
		tag     string
		draft   bool
		pre     bool
		state   State
	}{
		{name: "available", current: "v1.2.2", tag: "v1.2.3", state: StateAvailable},
		{name: "equal", current: "1.2.3", tag: "v1.2.3", state: StateUpToDate},
		{name: "newer local", current: "2.0.0", tag: "v1.2.3", state: StateUpToDate},
		{name: "draft rejected", current: "1.2.2", tag: "v1.2.3", draft: true, state: StateError},
		{name: "prerelease rejected", current: "1.2.2", tag: "v1.2.3", pre: true, state: StateError},
		{name: "prerelease tag rejected", current: "1.2.2", tag: "v1.2.3-rc.1", state: StateError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path != latestReleasePath {
					http.NotFound(response, request)
					return
				}
				if request.Header.Get("User-Agent") != "llm-api-uptime-updater" {
					t.Errorf("User-Agent = %q", request.Header.Get("User-Agent"))
				}
				fmt.Fprintf(response, `{"tag_name":%q,"html_url":%q,"draft":%t,"prerelease":%t,"published_at":"2026-07-01T12:00:00Z","assets":[]}`,
					test.tag, server.URL+"/release", test.draft, test.pre)
			}))
			defer server.Close()

			checker := NewChecker(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithCurrentVersion(test.current))
			status := checker.Check(context.Background())
			if status.State != test.state {
				t.Fatalf("Check().State = %q, want %q (error %q)", status.State, test.state, status.Error)
			}
			if status.Current != test.current {
				t.Errorf("Current = %q, want %q", status.Current, test.current)
			}
			if status.CheckedAt.IsZero() || status.CheckStartedAt.IsZero() {
				t.Error("check timestamps were not populated")
			}
		})
	}
}

func TestCheckerCheckHTTPAndPayloadErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "HTTP error", status: http.StatusForbidden, body: "rate limited"},
		{name: "invalid JSON", status: http.StatusOK, body: "{"},
		{name: "oversized", status: http.StatusOK, body: strings.Repeat("x", metadataLimit+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(test.status)
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()

			status := NewChecker(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithCurrentVersion("1.0.0")).Check(context.Background())
			if status.State != StateError || status.Error == "" {
				t.Fatalf("Check() = %#v, want error state", status)
			}
		})
	}
}

func TestParseChecksum(t *testing.T) {
	digest := sha256.Sum256([]byte("binary"))
	hexDigest := hex.EncodeToString(digest[:])
	data := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  other\n" + hexDigest + " *wanted\r\n")
	got, err := ParseChecksum(data, "wanted")
	if err != nil {
		t.Fatal(err)
	}
	if got != digest {
		t.Fatalf("ParseChecksum() = %x, want %x", got, digest)
	}

	for _, test := range []struct {
		name string
		data string
	}{
		{name: "missing", data: hexDigest + "  other\n"},
		{name: "malformed", data: "not-a-checksum wanted\n"},
		{name: "duplicate", data: hexDigest + "  wanted\n" + hexDigest + "  wanted\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseChecksum([]byte(test.data), "wanted"); err == nil {
				t.Fatal("ParseChecksum() succeeded, want error")
			}
		})
	}
}

func TestCheckerStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(response, `{"tag_name":"v1.1.0","html_url":%q,"published_at":"2026-07-01T12:00:00Z","assets":[]}`, "https://example.com/release")
	}))
	defer server.Close()
	checker := NewChecker(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithCurrentVersion("1.0.0"))
	checker.Start()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if checker.Status().State == StateAvailable {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Start did not complete: %#v", checker.Status())
}

func TestCheckerStartChecksPeriodicallyAndStops(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		fmt.Fprintf(response, `{"tag_name":"v1.0.0","html_url":%q,"published_at":"2026-07-01T12:00:00Z","assets":[]}`, "https://example.com/release")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	checker := NewChecker(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithCurrentVersion("1.0.0"),
		WithInterval(10*time.Millisecond),
	)
	checker.Start(ctx)
	deadline := time.Now().Add(time.Second)
	for requests.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if requests.Load() < 2 {
		t.Fatalf("requests = %d, want at least two", requests.Load())
	}
	cancel()
	checker.Wait()
	stoppedAt := requests.Load()
	time.Sleep(30 * time.Millisecond)
	if got := requests.Load(); got != stoppedAt {
		t.Fatalf("requests continued after cancellation: %d -> %d", stoppedAt, got)
	}
}

func TestCheckerStartAutoStagesAvailableRelease(t *testing.T) {
	if ok, _ := supportedTarget(); !ok {
		t.Skip("automatic staging is unsupported on this platform")
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(response, `{"tag_name":"v1.1.0","html_url":%q,"published_at":"2026-07-01T12:00:00Z","assets":[]}`, "https://example.com/release")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker := NewChecker(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithCurrentVersion("1.0.0"),
		WithInterval(time.Hour),
		WithAutoStage(true),
	)
	staged := make(chan struct{}, 1)
	checker.stage = func(context.Context) Status {
		staged <- struct{}{}
		return Status{State: StateRestartRequired, Current: "1.0.0", Latest: "v1.1.0"}
	}
	checker.Start(ctx)
	select {
	case <-staged:
	case <-time.After(time.Second):
		t.Fatal("available release was not staged")
	}
	cancel()
	checker.Wait()
}

func TestCheckerDevVersionIsUnsupportedWithoutRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(response, "unexpected", http.StatusInternalServerError)
	}))
	defer server.Close()

	status := NewChecker(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithCurrentVersion("dev")).Check(context.Background())
	if status.State != StateUnsupported || status.Error == "" {
		t.Fatalf("Check() = %#v, want clear unsupported status", status)
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want zero", requests.Load())
	}
}

func TestCheckerKeepsRestartRequiredState(t *testing.T) {
	checker := NewChecker(WithCurrentVersion("v1.0.0"), WithAutoStage(true))
	want := Status{
		State:      StateRestartRequired,
		Current:    "v1.0.0",
		Latest:     "v1.1.0",
		BackupPath: "/tmp/llm-api-uptime.old",
	}
	checker.setStatus(want)
	checker.checkAndStage(context.Background())

	if got := checker.Status(); got != want {
		t.Fatalf("status changed after staging: got %#v, want %#v", got, want)
	}
}
