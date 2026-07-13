package web

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

type contextKey string

const guestKey contextKey = "guest"

func isGuest(r *http.Request) bool {
	v, _ := r.Context().Value(guestKey).(bool)
	return v
}

func markGuest(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), guestKey, true))
}

func AuthMiddleware(password string, guestEnabled bool) func(http.Handler) http.Handler {
	if password == "" {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	expectedHash := sha256.Sum256([]byte(password))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/login" || r.URL.Path == "/login.html" {
				next.ServeHTTP(w, r)
				return
			}

			if isStaticFile(r.URL.Path) || r.URL.Path == "/" || r.URL.Path == "/index.html" {
				token := r.URL.Query().Get("token")
				if token != "" {
					hash := sha256.Sum256([]byte(token))
					if subtle.ConstantTimeCompare(hash[:], expectedHash[:]) == 1 {
						http.SetCookie(w, &http.Cookie{
							Name:  "auth_token",
							Value: token,
							Path:  "/",
						})
						next.ServeHTTP(w, r)
						return
					}
				}

				cookie, err := r.Cookie("auth_token")
				if err == nil {
					hash := sha256.Sum256([]byte(cookie.Value))
					if subtle.ConstantTimeCompare(hash[:], expectedHash[:]) == 1 {
						next.ServeHTTP(w, r)
						return
					}
				}

				if guestEnabled {
					next.ServeHTTP(w, markGuest(r))
					return
				}

				http.Redirect(w, r, "/login.html", http.StatusTemporaryRedirect)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				hash := sha256.Sum256([]byte(token))
				if subtle.ConstantTimeCompare(hash[:], expectedHash[:]) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}

			if guestEnabled && r.Method == http.MethodGet && isGuestAllowedPath(r.URL.Path) {
				next.ServeHTTP(w, markGuest(r))
				return
			}

			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		})
	}
}

func isGuestAllowedPath(path string) bool {
	allowed := []string{
		"/api/status",
		"/api/update",
		"/api/stats",
		"/api/stats/daily",
		"/api/probes",
		"/api/providers",
		"/api/export/csv",
	}
	for _, a := range allowed {
		if path == a {
			return true
		}
	}
	if strings.HasPrefix(path, "/api/probes/") &&
		(strings.HasSuffix(path, "/hourly") || strings.HasSuffix(path, "/daily")) {
		return true
	}
	return false
}

func isStaticFile(path string) bool {
	staticExts := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf"}
	for _, ext := range staticExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
