package generate_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
	"m-daily-news/internal/reportmode"
	"m-daily-news/internal/search"
)

func TestRunnerRun(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	runner := generate.NewRunner(store, fakeSearcher{}, fakeLLM{})
	today := time.Now().Format("2006-01-02")

	result, err := runner.Run(context.Background(), today)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Date != today || result.Summary != generatedSummary {
		t.Fatalf("unexpected result %#v", result)
	}
	if result.Attempts != 1 {
		t.Fatalf("attempts=%d, want 1", result.Attempts)
	}
	status := runner.Status()
	if !status.LastSuccess || status.LastFile == "" || !status.TodayReady || status.Attempts != 1 || status.MaxAttempts != 3 {
		t.Fatalf("unexpected status %#v", status)
	}

	_, err = runner.Run(context.Background(), today)
	if !errors.Is(err, daily.ErrExists) {
		t.Fatalf("duplicate err=%v, want ErrExists", err)
	}

	status = runner.Status()
	if !status.TodayReady || status.Attempts != 0 || status.LastStage != "" {
		t.Fatalf("today_ready should stay true and duplicate should not retry: %#v", status)
	}
}

func TestRunnerRerunTodayReplacesExisting(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	llm := &sequenceLLM{summaries: []string{generatedSummary, rerunSummary}}
	runner := generate.NewRunner(store, fakeSearcher{}, llm)
	today := time.Now().Format("2006-01-02")

	if _, err := runner.Run(context.Background(), today); err != nil {
		t.Fatalf("Run: %v", err)
	}
	result, err := runner.RerunToday(context.Background())
	if err != nil {
		t.Fatalf("RerunToday: %v", err)
	}
	if result.Date != today || result.Summary != rerunSummary {
		t.Fatalf("unexpected rerun result %#v", result)
	}

	raw, err := store.ReadRaw(today)
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	if !strings.Contains(string(raw), rerunSummary) {
		t.Fatalf("rerun did not replace today's report")
	}
}

func TestRunnerPassesModeToModeAwareDependencies(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	searcher := &modeAwareSearcher{}
	llm := &modeAwareLLM{}
	runner := generate.NewRunnerWithMode(store, searcher, llm, reportmode.Research)

	result, err := runner.Run(context.Background(), todayDate())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Mode != string(reportmode.Research) {
		t.Fatalf("mode=%q, want research", result.Mode)
	}
	if searcher.mode != reportmode.Research {
		t.Fatalf("search mode=%q, want research", searcher.mode)
	}
	if llm.mode != reportmode.Research {
		t.Fatalf("llm mode=%q, want research", llm.mode)
	}
}

