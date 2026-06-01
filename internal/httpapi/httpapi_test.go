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

	unauth := httptest.NewRequest(http.MethodPost, "/api/generate", nil)
	unauthResp := httptest.NewRecorder()
	router.ServeHTTP(unauthResp, unauth)
	if unauthResp.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status=%d", unauthResp.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"date":"2026-05-30"}`))
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

	rawReq := httptest.NewRequest(http.MethodGet, "/api/daily/2026-05-30/raw", nil)
	rawResp := httptest.NewRecorder()
	router.ServeHTTP(rawResp, rawReq)
	if rawResp.Code != http.StatusOK || !strings.Contains(rawResp.Body.String(), "API generated") {
		t.Fatalf("raw status=%d body=%s", rawResp.Code, rawResp.Body.String())
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
summary: "API generated"
tags: [AI, Agent]
---

1. API generated item
   - URL: https://example.com/api
   - 来源: Example
   - 发布日期: ` + date + `
   - 类型: 产品
   - 摘要: Generated content.
   - 为什么重要: It validates the API generation flow.
   - 不确定性/风险: No obvious risk.

2. API generated item 2
   - URL: https://news.ycombinator.com/item?id=3
   - 来源: Hacker News
   - 发布日期: ` + date + `
   - 类型: 产业
   - 摘要: Generated content.
   - 为什么重要: It validates source diversity.
   - 不确定性/风险: No obvious risk.

3. API generated item 3
   - URL: https://openai.com/index/api-generated
   - 来源: OpenAI
   - 发布日期: ` + date + `
   - 类型: 研究
   - 摘要: Generated content.
   - 为什么重要: It validates category coverage.
   - 不确定性/风险: No obvious risk.
`, nil
}
