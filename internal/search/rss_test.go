package search_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"m-daily-news/internal/search"
)

func TestRSSProviderSearch(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <item>
      <title>AI News</title>
      <link>https://example.com/news?utm_source=test</link>
      <description>Useful summary</description>
      <pubDate>Sat, 30 May 2026 10:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	provider := search.NewRSSProvider(client, []string{"https://feed.test/rss"})
	results, err := provider.Search(context.Background(), "", search.Options{
		MaxResults: 5,
		Since:      time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len=%d, want 1", len(results))
	}
	if results[0].Title != "AI News" || results[0].URL == "" || results[0].Provider != "rss" || results[0].Source != "Test" {
		t.Fatalf("unexpected result %#v", results[0])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
