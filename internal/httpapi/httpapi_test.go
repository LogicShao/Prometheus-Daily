package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
	"m-daily-news/internal/httpapi"
	"m-daily-news/internal/search"
)

func TestGenerateAndReadFlow(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	runner := generate.NewRunner(store, apiSearcher{}, apiLLM{})
	router := httpapi.NewRouter(store, runner, "secret", storeWorkspace(store), time.Now())
	today := time.Now().Format("2006-01-02")

	unauth := httptest.NewRequest(http.MethodPost, "/api/generate", nil)
	unauthResp := httptest.NewRecorder()
	router.ServeHTTP(unauthResp, unauth)
	if unauthResp.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status=%d", unauthResp.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"date":"`+today+`"}`))
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("generate status=%d body=%s", resp.Code, resp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/daily", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status=%d", listResp.Code)
	}
	var list struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &list); err != nil {
		t.Fatalf("list json: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("total=%d, want 1", list.Total)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/daily/"+today, nil)
	detailResp := httptest.NewRecorder()
	router.ServeHTTP(detailResp, detailReq)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Date    string   `json:"date"`
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
		Body    string   `json:"body"`
	}
	if err := json.Unmarshal(detailResp.Body.Bytes(), &detail); err != nil {
		t.Fatalf("detail json: %v", err)
	}
	if detail.Date != today || detail.Summary != apiSummary || len(detail.Tags) != 2 || !strings.Contains(detail.Body, "## API generated item") {
		t.Fatalf("unexpected detail %#v", detail)
	}

	rawReq := httptest.NewRequest(http.MethodGet, "/api/daily/"+today+"/raw", nil)
	rawResp := httptest.NewRecorder()
	router.ServeHTTP(rawResp, rawReq)
	if rawResp.Code != http.StatusOK || !strings.Contains(rawResp.Body.String(), "## API generated item") {
		t.Fatalf("raw status=%d body=%s", rawResp.Code, rawResp.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	statusResp := httptest.NewRecorder()
	router.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status code=%d body=%s", statusResp.Code, statusResp.Body.String())
	}
	var status struct {
		Running    bool   `json:"running"`
		TodayReady bool   `json:"today_ready"`
		LastError  string `json:"last_error"`
	}
	if err := json.Unmarshal(statusResp.Body.Bytes(), &status); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if status.Running || !status.TodayReady || status.LastError != "" {
		t.Fatalf("unexpected status %#v", status)
	}
}

func storeWorkspace(store *daily.Store) string {
	return strings.TrimSuffix(strings.TrimSuffix(store.Dir(), "/daily"), "/content")
}

type apiSearcher struct{}

func (apiSearcher) SearchDailySources(context.Context, string) ([]search.Result, error) {
	return []search.Result{{Title: "Source", URL: "https://example.com/api", Snippet: "snippet", Source: "test", Category: search.CategoryProduct}}, nil
}

type apiLLM struct{}

func (apiLLM) WriteDaily(_ context.Context, date string, _ []search.Result) (string, error) {
	return `---
date: ` + date + `
summary: "` + apiSummary + `"
tags: [AI, Agent]
---

# 日报 ` + date + `

## API generated item

URL: https://example.com/api
来源: Example
发布日期: ` + date + `
类型: 产品

摘要: Generated content now uses a readable paragraph format that carries enough detail for the API generation flow.

为什么重要: It validates the API generation flow while keeping the report body suitable for frontend reading.

不确定性/风险: No obvious risk, but generated API results should still be validated before publishing.

## API generated item 2

URL: https://news.ycombinator.com/item?id=3
来源: Hacker News
发布日期: ` + date + `
类型: 产业

摘要: Generated content now includes source diversity in a paragraph format, so the report does not collapse into a mechanical list.

为什么重要: It validates source diversity and ensures multiple domains are represented in the API flow.

不确定性/风险: No obvious risk, but external summaries can become stale and should be checked when reused.

## API generated item 3

URL: https://openai.com/index/api-generated
来源: OpenAI
发布日期: ` + date + `
类型: 研究

摘要: Generated content now includes category coverage in a paragraph format, preserving validation without sacrificing readability.

为什么重要: It validates category coverage and confirms the structured detail endpoint returns the expected Markdown body.

不确定性/风险: No obvious risk, but research claims should still be checked against primary sources.
`, nil
}

const apiSummary = "API generated summary now carries enough detail to represent the report and prove frontmatter metadata is returned correctly."
