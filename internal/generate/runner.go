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
	return r.run(ctx, rawDate, false)
}

func (r *Runner) RerunToday(ctx context.Context) (*Result, error) {
	return r.run(ctx, "", true)
}

func (r *Runner) run(ctx context.Context, rawDate string, replace bool) (*Result, error) {
	start := r.now()
	date, err := daily.NormalizeDate(rawDate, r.now())
	if err != nil {
		return nil, err
	}
	mode := "create"
	if replace {
		mode = "replace"
	}
	slog.Info("daily generation started", "date", date, "mode", mode)

	r.mu.Lock()
	if r.status.Running {
		r.mu.Unlock()
		slog.Warn("daily generation rejected", "date", date, "mode", mode, "reason", "already_running")
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

	if !replace {
		if exists, err := r.store.Exists(date); err != nil {
			r.fail(err)
			slog.Error("daily generation failed", "date", date, "mode", mode, "stage", "exists", "error", err.Error())
			return nil, err
		} else if exists {
			r.fail(daily.ErrExists)
			slog.Warn("daily generation skipped", "date", date, "mode", mode, "reason", "already_exists")
			return nil, daily.ErrExists
		}
	}

	searchStart := r.now()
	results, err := r.searcher.SearchDailySources(ctx, date)
	if err != nil {
		err = fmt.Errorf("search: %w", err)
		r.fail(err)
		slog.Error("daily generation failed", "date", date, "mode", mode, "stage", "search", "duration", r.now().Sub(searchStart).String(), "error", err.Error())
		return nil, err
	}
	slog.Info("daily generation search completed", "date", date, "mode", mode, "results", len(results), "duration", r.now().Sub(searchStart).String())

	llmStart := r.now()
	markdown, err := r.llm.WriteDaily(ctx, date, results)
	if err != nil {
		err = fmt.Errorf("llm: %w", err)
		r.fail(err)
		slog.Error("daily generation failed", "date", date, "mode", mode, "stage", "llm", "duration", r.now().Sub(llmStart).String(), "error", err.Error())
		return nil, err
	}
	slog.Info("daily generation llm completed", "date", date, "mode", mode, "bytes", len(markdown), "duration", r.now().Sub(llmStart).String())

	writeStart := r.now()
	file, err := r.write(date, markdown, replace)
	if err != nil {
		r.fail(err)
		slog.Error("daily generation failed", "date", date, "mode", mode, "stage", "write", "duration", r.now().Sub(writeStart).String(), "error", err.Error())
		return nil, err
	}
	slog.Info("daily generation write completed", "date", date, "mode", mode, "file", file, "duration", r.now().Sub(writeStart).String())

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
	slog.Info("daily generation completed", "date", date, "mode", mode, "file", file, "duration", r.now().Sub(start).String())
	return result, nil
}

func (r *Runner) write(date, markdown string, replace bool) (string, error) {
	if replace {
		return r.store.ReplaceValidated(date, markdown)
	}
	return r.store.WriteValidated(date, markdown)
}

func (r *Runner) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.LastSuccess = false
	r.status.LastError = err.Error()
}
