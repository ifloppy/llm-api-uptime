package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

func AuthMiddleware(password string) func(http.Handler) http.Handler {
	if password == "" {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	expectedHash := sha256.Sum256([]byte(password))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/login" {
				next.ServeHTTP(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/" || r.URL.Path == "/index.html" {
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

				http.Redirect(w, r, "/login.html", http.StatusTemporaryRedirect)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			hash := sha256.Sum256([]byte(token))
			if subtle.ConstantTimeCompare(hash[:], expectedHash[:]) != 1 {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
