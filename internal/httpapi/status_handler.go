package httpapi

import (
	"net/http"
	"time"
)

func (s *Server) Status(w http.ResponseWriter, r *http.Request) {
	status := s.generator.Status()
	status.Uptime = time.Since(s.startedAt).String()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
