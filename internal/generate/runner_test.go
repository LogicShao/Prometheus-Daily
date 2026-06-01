package generate_test

import (
	"context"
	"errors"
	"testing"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
	"m-daily-news/internal/search"
)

func TestRunnerRun(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	runner := generate.NewRunner(store, fakeSearcher{}, fakeLLM{})

	result, err := runner.Run(context.Background(), "2026-05-30")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Date != "2026-05-30" || result.Summary != generatedSummary {
		t.Fatalf("unexpected result %#v", result)
	}
	status := runner.Status()
	if !status.LastSuccess || status.LastFile == "" {
		t.Fatalf("unexpected status %#v", status)
	}

	_, err = runner.Run(context.Background(), "2026-05-30")
	if !errors.Is(err, daily.ErrExists) {
		t.Fatalf("duplicate err=%v, want ErrExists", err)
	}
}

type fakeSearcher struct{}

func (fakeSearcher) SearchDailySources(context.Context, string) ([]search.Result, error) {
	return []search.Result{{Title: "Source", URL: "https://example.com/source", Snippet: "snippet", Source: "test", Category: search.CategoryProduct}}, nil
}

type fakeLLM struct{}

func (fakeLLM) WriteDaily(_ context.Context, date string, _ []search.Result) (string, error) {
	return `---
date: ` + date + `
summary: "` + generatedSummary + `"
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
`, nil
}

const generatedSummary = "Generated summary now has enough detail to represent the report content and prevent shallow metadata from passing validation."