func TestRunnerRetriesInvalidMarkdownThenSucceeds(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	today := todayDate()
	llm := &sequenceLLM{
		markdowns: []string{
			invalidMarkdown(today),
			generatedMarkdown(today, generatedSummary),
		},
	}
	runner := newTestRunner(store, fakeSearcher{}, llm)

	result, err := runner.Run(context.Background(), today)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Attempts != 2 {
		t.Fatalf("attempts=%d, want 2", result.Attempts)
	}
	if llm.calls != 2 {
		t.Fatalf("llm calls=%d, want 2", llm.calls)
	}

	status := runner.Status()
	if !status.LastSuccess || status.Attempts != 2 || status.LastStage != "" || len(status.AttemptErrors) != 1 {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestRunnerRetryExhausted(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	today := todayDate()
	llm := &sequenceLLM{markdowns: []string{invalidMarkdown(today)}}
	runner := newTestRunner(store, fakeSearcher{}, llm)

	_, err := runner.Run(context.Background(), today)
	if !errors.Is(err, daily.ErrInvalidMarkdown) {
		t.Fatalf("Run err=%v, want ErrInvalidMarkdown", err)
	}
	if llm.calls != 3 {
		t.Fatalf("llm calls=%d, want 3", llm.calls)
	}

	status := runner.Status()
	if status.LastSuccess || status.Attempts != 3 || status.LastStage != "write" || len(status.AttemptErrors) != 3 {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestRunnerDoesNotRetryLLMContextCanceled(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	llm := &sequenceLLM{err: context.Canceled}
	runner := newTestRunner(store, fakeSearcher{}, llm)

	_, err := runner.Run(context.Background(), todayDate())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run err=%v, want context.Canceled", err)
	}
	if llm.calls != 1 {
		t.Fatalf("llm calls=%d, want 1", llm.calls)
	}

	status := runner.Status()
	if status.Attempts != 1 || status.LastStage != "llm" || len(status.AttemptErrors) != 1 {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestRunnerRetriesSearchThenSucceeds(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	searcher := &sequenceSearcher{errs: []error{errors.New("temporary search failure"), nil}}
	llm := &sequenceLLM{summaries: []string{generatedSummary}}
	runner := newTestRunner(store, searcher, llm)

	result, err := runner.Run(context.Background(), todayDate())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if searcher.calls != 2 {
		t.Fatalf("search calls=%d, want 2", searcher.calls)
	}
	if llm.calls != 1 || result.Attempts != 1 {
		t.Fatalf("llm calls=%d attempts=%d, want 1/1", llm.calls, result.Attempts)
	}

	status := runner.Status()
	if !status.LastSuccess || len(status.AttemptErrors) != 1 {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestRunnerDoesNotRetryCanceledSearch(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	searcher := &sequenceSearcher{errs: []error{context.Canceled}}
	llm := &sequenceLLM{summaries: []string{generatedSummary}}
	runner := newTestRunner(store, searcher, llm)

	_, err := runner.Run(context.Background(), todayDate())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run err=%v, want context.Canceled", err)
	}
	if searcher.calls != 1 {
		t.Fatalf("search calls=%d, want 1", searcher.calls)
	}
	if llm.calls != 0 {
		t.Fatalf("llm calls=%d, want 0", llm.calls)
	}
}

func TestRunnerRerunTodayRetriesAndReplacesExisting(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	today := todayDate()
	llm := &sequenceLLM{
		markdowns: []string{
			generatedMarkdown(today, generatedSummary),
			invalidMarkdown(today),
			generatedMarkdown(today, rerunSummary),
		},
	}
	runner := newTestRunner(store, fakeSearcher{}, llm)

	if _, err := runner.Run(context.Background(), today); err != nil {
		t.Fatalf("Run: %v", err)
	}
	result, err := runner.RerunToday(context.Background())
	if err != nil {
		t.Fatalf("RerunToday: %v", err)
	}
	if result.Attempts != 2 || result.Summary != rerunSummary {
		t.Fatalf("unexpected rerun result %#v", result)
	}

	raw, err := store.ReadRaw(today)
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	if !strings.Contains(string(raw), rerunSummary) {
		t.Fatalf("rerun did not replace today's report after retry")
	}
}

type fakeSearcher struct{}

func (fakeSearcher) SearchDailySources(context.Context, string) ([]search.Result, error) {
	return []search.Result{{Title: "Source", URL: "https://example.com/source", Snippet: "snippet", Source: "test", Category: search.CategoryProduct}}, nil
}

type fakeLLM struct{}

func (fakeLLM) WriteDaily(_ context.Context, date string, _ []search.Result) (string, error) {
	return generatedMarkdown(date, generatedSummary), nil
}

type modeAwareSearcher struct {
	mode reportmode.Mode
}

func (s *modeAwareSearcher) SearchDailySources(ctx context.Context, date string) ([]search.Result, error) {
	return s.SearchDailySourcesWithMode(ctx, date, reportmode.Balanced)
}

func (s *modeAwareSearcher) SearchDailySourcesWithMode(_ context.Context, _ string, mode reportmode.Mode) ([]search.Result, error) {
	s.mode = mode
	return []search.Result{{Title: "Source", URL: "https://example.com/source", Snippet: "snippet", Source: "test", Category: search.CategoryResearch}}, nil
}

type modeAwareLLM struct {
	mode reportmode.Mode
}

func (l *modeAwareLLM) WriteDaily(ctx context.Context, date string, results []search.Result) (string, error) {
	return l.WriteDailyWithMode(ctx, date, reportmode.Balanced, results)
}

func (l *modeAwareLLM) WriteDailyWithMode(_ context.Context, date string, mode reportmode.Mode, _ []search.Result) (string, error) {
	l.mode = mode
	return generatedMarkdown(date, generatedSummary), nil
}

type sequenceLLM struct {
	summaries []string
	markdowns []string
	err       error
	calls     int
}

func (l *sequenceLLM) WriteDaily(_ context.Context, date string, _ []search.Result) (string, error) {
	if l.err != nil {
		l.calls++
		return "", l.err
	}
	if len(l.markdowns) > 0 {
		markdown := l.markdowns[min(l.calls, len(l.markdowns)-1)]
		l.calls++
		return markdown, nil
	}
	summary := l.summaries[min(l.calls, len(l.summaries)-1)]
	l.calls++
	return generatedMarkdown(date, summary), nil
}

type sequenceSearcher struct {
	errs  []error
	calls int
}

func (s *sequenceSearcher) SearchDailySources(context.Context, string) ([]search.Result, error) {
	err := s.errs[min(s.calls, len(s.errs)-1)]
	s.calls++
	if err != nil {
		return nil, err
	}
	return fakeSearcher{}.SearchDailySources(context.Background(), "")
}

func newTestRunner(store *daily.Store, searcher generate.Searcher, llm *sequenceLLM) *generate.Runner {
	return generate.NewRunnerWithRetry(store, searcher, llm, generate.RetryConfig{MaxAttempts: 3, BaseDelay: 0})
}

func todayDate() string {
	return time.Now().Format("2006-01-02")
}

func invalidMarkdown(date string) string {
	return `---
date: ` + date + `
summary: "too short"
tags: [AI]
---

# 日报 ` + date + `

## Only one item

URL: https://example.com/source
来源: Example
发布日期: ` + date + `
类型: 产品

摘要: Too short.
`
}

func generatedMarkdown(date, summary string) string {
	return `---
date: ` + date + `
summary: "` + summary + `"
tags: [AI, Agent]
---

# 日报 ` + date + `

## Generated item

URL: https://example.com/source
来源: Example
发布日期: ` + date + `
类型: 产品

摘要: Generated content now uses a paragraph format with enough detail to validate persistence, rendering, and human readability.

为什么重要: It validates generation persistence with the paragraph-style report format.

不确定性/风险: No obvious risk, but generated content should still be checked against source material.

## Generated item 2

URL: https://news.ycombinator.com/item?id=2
来源: Hacker News
发布日期: ` + date + `
类型: 产业

摘要: Generated content now includes a second detailed paragraph so the validator can check source diversity and reject shallow reports.

为什么重要: It validates source diversity and keeps the generated daily aligned with the production prompt.

不确定性/风险: No obvious risk, but external links and summaries may drift over time.

## Generated item 3

URL: https://openai.com/index/generated
来源: OpenAI
发布日期: ` + date + `
类型: 研究

摘要: Generated content now includes a third detailed paragraph so the validator can check category coverage without requiring a bullet list.

为什么重要: It validates category coverage and confirms paragraph reports are accepted by the runner.

不确定性/风险: No obvious risk, but generated research summaries should still be compared with primary sources.
`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const generatedSummary = "Generated summary now has enough detail to represent the report content and prevent shallow metadata from passing validation."
const rerunSummary = "Rerun summary now has enough detail to prove today's existing report can be atomically regenerated and replaced."
