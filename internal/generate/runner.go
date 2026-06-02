package generate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/llm"
	"m-daily-news/internal/search"
)

type Searcher interface {
	SearchDailySources(ctx context.Context, date string) ([]search.Result, error)
}

type Runner struct {
	store    *daily.Store
	searcher Searcher
	llm      llm.Client
	mu       sync.Mutex
	status   Status
	now      func() time.Time
}

var ErrRunning = errors.New("generation already running")

func NewRunner(store *daily.Store, searcher Searcher, llmClient llm.Client) *Runner {
	return &Runner{store: store, searcher: searcher, llm: llmClient, now: time.Now}
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	status := r.status
	r.mu.Unlock()

	status.TodayReady = false
	if !status.Running {
		today, err := daily.NormalizeDate("", r.now())
		if err == nil {
			status.TodayDate = today
			ready, err := r.store.Exists(today)
			status.TodayReady = err == nil && ready
		}
	}
	return status
}

func (r *Runner) Run(ctx context.Context, rawDate string) (*Result, error) {
	start := r.now()
	date, err := daily.NormalizeDate(rawDate, r.now())
	if err != nil {
		return nil, err
	}
	slog.Info("daily generation started", "date", date)

	r.mu.Lock()
	if r.status.Running {
		r.mu.Unlock()
		slog.Warn("daily generation rejected", "date", date, "reason", "already_running")
		return nil, ErrRunning
	}
	r.status.Running = true
	r.status.LastError = ""
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.status.Running = false
		lastRun := r.now()
		r.status.LastRun = &lastRun
		r.mu.Unlock()
	}()

	if exists, err := r.store.Exists(date); err != nil {
		r.fail(err)
		slog.Error("daily generation failed", "date", date, "stage", "exists", "error", err.Error())
		return nil, err
	} else if exists {
		r.fail(daily.ErrExists)
		slog.Warn("daily generation skipped", "date", date, "reason", "already_exists")
		return nil, daily.ErrExists
	}

	searchStart := r.now()
	results, err := r.searcher.SearchDailySources(ctx, date)
	if err != nil {
		err = fmt.Errorf("search: %w", err)
		r.fail(err)
		slog.Error("daily generation failed", "date", date, "stage", "search", "duration", r.now().Sub(searchStart).String(), "error", err.Error())
		return nil, err
	}
	slog.Info("daily generation search completed", "date", date, "results", len(results), "duration", r.now().Sub(searchStart).String())

	llmStart := r.now()
	markdown, err := r.llm.WriteDaily(ctx, date, results)
	if err != nil {
		err = fmt.Errorf("llm: %w", err)
		r.fail(err)
		slog.Error("daily generation failed", "date", date, "stage", "llm", "duration", r.now().Sub(llmStart).String(), "error", err.Error())
		return nil, err
	}
	slog.Info("daily generation llm completed", "date", date, "bytes", len(markdown), "duration", r.now().Sub(llmStart).String())

	writeStart := r.now()
	file, err := r.store.WriteValidated(date, markdown)
	if err != nil {
		r.fail(err)
		slog.Error("daily generation failed", "date", date, "stage", "write", "duration", r.now().Sub(writeStart).String(), "error", err.Error())
		return nil, err
	}
	slog.Info("daily generation write completed", "date", date, "file", file, "duration", r.now().Sub(writeStart).String())

	result := &Result{
		Date:    date,
		File:    file,
		Summary: daily.ExtractSummary(markdown),
	}
	r.mu.Lock()
	r.status.LastSuccess = true
	r.status.LastFile = file
	r.status.LastError = ""
	r.mu.Unlock()
	slog.Info("daily generation completed", "date", date, "file", file, "duration", r.now().Sub(start).String())
	return result, nil
}

func (r *Runner) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.LastSuccess = false
	r.status.LastError = err.Error()
}
