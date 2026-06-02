package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Result struct {
	Title       string
	URL         string
	Snippet     string
	Source      string
	Category    string
	PublishedAt time.Time
}

type Options struct {
	MaxResults int
	Since      time.Time
}

type Provider interface {
	Name() string
	Search(ctx context.Context, query string, opts Options) ([]Result, error)
}

type Service struct {
	primary  []Provider
	fallback []Provider
}

var ErrNoResults = errors.New("no search results")

const (
	CategoryResearch   = "研究"
	CategoryProduct    = "产品"
	CategoryOpenSource = "开源"
	CategorySecurity   = "安全"
	CategoryIndustry   = "产业"
)

type querySpec struct {
	Query    string
	Category string
}

func NewService(primary []Provider, fallback []Provider) *Service {
	return &Service{primary: primary, fallback: fallback}
}

func (s *Service) SearchDailySources(ctx context.Context, date string) ([]Result, error) {
	start := time.Now()
	since := time.Now().Add(-7 * 24 * time.Hour)
	opts := Options{MaxResults: 5, Since: since}
	queries := dailyQuerySpecs()

	slog.Info("daily source search started", "date", date, "primary_providers", len(s.primary), "fallback_providers", len(s.fallback), "queries", len(queries))
	results, errs := searchProviders(ctx, s.primary, queries, opts)
	if shouldSearchFallback(results) && len(s.fallback) > 0 {
		slog.Info("daily source fallback search started", "date", date, "results_before_fallback", len(results))
		fallback, fallbackErrs := searchProviders(ctx, s.fallback, queries, opts)
		results = append(results, fallback...)
		errs = append(errs, fallbackErrs...)
	}

	results = dedupe(results)
	results = sortResults(results)
	results = selectBalancedResults(results, 2, 5, 20)
	if len(results) == 0 {
		slog.Error("daily source search failed", "date", date, "duration", time.Since(start).String(), "errors", joinErrors(errs))
		return nil, fmt.Errorf("%w: %s", ErrNoResults, joinErrors(errs))
	}
	slog.Info("daily source search completed", "date", date, "results", len(results), "categories", len(uniqueCategories(results)), "duration", time.Since(start).String())
	return results, nil
}

func dailyQuerySpecs() []querySpec {
	return []querySpec{
		{Query: "AI LLM agent research paper arxiv 大模型 研究 论文", Category: CategoryResearch},
		{Query: "AI developer tools API SDK model platform release 工具 平台 API 发布", Category: CategoryProduct},
		{Query: "AI agent LLM open source GitHub framework repository 开源 框架 仓库", Category: CategoryOpenSource},
		{Query: "AI agent LLM security vulnerability prompt injection database 安全 漏洞 数据库", Category: CategorySecurity},
		{Query: "AI production engineering case study architecture benchmark enterprise 工程 实践 架构 评测", Category: CategoryIndustry},
	}
}

func shouldSearchFallback(results []Result) bool {
	return len(results) < 8 || len(uniqueCategories(results)) < 3
}

func searchProviders(ctx context.Context, providers []Provider, queries []querySpec, opts Options) ([]Result, []error) {
	var results []Result
	var errs []error
	for _, provider := range providers {
		if provider.Name() == "rss" {
			items, err := provider.Search(ctx, "", opts)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", provider.Name(), err))
				slog.Warn("search provider failed", "provider", provider.Name(), "error", err.Error())
				continue
			}
			slog.Info("search provider completed", "provider", provider.Name(), "results", len(items))
			results = append(results, annotateResults(items, provider.Name(), "")...)
			continue
		}

		for _, spec := range queries {
			items, err := provider.Search(ctx, spec.Query, opts)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", provider.Name(), err))
				slog.Warn("search provider failed", "provider", provider.Name(), "category", spec.Category, "error", err.Error())
				continue
			}
			slog.Info("search provider completed", "provider", provider.Name(), "category", spec.Category, "results", len(items))
			results = append(results, annotateResults(items, provider.Name(), spec.Category)...)
		}
	}
	return results, errs
}

func annotateResults(results []Result, providerName, category string) []Result {
	out := make([]Result, 0, len(results))
	for _, result := range results {
		if result.Source == "" {
			result.Source = providerName
		}
		if result.Category == "" {
			result.Category = category
		}
		if result.Category == "" {
			result.Category = inferCategory(result)
		}
		out = append(out, result)
	}
	return out
}

