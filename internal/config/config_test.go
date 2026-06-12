package config

import "testing"

func TestFromEnvParsesPreferenceLists(t *testing.T) {
	t.Setenv("DAILY_REPORT_MODE", "balanced")
	t.Setenv("DAILY_LOW_PRIORITY_KEYWORDS", "github copilot;copilot cli")
	t.Setenv("DAILY_LOW_PRIORITY_URLS", "github.blog/changelog,docs.github.com/en/copilot")
	t.Setenv("DAILY_LOW_PRIORITY_DOMAINS", "example.com")
	t.Setenv("DAILY_HIGH_VALUE_KEYWORDS", "vulnerability;breaking change")

	cfg := FromEnv()
	if got, want := len(cfg.LowPriorityKeywords), 2; got != want {
		t.Fatalf("LowPriorityKeywords len=%d, want %d: %#v", got, want, cfg.LowPriorityKeywords)
	}
	if got, want := len(cfg.LowPriorityURLSubstrings), 2; got != want {
		t.Fatalf("LowPriorityURLSubstrings len=%d, want %d: %#v", got, want, cfg.LowPriorityURLSubstrings)
	}
	if got, want := len(cfg.LowPriorityDomains), 1; got != want {
		t.Fatalf("LowPriorityDomains len=%d, want %d: %#v", got, want, cfg.LowPriorityDomains)
	}
	if got, want := len(cfg.HighValueKeywords), 2; got != want {
		t.Fatalf("HighValueKeywords len=%d, want %d: %#v", got, want, cfg.HighValueKeywords)
	}
}
