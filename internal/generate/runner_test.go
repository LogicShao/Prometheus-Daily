package generate_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
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
	status := runner.Status()
	if !status.LastSuccess || status.LastFile == "" || !status.TodayReady {
		t.Fatalf("unexpected status %#v", status)
	}

	_, err = runner.Run(context.Background(), today)
	if !errors.Is(err, daily.ErrExists) {
		t.Fatalf("duplicate err=%v, want ErrExists", err)
	}

	status = runner.Status()
	if !status.TodayReady {
		t.Fatalf("today_ready should stay true after duplicate failure when today's file exists: %#v", status)
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

type fakeSearcher struct{}

func (fakeSearcher) SearchDailySources(context.Context, string) ([]search.Result, error) {
	return []search.Result{{Title: "Source", URL: "https://example.com/source", Snippet: "snippet", Source: "test", Category: search.CategoryProduct}}, nil
}

type fakeLLM struct{}

func (fakeLLM) WriteDaily(_ context.Context, date string, _ []search.Result) (string, error) {
	return generatedMarkdown(date, generatedSummary), nil
}

type sequenceLLM struct {
	summaries []string
	calls     int
}

func (l *sequenceLLM) WriteDaily(_ context.Context, date string, _ []search.Result) (string, error) {
	summary := l.summaries[min(l.calls, len(l.summaries)-1)]
	l.calls++
	return generatedMarkdown(date, summary), nil
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
