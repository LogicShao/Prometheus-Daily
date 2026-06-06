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
	"m-daily-news/internal/reportmode"
	"m-daily-news/internal/search"
)

type Searcher interface {
	SearchDailySources(ctx context.Context, date string) ([]search.Result, error)
}

type Runner struct {
	store       *daily.Store
	searcher    Searcher
	llm         llm.Client
	retry       RetryConfig
	defaultMode reportmode.Mode
	mu          sync.Mutex
	status      Status
	now         func() time.Time
}

var ErrRunning = errors.New("generation already running")

func NewRunner(store *daily.Store, searcher Searcher, llmClient llm.Client) *Runner {
	return NewRunnerWithRetry(store, searcher, llmClient, DefaultRetryConfig)
}

func NewRunnerWithRetry(store *daily.Store, searcher Searcher, llmClient llm.Client, retry RetryConfig) *Runner {
	return NewRunnerWithRetryAndMode(store, searcher, llmClient, retry, reportmode.Balanced)
}

func NewRunnerWithMode(store *daily.Store, searcher Searcher, llmClient llm.Client, defaultMode reportmode.Mode) *Runner {
	return NewRunnerWithRetryAndMode(store, searcher, llmClient, DefaultRetryConfig, defaultMode)
}

func NewRunnerWithRetryAndMode(store *daily.Store, searcher Searcher, llmClient llm.Client, retry RetryConfig, defaultMode reportmode.Mode) *Runner {
	retry = retry.normalized()
	defaultMode = reportmode.Default(defaultMode)
	return &Runner{
		store:       store,
		searcher:    searcher,
		llm:         llmClient,
		retry:       retry,
		defaultMode: defaultMode,
		status:      Status{MaxAttempts: retry.MaxAttempts},
		now:         time.Now,
	}
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	status := r.status
	status.AttemptErrors = append([]string(nil), r.status.AttemptErrors...)
	r.mu.Unlock()
	status.MaxAttempts = r.retry.normalized().MaxAttempts

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
	return r.RunWithOptions(ctx, rawDate, Options{Mode: r.defaultMode})
}

func (r *Runner) RunWithOptions(ctx context.Context, rawDate string, opts Options) (*Result, error) {
	return r.run(ctx, rawDate, false, normalizeOptions(opts))
}

func (r *Runner) RerunToday(ctx context.Context) (*Result, error) {
	return r.RerunTodayWithOptions(ctx, Options{Mode: r.defaultMode})
}

func (r *Runner) RerunTodayWithOptions(ctx context.Context, opts Options) (*Result, error) {
	return r.run(ctx, "", true, normalizeOptions(opts))
}

func (r *Runner) run(ctx context.Context, rawDate string, replace bool, opts Options) (*Result, error) {
	start := r.now()
	retry := r.retry.normalized()
	date, err := daily.NormalizeDate(rawDate, r.now())
	if err != nil {
		return nil, err
	}
	mode := "create"
	if replace {
		mode = "replace"
	}
	slog.Info("daily generation started", "date", date, "mode", mode, "report_mode", opts.Mode)

	r.mu.Lock()
	if r.status.Running {
		r.mu.Unlock()
		slog.Warn("daily generation rejected", "date", date, "mode", mode, "reason", "already_running")
		return nil, ErrRunning
	}
	r.status.Running = true
	r.status.LastError = ""
	r.status.Attempts = 0
	r.status.MaxAttempts = retry.MaxAttempts
	r.status.LastStage = ""
	r.status.AttemptErrors = nil
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
			r.fail("", err)
			slog.Error("daily generation failed", "date", date, "mode", mode, "report_mode", opts.Mode, "stage", "exists", "error", err.Error())
			return nil, err
		} else if exists {
			r.fail("", daily.ErrExists)
			slog.Warn("daily generation skipped", "date", date, "mode", mode, "report_mode", opts.Mode, "reason", "already_exists")
			return nil, daily.ErrExists
		}
	}

	results, err := r.searchWithRetry(ctx, date, mode, opts.Mode, retry)
	if err != nil {
		return nil, err
	}

	result, err := r.writeWithRetry(ctx, date, mode, opts.Mode, replace, results, retry)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.status.LastSuccess = true
	r.status.LastFile = result.File
	r.status.LastError = ""
	r.status.Attempts = result.Attempts
	r.status.LastStage = ""
	r.mu.Unlock()
	slog.Info("daily generation completed", "date", date, "mode", mode, "report_mode", opts.Mode, "file", result.File, "attempts", result.Attempts, "duration", r.now().Sub(start).String())
	return result, nil
}

