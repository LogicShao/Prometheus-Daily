package search_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"m-daily-news/internal/search"
)

func TestZhipuProviderSearch(t *testing.T) {
	client := &http.Client{Transport: zhipuRoundTrip(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization=%q", got)
		}
		body := `{
			"search_result": [
				{
					"title": "Zhipu result",
					"link": "https://example.com/zhipu",
					"content": "Result summary"
				}
			]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	provider := search.NewZhipuProvider("test-key", "https://zhipu.test/search", client)
	results, err := provider.Search(context.Background(), "AI news", search.Options{MaxResults: 3})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len=%d, want 1", len(results))
	}
	if results[0].Title != "Zhipu result" || results[0].URL != "https://example.com/zhipu" || results[0].Source != "zhipu" {
		t.Fatalf("unexpected result %#v", results[0])
	}
}

type zhipuRoundTrip func(*http.Request) (*http.Response, error)

func (f zhipuRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
