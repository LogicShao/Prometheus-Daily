package httpapi

import (
	"context"
	"net/http"
	"time"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
	"m-daily-news/internal/reportmode"
)

type Generator interface {
	Run(ctx context.Context, rawDate string) (*generate.Result, error)
	RunWithOptions(ctx context.Context, rawDate string, opts generate.Options) (*generate.Result, error)
	RerunToday(ctx context.Context) (*generate.Result, error)
	RerunTodayWithOptions(ctx context.Context, opts generate.Options) (*generate.Result, error)
	Status() generate.Status
}

type Server struct {
	store      *daily.Store
	generator  Generator
	adminToken string
	workspace  string
	startedAt  time.Time
	reportMode reportmode.Mode
}

func NewRouter(store *daily.Store, generator Generator, adminToken, workspace string, startedAt time.Time) http.Handler {
	return NewRouterWithMode(store, generator, adminToken, workspace, startedAt, reportmode.Balanced)
}

func NewRouterWithMode(store *daily.Store, generator Generator, adminToken, workspace string, startedAt time.Time, mode reportmode.Mode) http.Handler {
	s := &Server{
		store:      store,
		generator:  generator,
		adminToken: adminToken,
		workspace:  workspace,
		startedAt:  startedAt,
		reportMode: reportmode.Default(mode),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.Index)
	mux.HandleFunc("GET /admin", s.AdminIndex)
	mux.HandleFunc("GET /about", s.AboutIndex)
	mux.HandleFunc("GET /feed.xml", s.Feed)
	mux.HandleFunc("GET /rss.xml", s.Feed)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/daily", s.ListDaily)
	mux.HandleFunc("GET /api/daily/{date}", s.GetDaily)
	mux.HandleFunc("GET /api/daily/{date}/raw", s.GetDailyRaw)
	mux.HandleFunc("POST /api/generate", requireAdmin(adminToken, s.Generate))
	mux.HandleFunc("POST /api/generate/rerun", requireAdmin(adminToken, s.RerunToday))
	mux.HandleFunc("GET /api/status", s.Status)
	mux.HandleFunc("GET /health", s.Health)
	return requestLog(mux)
}
