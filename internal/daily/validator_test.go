package daily_test

import (
	"strings"
	"testing"

	"m-daily-news/internal/daily"
)

func TestValidateAllowsSummaryOverPreviousLimit(t *testing.T) {
	date := "2026-05-30"
	md := strings.Replace(validMarkdown(date), testSummary, strings.Repeat("摘", 260), 1)

	if err := daily.Validate(md, date); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsVeryLongSummary(t *testing.T) {
	date := "2026-05-30"
	md := strings.Replace(validMarkdown(date), testSummary, strings.Repeat("摘", 301), 1)

	err := daily.Validate(md, date)
	if err == nil {
		t.Fatalf("expected summary too long validation error")
	}
	if !strings.Contains(err.Error(), "summary too long") {
		t.Fatalf("unexpected error %v", err)
	}
}