func (r *Runner) searchWithRetry(ctx context.Context, date, mode string, reportMode reportmode.Mode, retry RetryConfig) ([]search.Result, error) {
	for attempt := 1; attempt <= retry.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			r.fail("search", err)
			slog.Error("daily generation failed", "date", date, "mode", mode, "report_mode", reportMode, "stage", "search", "attempt", attempt, "max_attempts", retry.MaxAttempts, "error", err.Error())
			return nil, err
		}

		r.setStage("search", attempt)
		attemptStart := r.now()
		slog.Info("daily generation attempt started", "date", date, "mode", mode, "report_mode", reportMode, "stage", "search", "attempt", attempt, "max_attempts", retry.MaxAttempts)

		results, err := searchDailySources(ctx, r.searcher, date, reportMode)
		duration := r.now().Sub(attemptStart)
		if err == nil {
			slog.Info("daily generation search completed", "date", date, "mode", mode, "report_mode", reportMode, "attempt", attempt, "results", len(results), "duration", duration.String())
			return results, nil
		}

		err = fmt.Errorf("search: %w", err)
		r.failAttempt("search", attempt, err)
		slog.Error("daily generation attempt failed", "date", date, "mode", mode, "report_mode", reportMode, "stage", "search", "attempt", attempt, "max_attempts", retry.MaxAttempts, "duration", duration.String(), "error", err.Error())
		if !retryable("search", err) || attempt == retry.MaxAttempts {
			slog.Error("daily generation failed", "date", date, "mode", mode, "report_mode", reportMode, "stage", "search", "attempt", attempt, "max_attempts", retry.MaxAttempts, "duration", duration.String(), "error", err.Error())
			return nil, err
		}
		if err := r.waitBeforeRetry(ctx, date, mode, "search", attempt, retry); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (r *Runner) writeWithRetry(ctx context.Context, date, mode string, reportMode reportmode.Mode, replace bool, results []search.Result, retry RetryConfig) (*Result, error) {
	for attempt := 1; attempt <= retry.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			r.fail("llm", err)
			slog.Error("daily generation failed", "date", date, "mode", mode, "report_mode", reportMode, "stage", "llm", "attempt", attempt, "max_attempts", retry.MaxAttempts, "error", err.Error())
			return nil, err
		}

		r.setStage("llm", attempt)
		attemptStart := r.now()
		slog.Info("daily generation attempt started", "date", date, "mode", mode, "report_mode", reportMode, "stage", "llm", "attempt", attempt, "max_attempts", retry.MaxAttempts)

		llmStart := r.now()
		markdown, err := writeDaily(ctx, r.llm, date, reportMode, results)
		llmDuration := r.now().Sub(llmStart)
		if err != nil {
			err = fmt.Errorf("llm: %w", err)
			r.failAttempt("llm", attempt, err)
			slog.Error("daily generation attempt failed", "date", date, "mode", mode, "report_mode", reportMode, "stage", "llm", "attempt", attempt, "max_attempts", retry.MaxAttempts, "duration", llmDuration.String(), "error", err.Error())
			if !retryable("llm", err) || attempt == retry.MaxAttempts {
				slog.Error("daily generation failed", "date", date, "mode", mode, "report_mode", reportMode, "stage", "llm", "attempt", attempt, "max_attempts", retry.MaxAttempts, "duration", llmDuration.String(), "error", err.Error())
				return nil, err
			}
			if err := r.waitBeforeRetry(ctx, date, mode, "llm", attempt, retry); err != nil {
				return nil, err
			}
			continue
		}
		slog.Info("daily generation llm completed", "date", date, "mode", mode, "report_mode", reportMode, "attempt", attempt, "bytes", len(markdown), "duration", llmDuration.String())

		r.setStage("write", attempt)
		writeStart := r.now()
		file, err := r.write(date, markdown, replace)
		writeDuration := r.now().Sub(writeStart)
		if err != nil {
			r.failAttempt("write", attempt, err)
			slog.Error("daily generation attempt failed", "date", date, "mode", mode, "report_mode", reportMode, "stage", "write", "attempt", attempt, "max_attempts", retry.MaxAttempts, "duration", writeDuration.String(), "error", err.Error())
			if !retryable("write", err) || attempt == retry.MaxAttempts {
				slog.Error("daily generation failed", "date", date, "mode", mode, "report_mode", reportMode, "stage", "write", "attempt", attempt, "max_attempts", retry.MaxAttempts, "duration", writeDuration.String(), "error", err.Error())
				return nil, err
			}
			if err := r.waitBeforeRetry(ctx, date, mode, "write", attempt, retry); err != nil {
				return nil, err
			}
			continue
		}
		slog.Info("daily generation write completed", "date", date, "mode", mode, "report_mode", reportMode, "attempt", attempt, "file", file, "duration", writeDuration.String(), "total_attempt_duration", r.now().Sub(attemptStart).String())

		return &Result{
			Date:     date,
			File:     file,
			Summary:  daily.ExtractSummary(markdown),
			Attempts: attempt,
			Mode:     string(reportMode),
		}, nil
	}
	return nil, nil
}

