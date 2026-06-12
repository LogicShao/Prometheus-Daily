package search

import (
	"net/url"
	"strings"
)

const (
	lowPriorityPenalty      = 4
	highValuePenalty        = 1
	fatiguePenaltyPerReport = 2
	maxFatiguePenalty       = 4
)

type PreferenceConfig struct {
	LowPriorityKeywords      []string
	LowPriorityURLSubstrings []string
	LowPriorityDomains       []string
	HighValueKeywords        []string
}

func DefaultPreferenceConfig() PreferenceConfig {
	return PreferenceConfig{
		LowPriorityKeywords: []string{
			"github copilot",
			"copilot cli",
			"copilot chat",
			"copilot coding agent",
			"copilot workspace",
			"copilot usage metrics",
			"copilot evaluation models",
			"copilot in vs code",
		},
		LowPriorityURLSubstrings: []string{
			"github.blog/changelog",
			"/features/copilot",
			"/github-copilot",
			"/copilot/",
			"docs.github.com/en/copilot",
		},
		HighValueKeywords: []string{
			"security advisory",
			"vulnerability",
			"cve",
			"prompt injection",
			"data leak",
			"token leak",
			"credential leak",
			"exploit",
			"supply chain",
			"breaking change",
			"model deprecation",
			"deprecated",
			"end of support",
			"enterprise impact",
			"漏洞",
			"泄露",
			"泄漏",
			"攻击",
			"提示注入",
			"供应链",
			"破坏性变更",
			"弃用",
			"停止支持",
			"企业影响",
		},
	}
}

func (c PreferenceConfig) Merge(extra PreferenceConfig) PreferenceConfig {
	c.LowPriorityKeywords = append(c.LowPriorityKeywords, extra.LowPriorityKeywords...)
	c.LowPriorityURLSubstrings = append(c.LowPriorityURLSubstrings, extra.LowPriorityURLSubstrings...)
	c.LowPriorityDomains = append(c.LowPriorityDomains, extra.LowPriorityDomains...)
	c.HighValueKeywords = append(c.HighValueKeywords, extra.HighValueKeywords...)
	return c.normalized()
}

func (c PreferenceConfig) normalized() PreferenceConfig {
	return PreferenceConfig{
		LowPriorityKeywords:      normalizePreferenceList(c.LowPriorityKeywords),
		LowPriorityURLSubstrings: normalizePreferenceList(c.LowPriorityURLSubstrings),
		LowPriorityDomains:       normalizeDomains(c.LowPriorityDomains),
		HighValueKeywords:        normalizePreferenceList(c.HighValueKeywords),
	}
}

func applyPreferencePenalty(results []Result, config PreferenceConfig, history []string) []Result {
	config = config.normalized()
	if preferenceConfigEmpty(config) {
		return results
	}

	historyHits := preferenceHistoryHits(history, lowPriorityTerms(config))
	out := make([]Result, 0, len(results))
	for _, result := range results {
		result.PreferencePenalty = preferencePenalty(result, config, historyHits)
		out = append(out, result)
	}
	return out
}

func preferencePenalty(result Result, config PreferenceConfig, historyHits map[string]int) int {
	text := preferenceText(result)
	lowPriorityTerms := matchedLowPriorityTerms(result, text, config)
	if len(lowPriorityTerms) == 0 {
		return 0
	}
	if containsPreferenceTerm(text, config.HighValueKeywords) {
		return highValuePenalty
	}

	penalty := lowPriorityPenalty
	fatigue := 0
	for _, term := range lowPriorityTerms {
		fatigue += historyHits[term] * fatiguePenaltyPerReport
	}
	if fatigue > maxFatiguePenalty {
		fatigue = maxFatiguePenalty
	}
	return penalty + fatigue
}

func matchedLowPriorityTerms(result Result, text string, config PreferenceConfig) []string {
	var terms []string
	for _, keyword := range config.LowPriorityKeywords {
		if strings.Contains(text, keyword) {
			terms = append(terms, keyword)
		}
	}

	urlText := strings.ToLower(strings.TrimSpace(result.URL))
	for _, part := range config.LowPriorityURLSubstrings {
		if strings.Contains(urlText, part) {
			terms = append(terms, part)
		}
	}

	for _, domain := range config.LowPriorityDomains {
		if urlHostMatches(result.URL, domain) {
			terms = append(terms, domain)
		}
	}
	return terms
}

func preferenceHistoryHits(history []string, terms []string) map[string]int {
	hits := make(map[string]int, len(terms))
	for _, report := range history {
		text := normalizePreferenceText(report)
		for _, term := range terms {
			if strings.Contains(text, term) {
				hits[term]++
			}
		}
	}
	return hits
}

func lowPriorityTerms(config PreferenceConfig) []string {
	terms := make([]string, 0,
		len(config.LowPriorityKeywords)+len(config.LowPriorityURLSubstrings)+len(config.LowPriorityDomains))
	terms = append(terms, config.LowPriorityKeywords...)
	terms = append(terms, config.LowPriorityURLSubstrings...)
	terms = append(terms, config.LowPriorityDomains...)
	return terms
}

func containsPreferenceTerm(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func preferenceText(result Result) string {
	return normalizePreferenceText(strings.Join([]string{
		result.Title,
		result.Snippet,
		result.URL,
		result.Source,
		result.Provider,
		result.Category,
	}, " "))
}

func normalizePreferenceText(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(raw), " "))
}

func normalizePreferenceList(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = normalizePreferenceText(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeDomains(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = normalizeDomain(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeDomain(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "www.")
	if i := strings.IndexAny(raw, "/:"); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimSpace(raw)
}

func urlHostMatches(rawURL, domain string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" || domain == "" {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func preferenceConfigEmpty(config PreferenceConfig) bool {
	return len(config.LowPriorityKeywords) == 0 &&
		len(config.LowPriorityURLSubstrings) == 0 &&
		len(config.LowPriorityDomains) == 0
}

func preferencePenaltyCount(results []Result) int {
	count := 0
	for _, result := range results {
		if result.PreferencePenalty > 0 {
			count++
		}
	}
	return count
}

func totalPreferencePenalty(results []Result) int {
	total := 0
	for _, result := range results {
		total += result.PreferencePenalty
	}
	return total
}
