package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func requireAdmin(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Success: false, Error: "admin token not configured"})
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Success: false, Error: "unauthorized"})
			return
		}
		next(w, r)
	}
}
