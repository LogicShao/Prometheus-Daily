package httpapi

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
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

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"bytes", ww.bytes,
			"duration", time.Since(start).String(),
			"remote", remoteAddr(r),
		}
		if ww.status >= http.StatusInternalServerError {
			slog.Error("http request completed", attrs...)
			return
		}
		if ww.status >= http.StatusBadRequest {
			slog.Warn("http request completed", attrs...)
			return
		}
		slog.Info("http request completed", attrs...)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func remoteAddr(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		if first := strings.TrimSpace(strings.Split(forwardedFor, ",")[0]); first != "" {
			return first
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