func normalizeOptions(opts Options) Options {
	opts.Mode = reportmode.Default(opts.Mode)
	return opts
}

type modeSearcher interface {
	SearchDailySourcesWithMode(ctx context.Context, date string, mode reportmode.Mode) ([]search.Result, error)
}

func searchDailySources(ctx context.Context, searcher Searcher, date string, mode reportmode.Mode) ([]search.Result, error) {
	if s, ok := searcher.(modeSearcher); ok {
		return s.SearchDailySourcesWithMode(ctx, date, mode)
	}
	return searcher.SearchDailySources(ctx, date)
}

type modeWriter interface {
	WriteDailyWithMode(ctx context.Context, date string, mode reportmode.Mode, results []search.Result) (string, error)
}

func writeDaily(ctx context.Context, client llm.Client, date string, mode reportmode.Mode, results []search.Result) (string, error) {
	if c, ok := client.(modeWriter); ok {
		return c.WriteDailyWithMode(ctx, date, mode, results)
	}
	return client.WriteDaily(ctx, date, results)
}

func (r *Runner) write(date, markdown string, replace bool) (string, error) {
	if replace {
		return r.store.ReplaceValidated(date, markdown)
	}
	return r.store.WriteValidated(date, markdown)
}

func (r *Runner) waitBeforeRetry(ctx context.Context, date, mode, stage string, attempt int, retry RetryConfig) error {
	slog.Info("daily generation retry scheduled", "date", date, "mode", mode, "stage", stage, "attempt", attempt, "max_attempts", retry.MaxAttempts, "delay", retry.BaseDelay.String())
	if err := sleepRetry(ctx, retry.BaseDelay); err != nil {
		r.fail(stage, err)
		slog.Error("daily generation failed", "date", date, "mode", mode, "stage", stage, "attempt", attempt, "max_attempts", retry.MaxAttempts, "error", err.Error())
		return err
	}
	return nil
}

func (r *Runner) setStage(stage string, attempt int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.LastStage = stage
	if attempt > 0 {
		r.status.Attempts = attempt
	}
}

func (r *Runner) fail(stage string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.LastSuccess = false
	r.status.LastError = err.Error()
	if stage != "" {
		r.status.LastStage = stage
	}
}

func (r *Runner) failAttempt(stage string, attempt int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.LastSuccess = false
	r.status.LastError = err.Error()
	r.status.LastStage = stage
	r.status.Attempts = attempt
	r.status.AttemptErrors = appendAttemptError(r.status.AttemptErrors, stage, attempt, err)
}

func appendAttemptError(errors []string, stage string, attempt int, err error) []string {
	entry := fmt.Sprintf("%s attempt %d: %s", stage, attempt, truncate(err.Error(), 500))
	if len(errors) >= 3 {
		copy(errors, errors[1:])
		errors[len(errors)-1] = entry
		return errors
	}
	return append(errors, entry)
}

func truncate(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}
