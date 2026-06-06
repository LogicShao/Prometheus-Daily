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
	"m-daily-news/internal/reportmode"
)

type generateRequest struct {
	Date string `json:"date"`
	Mode string `json:"mode"`
}

type generateResponse struct {
	Success  bool   `json:"success"`
	Date     string `json:"date,omitempty"`
	File     string `json:"file,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Attempts int    `json:"attempts,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Error    string `json:"error,omitempty"`
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
	mode, err := s.requestMode(req.Mode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
		return
	}

	slog.Info("generate request received", "date", req.Date, "report_mode", mode)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := s.generator.RunWithOptions(ctx, req.Date, generate.Options{Mode: mode})
	if err != nil {
		slog.Warn("generate request failed", "date", req.Date, "report_mode", mode, "error", err.Error())
		writeJSON(w, statusForGenerateError(err), errorResponse{Success: false, Error: err.Error()})
		return
	}
	slog.Info("generate request succeeded", "date", result.Date, "file", result.File, "report_mode", result.Mode)
	writeJSON(w, http.StatusOK, generateResponse{
		Success:  true,
		Date:     result.Date,
		File:     result.File,
		Summary:  result.Summary,
		Attempts: result.Attempts,
		Mode:     result.Mode,
	})
}

func (s *Server) RerunToday(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: "invalid json body"})
			return
		}
	}
	mode, err := s.requestMode(req.Mode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Success: false, Error: err.Error()})
		return
	}

	slog.Info("generate rerun request received", "report_mode", mode)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := s.generator.RerunTodayWithOptions(ctx, generate.Options{Mode: mode})
	if err != nil {
		slog.Warn("generate rerun request failed", "report_mode", mode, "error", err.Error())
		writeJSON(w, statusForGenerateError(err), errorResponse{Success: false, Error: err.Error()})
		return
	}
	slog.Info("generate rerun request succeeded", "date", result.Date, "file", result.File, "report_mode", result.Mode)
	writeJSON(w, http.StatusOK, generateResponse{
		Success:  true,
		Date:     result.Date,
		File:     result.File,
		Summary:  result.Summary,
		Attempts: result.Attempts,
		Mode:     result.Mode,
	})
}

func (s *Server) requestMode(raw string) (reportmode.Mode, error) {
	if raw == "" {
		return s.reportMode, nil
	}
	mode, err := reportmode.Normalize(raw)
	if err != nil {
		return "", errors.New("mode must be one of: " + reportmode.AllowedValues())
	}
	return mode, nil
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
