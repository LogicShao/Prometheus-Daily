package daily_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"m-daily-news/internal/daily"
)

func TestLocalDailyReportsAreValid(t *testing.T) {
	dir := repoPath(t, "content", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		count++
		if err := daily.ValidateFile(filepath.Join(dir, entry.Name()), strings.TrimSuffix(entry.Name(), ".md")); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
	}
	if count == 0 {
		t.Fatal("no local daily reports found")
	}
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
