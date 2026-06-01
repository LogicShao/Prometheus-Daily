package generate

import (
	"context"
	"errors"
	"fmt"
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
	defer r.mu.Unlock()
	return r.status
}

func (r *Runner) Run(ctx context.Context, rawDate string) (*Result, error) {
	date, err := daily.NormalizeDate(rawDate, r.now())
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	if r.status.Running {
		r.mu.Unlock()
		return nil, ErrRunning
	}
	r.status.Running = true
	r.status.LastError = ""
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.status.Running = false
		r.status.LastRun = r.now()
		r.mu.Unlock()
	}()

	if exists, err := r.store.Exists(date); err != nil {
		r.fail(err)
		return nil, err
	} else if exists {
		r.fail(daily.ErrExists)
		return nil, daily.ErrExists
	}

	results, err := r.searcher.SearchDailySources(ctx, date)
	if err != nil {
		err = fmt.Errorf("search: %w", err)
		r.fail(err)
		return nil, err
	}

	markdown, err := r.llm.WriteDaily(ctx, date, results)
	if err != nil {
		err = fmt.Errorf("llm: %w", err)
		r.fail(err)
		return nil, err
	}

	file, err := r.store.WriteValidated(date, markdown)
	if err != nil {
		r.fail(err)
		return nil, err
	}

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
	return result, nil
}

func (r *Runner) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.LastSuccess = false
	r.status.LastError = err.Error()
}
