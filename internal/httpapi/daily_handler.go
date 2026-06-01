package httpapi

import (
	"errors"
	"net/http"

	"m-daily-news/internal/daily"
)

type listResponse struct {
	Items []daily.Item `json:"items"`
	Total int          `json:"total"`
}

func (s *Server) ListDaily(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Total: len(items)})
}

func (s *Server) GetDailyRaw(w http.ResponseWriter, r *http.Request) {
	date := r.PathValue("date")
	data, err := s.store.ReadRaw(date)
	if err != nil {
		status := http.StatusNotFound
		if errors.Is(err, daily.ErrInvalidDate) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, errorResponse{Success: false, Error: err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
