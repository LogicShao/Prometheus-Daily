package search

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"m-daily-news/internal/reportmode"
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

func TestSortResultsPrefersTechnicalSignal(t *testing.T) {
	now := time.Now()
	results := []Result{
		{
			Title:       "牧原股份与阿里云签署战略合作协议共建养猪大模型",
			URL:         "https://industry.example.com/pig",
			Snippet:     "合作协议覆盖猪场应用和产业落地。",
			Category:    CategoryIndustry,
			PublishedAt: now,
		},
		{
			Title:       "GitHub Copilot evaluation models support developer code completion",
			URL:         "https://github.blog/changelog/copilot-evaluation-models",
			Snippet:     "Developer tools and code completion model update.",
			Category:    CategoryProduct,
			PublishedAt: now.Add(-time.Hour),
		},
	}

	selected := sortResults(results)
	if selected[0].Category == CategoryIndustry {
		t.Fatalf("low-signal industry item ranked first: %#v", selected)
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

func TestSelectBalancedResultsLimitsIndustryFrequency(t *testing.T) {
	results := []Result{
		{Title: "Industry 1", URL: "https://industry-a.example.com/1", Category: CategoryIndustry},
		{Title: "Industry 2", URL: "https://industry-b.example.com/2", Category: CategoryIndustry},
		{Title: "Product", URL: "https://product.example.com/1", Category: CategoryProduct},
		{Title: "Security", URL: "https://security.example.com/1", Category: CategorySecurity},
	}

	selected := selectBalancedResults(results, 2, 5, 10)
	industryCount := 0
	for _, result := range selected {
		if result.Category == CategoryIndustry {
			industryCount++
		}
	}
	if industryCount != 1 {
		t.Fatalf("industryCount=%d, want 1: %#v", industryCount, selected)
	}
}

func TestSelectModeResultsKeepsResearchFloor(t *testing.T) {
	results := sortResultsWithMode([]Result{
		{Title: "Product 1", URL: "https://product-a.example.com/1", Category: CategoryProduct},
		{Title: "Product 2", URL: "https://product-b.example.com/2", Category: CategoryProduct},
		{Title: "Security", URL: "https://security.example.com/1", Category: CategorySecurity},
		{Title: "arXiv cs.AI paper 1", URL: "https://arxiv.org/abs/2606.00001", Source: "cs.AI updates on arXiv.org", Category: CategoryResearch},
		{Title: "arXiv cs.CL paper 2", URL: "https://arxiv.org/abs/2606.00002", Source: "cs.CL updates on arXiv.org", Category: CategoryResearch},
	}, reportmode.Balanced)

	selected := selectModeResults(results, reportmode.Balanced)
	if researchCount(selected) < 2 {
		t.Fatalf("researchCount=%d, want at least 2: %#v", researchCount(selected), selected)
	}
}

func TestSelectResearchModeRaisesResearchFloorAndDropsIndustry(t *testing.T) {
	results := []Result{
		{Title: "Industry", URL: "https://industry.example.com/1", Category: CategoryIndustry},
		{Title: "Product", URL: "https://product.example.com/1", Category: CategoryProduct},
		{Title: "Security", URL: "https://security.example.com/1", Category: CategorySecurity},
		{Title: "Paper 1", URL: "https://arxiv.org/abs/2606.00001", Source: "cs.AI updates on arXiv.org", Category: CategoryResearch},
		{Title: "Paper 2", URL: "https://arxiv.org/abs/2606.00002", Source: "cs.AI updates on arXiv.org", Category: CategoryResearch},
		{Title: "Paper 3", URL: "https://arxiv.org/abs/2606.00003", Source: "cs.CL updates on arXiv.org", Category: CategoryResearch},
		{Title: "Paper 4", URL: "https://arxiv.org/abs/2606.00004", Source: "cs.CL updates on arXiv.org", Category: CategoryResearch},
		{Title: "Paper 5", URL: "https://arxiv.org/abs/2606.00005", Source: "cs.AI updates on arXiv.org", Category: CategoryResearch},
	}

	selected := selectModeResults(sortResultsWithMode(results, reportmode.Research), reportmode.Research)
	if researchCount(selected) < 5 {
		t.Fatalf("researchCount=%d, want at least 5: %#v", researchCount(selected), selected)
	}
	for _, result := range selected {
		if result.Category == CategoryIndustry {
			t.Fatalf("research mode should drop industry result: %#v", selected)
		}
	}
}

func TestHistoryPenaltyLowersRepeatedResult(t *testing.T) {
	results := []Result{
		{Title: "Fresh benchmark for LLM agents", URL: "https://fresh.example.com/agent-benchmark", Category: CategoryResearch},
		{Title: "Repeated Copilot release", URL: "https://github.blog/changelog/repeated-copilot", Category: CategoryProduct},
	}
	history := []string{"URL: https://github.blog/changelog/repeated-copilot\n## Repeated Copilot release"}

	selected := sortResults(applyHistoryPenalty(results, history))
	if selected[0].URL == "https://github.blog/changelog/repeated-copilot" {
		t.Fatalf("repeated result ranked first: %#v", selected)
	}
	if selected[1].HistoryPenalty == 0 {
		t.Fatalf("expected history penalty: %#v", selected)
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
