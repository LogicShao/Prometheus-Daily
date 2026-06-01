package daily_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"m-daily-news/internal/daily"
)

func TestStoreWriteListReadAndDuplicate(t *testing.T) {
	workspace := t.TempDir()
	store := daily.NewStore(workspace)
	md := validMarkdown("2026-05-30")

	path, err := store.WriteValidated("2026-05-30", md)
	if err != nil {
		t.Fatalf("WriteValidated: %v", err)
	}
	if filepath.Base(path) != "2026-05-30.md" {
		t.Fatalf("unexpected path %q", path)
	}

	items, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Date != "2026-05-30" || items[0].Summary != testSummary {
		t.Fatalf("unexpected items %#v", items)
	}

	raw, err := store.ReadRaw("2026-05-30")
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	if string(raw) != md {
		t.Fatalf("raw mismatch")
	}

	if _, err := store.WriteValidated("2026-05-30", md); !errors.Is(err, daily.ErrExists) {
		t.Fatalf("duplicate err=%v, want ErrExists", err)
	}
}

func TestStoreValidationFailureLeavesNoTarget(t *testing.T) {
	workspace := t.TempDir()
	store := daily.NewStore(workspace)
	bad := `---
date: 2026-05-30
summary: "Bad"
tags: [AI]
---

1. Bad
   https://example.com
   <script>alert(1)</script>
`

	if _, err := store.WriteValidated("2026-05-30", bad); err == nil {
		t.Fatalf("expected validation error")
	}
	target := filepath.Join(workspace, "content", "daily", "2026-05-30.md")
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target stat err=%v, want not exist", err)
	}
}

func validMarkdown(date string) string {
	return `---
date: ` + date + `
summary: "` + testSummary + `"
tags: [AI, Agent]
---

# 日报 ` + date + `

## Example item

URL: https://example.com/news
来源: Example
发布日期: ` + date + `
类型: 产品

摘要: This paragraph gives enough detail about the first item to validate the readable daily report format and avoid shallow content.

为什么重要: It validates that the expected paragraph structure can be persisted and listed by the store.

不确定性/风险: No obvious risk, but the test still keeps an explicit risk paragraph to match production reports.

## Second item

URL: https://news.ycombinator.com/item?id=1
来源: Hacker News
发布日期: ` + date + `
类型: 产业

摘要: This paragraph gives enough detail about the second item to validate source diversity and keep the daily readable for humans.

为什么重要: It adds another domain so the validator can check the report is not built from a single source.

不确定性/风险: No obvious risk, but source quality should still be reviewed when the content is generated.

## Third item

URL: https://openai.com/index/example
来源: OpenAI
发布日期: ` + date + `
类型: 研究

摘要: This paragraph gives enough detail about the third item to validate category coverage and the paragraph style expected by the renderer.

为什么重要: It confirms that the report can combine several categories without falling back to a mechanical list.

不确定性/风险: No obvious risk, but model-generated summaries still need validation against the original source.
`
}

const testSummary = "This generated test report uses a longer frontmatter summary so the validator can reject shallow metadata while still keeping fixtures readable."
