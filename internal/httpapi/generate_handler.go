package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
)

type generateRequest struct {
	Date string `json:"date"`
}

type generateResponse struct {
	Success bool   `json:"success"`
	Date    string `json:"date,omitempty"`
	File    string `json:"file,omitempty"`
	Summary string `json:"summary,omitempty"`
	Error   string `json:"error,omitempty"`
}

type errorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func (s *Server) Generate(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: "invalid json body"})
			return
		}
	}

	result, err := s.generator.Run(r.Context(), req.Date)
	if err != nil {
		writeJSON(w, statusForGenerateError(err), errorResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, generateResponse{
		Success: true,
		Date:    result.Date,
		File:    result.File,
		Summary: result.Summary,
	})
}

func statusForGenerateError(err error) int {
	switch {
	case errors.Is(err, daily.ErrInvalidDate):
		return http.StatusBadRequest
	case errors.Is(err, daily.ErrExists), errors.Is(err, generate.ErrRunning):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
