package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
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
	var generateBody struct {
		Attempts int `json:"attempts"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &generateBody); err != nil {
		t.Fatalf("generate json: %v", err)
	}
	if generateBody.Attempts != 1 {
		t.Fatalf("attempts=%d, want 1", generateBody.Attempts)
	}

	rerunReq := httptest.NewRequest(http.MethodPost, "/api/generate/rerun", nil)
	rerunReq.Header.Set("Authorization", "Bearer secret")
	rerunResp := httptest.NewRecorder()
	router.ServeHTTP(rerunResp, rerunReq)
	if rerunResp.Code != http.StatusOK {
		t.Fatalf("rerun status=%d body=%s", rerunResp.Code, rerunResp.Body.String())
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

	feedReq := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	feedReq.Header.Set("X-Forwarded-Proto", "https")
	feedReq.Header.Set("X-Forwarded-Host", "daily.example.com")
	feedResp := httptest.NewRecorder()
	router.ServeHTTP(feedResp, feedReq)
	feedBody := feedResp.Body.String()
	if feedResp.Code != http.StatusOK || !strings.Contains(feedResp.Header().Get("Content-Type"), "application/rss+xml") {
		t.Fatalf("feed status=%d content-type=%s body=%s", feedResp.Code, feedResp.Header().Get("Content-Type"), feedBody)
	}
	if !strings.Contains(feedBody, `<rss version="2.0">`) ||
		!strings.Contains(feedBody, "Prometheus Daily "+today) ||
		!strings.Contains(feedBody, apiSummary) ||
		!strings.Contains(feedBody, "https://daily.example.com/api/daily/"+today+"/raw") {
		t.Fatalf("unexpected feed body=%s", feedBody)
	}

	rssReq := httptest.NewRequest(http.MethodGet, "/rss.xml", nil)
	rssResp := httptest.NewRecorder()
	router.ServeHTTP(rssResp, rssReq)
	if rssResp.Code != http.StatusOK {
		t.Fatalf("rss alias status=%d body=%s", rssResp.Code, rssResp.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	statusResp := httptest.NewRecorder()
	router.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status code=%d body=%s", statusResp.Code, statusResp.Body.String())
	}
	var status struct {
		Running     bool     `json:"running"`
		TodayReady  bool     `json:"today_ready"`
		LastError   string   `json:"last_error"`
		Attempts    int      `json:"attempts"`
		MaxAttempts int      `json:"max_attempts"`
		LastStage   string   `json:"last_stage"`
		Errors      []string `json:"attempt_errors"`
	}
	if err := json.Unmarshal(statusResp.Body.Bytes(), &status); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if status.Running || !status.TodayReady || status.LastError != "" || status.Attempts != 1 || status.MaxAttempts != 3 || status.LastStage != "" || len(status.Errors) != 0 {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestGenerateRetriesInvalidMarkdownAndReturnsOK(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	today := time.Now().Format("2006-01-02")
	llm := &apiSequenceLLM{
		markdowns: []string{
			apiInvalidMarkdown(today),
			apiMarkdown(today, apiSummary),
		},
	}
	runner := generate.NewRunnerWithRetry(store, apiSearcher{}, llm, generate.RetryConfig{MaxAttempts: 3})
	router := httpapi.NewRouter(store, runner, "secret", storeWorkspace(store), time.Now())

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"date":"`+today+`"}`))
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("generate status=%d body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Attempts int `json:"attempts"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("generate json: %v", err)
	}
	if body.Attempts != 2 || llm.calls != 2 {
		t.Fatalf("attempts=%d llm calls=%d, want 2/2", body.Attempts, llm.calls)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	statusResp := httptest.NewRecorder()
	router.ServeHTTP(statusResp, statusReq)
	var status struct {
		Attempts      int      `json:"attempts"`
		MaxAttempts   int      `json:"max_attempts"`
		LastStage     string   `json:"last_stage"`
		AttemptErrors []string `json:"attempt_errors"`
		LastError     string   `json:"last_error"`
	}
	if err := json.Unmarshal(statusResp.Body.Bytes(), &status); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if status.Attempts != 2 || status.MaxAttempts != 3 || status.LastStage != "" || status.LastError != "" || len(status.AttemptErrors) != 1 {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestGenerateRetryExhaustedReturnsInternalServerError(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	today := time.Now().Format("2006-01-02")
	llm := &apiSequenceLLM{err: errors.New("temporary llm failure")}
	runner := generate.NewRunnerWithRetry(store, apiSearcher{}, llm, generate.RetryConfig{MaxAttempts: 3})
	router := httpapi.NewRouter(store, runner, "secret", storeWorkspace(store), time.Now())

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"date":"`+today+`"}`))
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("generate status=%d body=%s", resp.Code, resp.Body.String())
	}
	if llm.calls != 3 {
		t.Fatalf("llm calls=%d, want 3", llm.calls)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	statusResp := httptest.NewRecorder()
	router.ServeHTTP(statusResp, statusReq)
	var status struct {
		Attempts      int      `json:"attempts"`
		MaxAttempts   int      `json:"max_attempts"`
		LastStage     string   `json:"last_stage"`
		AttemptErrors []string `json:"attempt_errors"`
		LastError     string   `json:"last_error"`
	}
	if err := json.Unmarshal(statusResp.Body.Bytes(), &status); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if status.Attempts != 3 || status.MaxAttempts != 3 || status.LastStage != "llm" || status.LastError == "" || len(status.AttemptErrors) != 3 {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestUnknownRoutesDoNotRenderIndex(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	runner := generate.NewRunner(store, apiSearcher{}, apiLLM{})
	router := httpapi.NewRouter(store, runner, "secret", storeWorkspace(store), time.Now())

	for _, path := range []string{"/.env", "/.git/config", "/_ignition/health-check"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d, want 404", path, resp.Code)
		}
	}

	faviconReq := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	faviconResp := httptest.NewRecorder()
	router.ServeHTTP(faviconResp, faviconReq)
	if faviconResp.Code != http.StatusNoContent {
		t.Fatalf("favicon status=%d, want 204", faviconResp.Code)
	}
}

func TestPageRoutesRender(t *testing.T) {
	store := daily.NewStore(t.TempDir())
	runner := generate.NewRunner(store, apiSearcher{}, apiLLM{})
	router := httpapi.NewRouter(store, runner, "secret", repoWorkspace(t), time.Now())

	for _, path := range []string{"/", "/about", "/admin"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, resp.Code, resp.Body.String())
		}
	}
}

func storeWorkspace(store *daily.Store) string {
	return strings.TrimSuffix(strings.TrimSuffix(store.Dir(), "/daily"), "/content")
}

func repoWorkspace(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

type apiSearcher struct{}

func (apiSearcher) SearchDailySources(context.Context, string) ([]search.Result, error) {
	return []search.Result{{Title: "Source", URL: "https://example.com/api", Snippet: "snippet", Source: "test", Category: search.CategoryProduct}}, nil
}

type apiLLM struct{}

func (apiLLM) WriteDaily(_ context.Context, date string, _ []search.Result) (string, error) {
	return apiMarkdown(date, apiSummary), nil
}

type apiSequenceLLM struct {
	markdowns []string
	err       error
	calls     int
}

func (l *apiSequenceLLM) WriteDaily(_ context.Context, date string, _ []search.Result) (string, error) {
	l.calls++
	if l.err != nil {
		return "", l.err
	}
	return l.markdowns[min(l.calls-1, len(l.markdowns)-1)], nil
}

func apiMarkdown(date, summary string) string {
	return `---
date: ` + date + `
summary: "` + summary + `"
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
`
}

func apiInvalidMarkdown(date string) string {
	return `---
date: ` + date + `
summary: "too short"
tags: [AI]
---

# 日报 ` + date + `

## API generated item

URL: https://example.com/api
来源: Example
发布日期: ` + date + `
类型: 产品

摘要: Too short.
`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const apiSummary = "API generated summary now carries enough detail to represent the report and prove frontmatter metadata is returned correctly."
