package search

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestSearchProvidersAnnotatesQueryCategory(t *testing.T) {
	provider := queryEchoProvider{name: "zhipu"}
	results, errs := searchProviders(context.Background(), []Provider{provider}, []querySpec{
		{Query: "research query", Category: CategoryResearch},
		{Query: "security query", Category: CategorySecurity},
	}, Options{MaxResults: 2})
	if len(errs) != 0 {
		t.Fatalf("errs=%v, want none", errs)
	}
	if len(results) != 2 {
		t.Fatalf("len=%d, want 2", len(results))
	}
	if results[0].Category != CategoryResearch || results[1].Category != CategorySecurity {
		t.Fatalf("unexpected categories %#v", results)
	}
}

func TestSearchProvidersInfersRSSCategory(t *testing.T) {
	calls := []string{}
	provider := staticProvider{
		name: "rss",
		results: []Result{{
			Title:   "Prompt injection vulnerability in AI agents",
			URL:     "https://example.com/security",
			Snippet: "Security researchers described a new vulnerability.",
		}},
		calls: &calls,
	}
	results, errs := searchProviders(context.Background(), []Provider{provider}, dailyQuerySpecs(), Options{})
	if len(errs) != 0 {
		t.Fatalf("errs=%v, want none", errs)
	}
	if len(results) != 1 {
		t.Fatalf("len=%d, want 1", len(results))
	}
	if results[0].Category != CategorySecurity {
		t.Fatalf("category=%q, want %q", results[0].Category, CategorySecurity)
	}
	if len(calls) != 1 || calls[0] != "" {
		t.Fatalf("rss calls=%#v, want one empty query", calls)
	}
}

func TestSelectBalancedResultsLimitsDomainsAndCategories(t *testing.T) {
	results := []Result{
		{Title: "Product 1", URL: "https://a.example.com/1", Category: CategoryProduct},
		{Title: "Product 2", URL: "https://a.example.com/2", Category: CategoryProduct},
		{Title: "Product 3", URL: "https://b.example.com/3", Category: CategoryProduct},
		{Title: "Research 1", URL: "https://c.example.com/1", Category: CategoryResearch},
		{Title: "Research 2", URL: "https://d.example.com/2", Category: CategoryResearch},
		{Title: "Security 1", URL: "https://e.example.com/1", Category: CategorySecurity},
	}

	selected := selectBalancedResults(results, 1, 2, 10)
	if len(selected) != 5 {
		t.Fatalf("len=%d, want 5: %#v", len(selected), selected)
	}

	domainCount := map[string]int{}
	categoryCount := map[string]int{}
	for _, result := range selected {
		domain := strings.TrimPrefix(strings.Split(result.URL, "/")[2], "www.")
		domainCount[domain]++
		categoryCount[result.Category]++
		if domainCount[domain] > 1 {
			t.Fatalf("domain %s selected too often: %#v", domain, selected)
		}
		if categoryCount[result.Category] > 2 {
			t.Fatalf("category %s selected too often: %#v", result.Category, selected)
		}
	}
}

func BenchmarkSearchDedupeSortAndBalance(b *testing.B) {
	results := make([]Result, 0, 500)
	categories := []string{CategoryResearch, CategoryProduct, CategoryOpenSource, CategorySecurity, CategoryIndustry}
	for i := 0; i < 500; i++ {
		results = append(results, Result{
			Title:    fmt.Sprintf("AI result %d", i),
			URL:      fmt.Sprintf("https://source-%02d.example.com/news/%d?utm_source=test", i%25, i),
			Snippet:  "Short result summary.",
			Source:   "benchmark",
			Category: categories[i%len(categories)],
		})
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		selected := selectBalancedResults(sortResults(dedupe(results)), 2, 5, 20)
		if len(selected) == 0 {
			b.Fatal("no selected results")
		}
	}
}

type queryEchoProvider struct {
	name string
}

func (p queryEchoProvider) Name() string {
	return p.name
}

func (p queryEchoProvider) Search(_ context.Context, query string, _ Options) ([]Result, error) {
	return []Result{{
		Title:   query,
		URL:     "https://example.com/" + strings.ReplaceAll(query, " ", "-"),
		Snippet: "snippet",
		Source:  p.name,
	}}, nil
}

type staticProvider struct {
	name    string
	results []Result
	calls   *[]string
}

func (p staticProvider) Name() string {
	return p.name
}

func (p staticProvider) Search(_ context.Context, query string, _ Options) ([]Result, error) {
	if p.calls != nil {
		*p.calls = append(*p.calls, query)
	}
	return p.results, nil
}
