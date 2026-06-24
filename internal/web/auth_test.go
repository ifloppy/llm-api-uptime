package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddlewareDisabled(t *testing.T) {
	middleware := AuthMiddleware("", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/providers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddlewareBearerToken(t *testing.T) {
	middleware := AuthMiddleware("test-password", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/providers", nil)
	req.Header.Set("Authorization", "Bearer test-password")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	middleware := AuthMiddleware("test-password", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/providers", nil)
	req.Header.Set("Authorization", "Bearer wrong-password")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareMissingAuth(t *testing.T) {
	middleware := AuthMiddleware("test-password", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/providers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareLoginEndpoint(t *testing.T) {
	middleware := AuthMiddleware("test-password", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for login endpoint, got %d", rec.Code)
	}
}

func TestAuthMiddlewareStaticFiles(t *testing.T) {
	middleware := AuthMiddleware("test-password", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name string
		path string
	}{
		{"index", "/"},
		{"index.html", "/index.html"},
		{"css", "/static/style.css"},
		{"js", "/static/app.js"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusTemporaryRedirect {
				t.Errorf("expected 307 redirect for %s, got %d", tt.path, rec.Code)
			}
			if rec.Header().Get("Location") != "/login.html" {
				t.Errorf("expected redirect to /login.html, got %s", rec.Header().Get("Location"))
			}
		})
	}
}

func TestAuthMiddlewareCookieAuth(t *testing.T) {
	middleware := AuthMiddleware("test-password", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: "test-password",
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with valid cookie, got %d", rec.Code)
	}
}

func TestAuthMiddlewareTokenQueryParam(t *testing.T) {
	middleware := AuthMiddleware("test-password", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/?token=test-password", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token param, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "auth_token" && c.Value == "test-password" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth_token cookie to be set")
	}
}

func TestGuestModeAllowedPaths(t *testing.T) {
	middleware := AuthMiddleware("test-password", true)

	paths := []string{
		"/api/status",
		"/api/stats",
		"/api/probes",
		"/api/providers",
		"/api/export/csv",
		"/api/probes/42/hourly",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			var gotGuest bool
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotGuest = isGuest(r)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 for guest path %s, got %d", path, rec.Code)
			}
			if !gotGuest {
				t.Errorf("expected guest context for %s", path)
			}
		})
	}
}

func TestGuestModeBlocksWritePaths(t *testing.T) {
	middleware := AuthMiddleware("test-password", true)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	paths := []string{
		"/api/probes/42/results",
		"/api/results/1",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 for %s, got %d", path, rec.Code)
			}
		})
	}
}

func TestGuestModeBlocksPostRequests(t *testing.T) {
	middleware := AuthMiddleware("test-password", true)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/providers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for POST without auth, got %d", rec.Code)
	}
}

func TestGuestModeStaticFilesAllowed(t *testing.T) {
	middleware := AuthMiddleware("test-password", true)

	paths := []string{"/", "/index.html", "/style.css", "/app.js"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			var gotGuest bool
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotGuest = isGuest(r)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d", path, rec.Code)
			}
			if !gotGuest {
				t.Errorf("expected guest context for %s", path)
			}
		})
	}
}

func TestGuestModeDisabledBlocksStaticFiles(t *testing.T) {
	middleware := AuthMiddleware("test-password", false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected 307 redirect when guest disabled, got %d", rec.Code)
	}
}

func TestGuestModeNoPasswordNoChange(t *testing.T) {
	middleware := AuthMiddleware("", true)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/providers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when no password set, got %d", rec.Code)
	}
}
