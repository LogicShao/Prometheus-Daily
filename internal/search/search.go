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

	"m-daily-news/internal/reportmode"
)

type Result struct {
	Title          string
	URL            string
	Snippet        string
	Source         string
	Provider       string
	Category       string
	PublishedAt    time.Time
	HistoryPenalty int
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
	history  HistoryProvider
}

type HistoryProvider interface {
	RecentReports(beforeDate string, limit int) ([]string, error)
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

func NewServiceWithHistory(primary []Provider, fallback []Provider, history HistoryProvider) *Service {
	return &Service{primary: primary, fallback: fallback, history: history}
}

func (s *Service) SearchDailySources(ctx context.Context, date string) ([]Result, error) {
	return s.SearchDailySourcesWithMode(ctx, date, reportmode.Balanced)
}

func (s *Service) SearchDailySourcesWithMode(ctx context.Context, date string, mode reportmode.Mode) ([]Result, error) {
	mode = reportmode.Default(mode)
	start := time.Now()
	since := time.Now().Add(-7 * 24 * time.Hour)
	opts := Options{MaxResults: 5, Since: since}
	queries := dailyQuerySpecs()

	slog.Info("daily source search started", "date", date, "report_mode", mode, "primary_providers", len(s.primary), "fallback_providers", len(s.fallback), "queries", len(queries))
	results, errs := searchProviders(ctx, s.primary, queries, opts)
	if shouldSearchFallback(results) && len(s.fallback) > 0 {
		slog.Info("daily source fallback search started", "date", date, "report_mode", mode, "results_before_fallback", len(results))
		fallback, fallbackErrs := searchProviders(ctx, s.fallback, queries, opts)
		results = append(results, fallback...)
		errs = append(errs, fallbackErrs...)
	}

	results = dedupe(results)
	history, historyErr := s.historyTexts(date, 7)
	if historyErr != nil {
		slog.Warn("daily source history load failed", "date", date, "error", historyErr.Error())
	}
	results = applyHistoryPenalty(results, history)
	results = sortResultsWithMode(results, mode)
	results = selectModeResults(results, mode)
	if len(results) == 0 {
		slog.Error("daily source search failed", "date", date, "report_mode", mode, "duration", time.Since(start).String(), "errors", joinErrors(errs))
		return nil, fmt.Errorf("%w: %s", ErrNoResults, joinErrors(errs))
	}
	slog.Info("daily source search completed", "date", date, "report_mode", mode, "results", len(results), "categories", len(uniqueCategories(results)), "duration", time.Since(start).String())
	return results, nil
}

func (s *Service) historyTexts(date string, limit int) ([]string, error) {
	if s.history == nil {
		return nil, nil
	}
	return s.history.RecentReports(date, limit)
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
		if result.Provider == "" {
			result.Provider = providerName
		}
		if result.Source == "" {
			result.Source = providerName
		}
		if isSearchProvider(providerName) && result.Source == providerName {
			result.Source = sourceFromURL(result.URL, providerName)
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
	return technicalSignalScoreWithMode(result, reportmode.Balanced)
}

func technicalSignalScoreWithMode(result Result, mode reportmode.Mode) int {
	mode = reportmode.Default(mode)
	text := strings.ToLower(strings.Join([]string{
		result.Title,
		result.Snippet,
		result.URL,
		result.Source,
		result.Provider,
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
	if isArxiv(result) {
		score += 5
		if mode == reportmode.Research {
			score += 4
		}
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
	if isSecondarySource(result) {
		score -= 2
	}
	if mode == reportmode.Research {
		switch result.Category {
		case CategoryResearch:
			score += 4
		case CategoryIndustry:
			score -= 3
		}
	}
	score -= result.HistoryPenalty
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
	return sortResultsWithMode(results, reportmode.Balanced)
}

func sortResultsWithMode(results []Result, mode reportmode.Mode) []Result {
	mode = reportmode.Default(mode)
	out := make([]Result, len(results))
	copy(out, results)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		aScore, bScore := technicalSignalScoreWithMode(a, mode), technicalSignalScoreWithMode(b, mode)
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
		return sourcePriority(a) < sourcePriority(b)
	})
	return out
}

func selectModeResults(results []Result, mode reportmode.Mode) []Result {
	mode = reportmode.Default(mode)
	switch mode {
	case reportmode.Research:
		return selectResearchResults(results, 20)
	default:
		return selectBalancedResultsWithResearchFloor(results, 20)
	}
}

func selectBalancedResultsWithResearchFloor(results []Result, maxTotal int) []Result {
	selected := selectBalancedResults(results, 2, 5, maxTotal)
	return ensureResearchFloor(selected, results, 2, maxTotal)
}

func selectResearchResults(results []Result, maxTotal int) []Result {
	selected := selectBalancedResults(results, 4, 8, maxTotal)
	selected = ensureResearchFloor(selected, results, 5, maxTotal)
	selected = limitIndustry(selected, 0)
	return fillToTotal(selected, results, maxTotal)
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
		host := quotaHost(result, u)
		limit := maxPerDomain
		if isArxiv(result) && maxPerDomain < 4 {
			limit = 4
		}
		if hostCount[host] >= limit {
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

func ensureResearchFloor(selected, candidates []Result, minResearch, maxTotal int) []Result {
	if minResearch <= 0 {
		return selected
	}
	out := append([]Result(nil), selected...)
	for researchCount(out) < minResearch {
		candidate, ok := nextResearchCandidate(out, candidates)
		if !ok {
			break
		}
		if len(out) >= maxTotal {
			replaceIndex := lowestPriorityNonResearchIndex(out)
			if replaceIndex < 0 {
				break
			}
			out[replaceIndex] = candidate
			continue
		}
		out = append(out, candidate)
	}
	return sortResults(out)
}

func nextResearchCandidate(selected, candidates []Result) (Result, bool) {
	for _, candidate := range candidates {
		if candidate.Category != CategoryResearch && !isArxiv(candidate) {
			continue
		}
		if containsResult(selected, candidate) {
			continue
		}
		return candidate, true
	}
	return Result{}, false
}

func lowestPriorityNonResearchIndex(results []Result) int {
	index := -1
	score := 0
	for i, result := range results {
		if result.Category == CategoryResearch || isArxiv(result) {
			continue
		}
		resultScore := technicalSignalScore(result)
		if index < 0 || resultScore < score {
			index = i
			score = resultScore
		}
	}
	return index
}

func researchCount(results []Result) int {
	count := 0
	for _, result := range results {
		if result.Category == CategoryResearch || isArxiv(result) {
			count++
		}
	}
	return count
}

func limitIndustry(results []Result, maxIndustry int) []Result {
	out := make([]Result, 0, len(results))
	count := 0
	for _, result := range results {
		if result.Category == CategoryIndustry {
			if count >= maxIndustry {
				continue
			}
			count++
		}
		out = append(out, result)
	}
	return out
}

func fillToTotal(selected, candidates []Result, maxTotal int) []Result {
	out := append([]Result(nil), selected...)
	for _, candidate := range candidates {
		if len(out) >= maxTotal {
			break
		}
		if candidate.Category == CategoryIndustry {
			continue
		}
		if containsResult(out, candidate) {
			continue
		}
		out = append(out, candidate)
	}
	return sortResultsWithMode(out, reportmode.Research)
}

func containsResult(results []Result, candidate Result) bool {
	key := canonicalURL(candidate.URL)
	for _, result := range results {
		if canonicalURL(result.URL) == key {
			return true
		}
	}
	return false
}

func quotaHost(result Result, u *url.URL) string {
	host := strings.ToLower(u.Host)
	if isArxiv(result) {
		source := strings.ToLower(result.Source)
		switch {
		case strings.Contains(source, "cs.ai"):
			return host + "/cs.ai"
		case strings.Contains(source, "cs.cl"):
			return host + "/cs.cl"
		}
	}
	return host
}

func isArxiv(result Result) bool {
	text := strings.ToLower(result.URL + " " + result.Source)
	return strings.Contains(text, "arxiv.org")
}

func isSecondarySource(result Result) bool {
	text := strings.ToLower(result.URL + " " + result.Source)
	return containsAny(text,
		"cnblogs.com", "new.qq.com", "tool.lu", "jiqizhixin.com", "36kr.com",
		"博客园", "腾讯新闻", "在线工具", "转载",
	)
}

func applyHistoryPenalty(results []Result, history []string) []Result {
	if len(history) == 0 {
		return results
	}
	historyText := normalizeHistory(strings.Join(history, "\n"))
	out := make([]Result, 0, len(results))
	for _, result := range results {
		penalty := historyPenalty(result, historyText)
		if penalty > 0 {
			result.HistoryPenalty = penalty
		}
		out = append(out, result)
	}
	return out
}

func historyPenalty(result Result, historyText string) int {
	if historyText == "" {
		return 0
	}
	u := strings.ToLower(canonicalURL(result.URL))
	if u != "" && strings.Contains(historyText, u) {
		return 8
	}
	tokens := titleTokens(result.Title)
	if len(tokens) < 2 {
		return 0
	}
	matches := 0
	for _, token := range tokens {
		if strings.Contains(historyText, token) {
			matches++
		}
	}
	if matches >= 3 || matches == len(tokens) {
		return 4
	}
	return 0
}

func normalizeHistory(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(raw), " "))
}

func titleTokens(title string) []string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= '\u4e00' && r <= '\u9fff':
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	parts := strings.Fields(b.String())
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		if isStopToken(part) {
			continue
		}
		tokens = append(tokens, part)
	}
	return tokens
}

func isStopToken(token string) bool {
	if len(token) <= 2 {
		return true
	}
	switch token {
	case "the", "and", "for", "with", "from", "into", "about", "agent", "agents", "model", "models", "开源", "发布", "新增", "支持", "日报", "研究":
		return true
	default:
		return false
	}
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

func sourcePriority(result Result) int {
	source := result.Provider
	if source == "" {
		source = result.Source
	}
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

func isSearchProvider(source string) bool {
	switch strings.ToLower(source) {
	case "zhipu", "tavily":
		return true
	default:
		return false
	}
}

func sourceFromURL(raw, fallback string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return fallback
	}
	return strings.TrimPrefix(strings.ToLower(u.Host), "www.")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
