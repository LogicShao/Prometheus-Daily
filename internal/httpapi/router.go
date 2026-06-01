package httpapi

import (
	"context"
	"net/http"
	"time"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
)

type Generator interface {
	Run(ctx context.Context, rawDate string) (*generate.Result, error)
	Status() generate.Status
}

type Server struct {
	store      *daily.Store
	generator  Generator
	adminToken string
	workspace  string
	startedAt  time.Time
}

func NewRouter(store *daily.Store, generator Generator, adminToken, workspace string, startedAt time.Time) http.Handler {
	s := &Server{
		store:      store,
		generator:  generator,
		adminToken: adminToken,
		workspace:  workspace,
		startedAt:  startedAt,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.Index)
	mux.HandleFunc("GET /admin", s.AdminIndex)
	mux.HandleFunc("GET /api/daily", s.ListDaily)
	mux.HandleFunc("GET /api/daily/{date}", s.GetDaily)
	mux.HandleFunc("GET /api/daily/{date}/raw", s.GetDailyRaw)
	mux.HandleFunc("POST /api/generate", requireAdmin(adminToken, s.Generate))
	mux.HandleFunc("GET /api/status", s.Status)
	mux.HandleFunc("GET /health", s.Health)
	return mux
}
