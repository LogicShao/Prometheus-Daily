package daily_test

import (
	"strings"
	"testing"

	"m-daily-news/internal/daily"
)

func TestInjectAppVersionInsertsAfterDate(t *testing.T) {
	out, err := daily.InjectAppVersion(validMarkdown("2026-05-30"), "1.2.3")
	if err != nil {
		t.Fatalf("InjectAppVersion: %v", err)
	}

	if !strings.Contains(out, "date: 2026-05-30\napp_version: 1.2.3\nsummary:") {
		t.Fatalf("app_version not inserted after date:\n%s", out)
	}
	fm, _, err := daily.ParseFrontmatter(out)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm.AppVersion != "1.2.3" {
		t.Fatalf("app_version=%q, want 1.2.3", fm.AppVersion)
	}
}

func TestInjectAppVersionReplacesExistingValue(t *testing.T) {
	raw := strings.Replace(validMarkdown("2026-05-30"), "date: 2026-05-30\n", "date: 2026-05-30\napp_version: 9.9.9\n", 1)

	out, err := daily.InjectAppVersion(raw, "1.2.3")
	if err != nil {
		t.Fatalf("InjectAppVersion: %v", err)
	}

	if strings.Contains(out, "app_version: 9.9.9") {
		t.Fatalf("old app_version should be replaced:\n%s", out)
	}
	if strings.Count(out, "app_version:") != 1 {
		t.Fatalf("expected one app_version field:\n%s", out)
	}
	if !strings.Contains(out, "app_version: 1.2.3") {
		t.Fatalf("new app_version missing:\n%s", out)
	}
}
