package daily_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"m-daily-news/internal/daily"
)

func TestLocalDailyReportsAreValid(t *testing.T) {
	dir := repoPath(t, "content", "daily")
	reports := trackedDailyReports(t)
	for _, report := range reports {
		name := filepath.Base(report)
		if err := daily.ValidateFile(filepath.Join(dir, name), strings.TrimSuffix(name, ".md")); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if len(reports) == 0 {
		t.Fatal("no tracked daily reports found")
	}
}

func trackedDailyReports(t *testing.T) []string {
	t.Helper()

	cmd := exec.Command("git", "ls-files", "--", "content/daily/*.md")
	cmd.Dir = repoPath(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	reports := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" || filepath.Ext(line) != ".md" {
			continue
		}
		reports = append(reports, line)
	}
	return reports
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	parts := append([]string{filepath.Dir(file), "..", ".."}, elems...)
	return filepath.Clean(filepath.Join(parts...))
}
