package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

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

	slog.Info("generate request received", "date", req.Date)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	result, err := s.generator.Run(ctx, req.Date)
	if err != nil {
		slog.Warn("generate request failed", "date", req.Date, "error", err.Error())
		writeJSON(w, statusForGenerateError(err), errorResponse{Success: false, Error: err.Error()})
		return
	}
	slog.Info("generate request succeeded", "date", result.Date, "file", result.File)
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