func inferCategory(result Result) string {
	text := strings.ToLower(strings.Join([]string{
		result.Title,
		result.Snippet,
		result.URL,
		result.Source,
	}, " "))

	switch {
	case containsAny(text, "security", "vulnerability", "prompt injection", "jailbreak", "cve", "malware", "安全", "漏洞", "攻击", "风险"):
		return CategorySecurity
	case containsAny(text, "arxiv", "paper", "research", "study", "benchmark", "evaluation", "论文", "研究", "评测"):
		return CategoryResearch
	case containsAny(text, "github", "open source", "open-source", "oss", "repository", "开源", "仓库"):
		return CategoryOpenSource
	case containsAny(text, "launch", "release", "product", "platform", "api", "developer tool", "发布", "产品", "平台", "工具"):
		return CategoryProduct
	default:
		return CategoryIndustry
	}
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func dedupe(results []Result) []Result {
	seen := make(map[string]struct{}, len(results))
	out := make([]Result, 0, len(results))
	for _, result := range results {
		key := canonicalURL(result.URL)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result.URL = key
		out = append(out, result)
	}
	return out
}

func technicalSignalScore(result Result) int {
	text := strings.ToLower(strings.Join([]string{
		result.Title,
		result.Snippet,
		result.URL,
		result.Source,
	}, " "))

	score := 0
	switch result.Category {
	case CategoryResearch, CategoryOpenSource, CategorySecurity:
		score += 2
	case CategoryProduct:
		score += 1
	case CategoryIndustry:
		score -= 1
	}
	if containsAny(text,
		"github.com", "github.blog/changelog", "arxiv.org", "cve", "security advisory",
		"vulnerability", "prompt injection", "database", "repository", "open source",
		"benchmark", "paper", "architecture", "inference", "training",
		"漏洞", "提示注入", "数据库", "仓库", "开源", "论文", "基准", "架构", "推理", "训练",
	) {
		score += 3
	}
	if containsAny(text,
		"api", "sdk", "framework", "agent", "llm", "copilot", "developer tool",
		"model", "evaluation", "研究", "评测", "模型", "工具", "框架", "开发者", "代码",
	) {
		score += 1
	}
	if containsAny(text,
		"political advocacy", "policy", "政策", "政治", "倡导", "战略合作", "合作协议",
		"融资", "榜单", "排名", "测评报告", "企业测评", "产业规模", "市场规模",
		"标杆企业", "商业化能力", "养猪", "生猪", "猪场", "营收", "股价",
	) {
		score -= 4
	}
	return score
}

func canonicalURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Fragment = ""
	q := u.Query()
	for key := range q {
		if strings.HasPrefix(strings.ToLower(key), "utm_") {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func joinErrors(errs []error) string {
	if len(errs) == 0 {
		return "no provider errors"
	}
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}

func sortResults(results []Result) []Result {
	out := make([]Result, len(results))
	copy(out, results)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		aScore, bScore := technicalSignalScore(a), technicalSignalScore(b)
		if aScore != bScore {
			return aScore > bScore
		}
		if !a.PublishedAt.IsZero() && !b.PublishedAt.IsZero() && !a.PublishedAt.Equal(b.PublishedAt) {
			return a.PublishedAt.After(b.PublishedAt)
		}
		if !a.PublishedAt.IsZero() && b.PublishedAt.IsZero() {
			return true
		}
		if a.PublishedAt.IsZero() && !b.PublishedAt.IsZero() {
			return false
		}
		return sourcePriority(a.Source) < sourcePriority(b.Source)
	})
	return out
}

func selectBalancedResults(results []Result, maxPerDomain, maxPerCategory, maxTotal int) []Result {
	if maxPerDomain <= 0 {
		maxPerDomain = 2
	}
	if maxPerCategory <= 0 {
		maxPerCategory = 5
	}
	if maxTotal <= 0 {
		maxTotal = 20
	}
	hostCount := make(map[string]int)
	categoryCount := make(map[string]int)
	out := make([]Result, 0, min(len(results), maxTotal))
	for _, result := range results {
		if len(out) >= maxTotal {
			break
		}
		u, err := url.Parse(result.URL)
		if err != nil || u.Host == "" {
			continue
		}
		host := strings.ToLower(u.Host)
		if hostCount[host] >= maxPerDomain {
			continue
		}
		category := result.Category
		if category == "" {
			category = CategoryIndustry
			result.Category = category
		}
		if categoryCount[category] >= categoryLimit(category, maxPerCategory) {
			continue
		}
		hostCount[host]++
		categoryCount[category]++
		out = append(out, result)
	}
	return out
}

func categoryLimit(category string, maxPerCategory int) int {
	if category == CategoryIndustry && maxPerCategory > 1 {
		return 1
	}
	return maxPerCategory
}

func uniqueCategories(results []Result) map[string]struct{} {
	categories := make(map[string]struct{})
	for _, result := range results {
		category := result.Category
		if category == "" {
			category = inferCategory(result)
		}
		if category != "" {
			categories[category] = struct{}{}
		}
	}
	return categories
}

func sourcePriority(source string) int {
	switch strings.ToLower(source) {
	case "rss":
		return 0
	case "zhipu":
		return 1
	case "tavily":
		return 2
	default:
		return 3
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
