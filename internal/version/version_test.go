package version_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"m-daily-news/internal/version"
)

func TestReadUsesWorkspaceVersion(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "VERSION"), []byte("1.2.3\n"), 0644); err != nil {
		t.Fatalf("WriteFile VERSION: %v", err)
	}

	if got := version.Read(workspace); got != "1.2.3" {
		t.Fatalf("Read=%q, want 1.2.3", got)
	}
}

func TestReadFallsBackToDefault(t *testing.T) {
	if got := version.Read(t.TempDir()); got != version.Default {
		t.Fatalf("Read=%q, want %q", got, version.Default)
	}
}

func TestRootVersionMatchesDefault(t *testing.T) {
	raw, err := os.ReadFile(repoPath(t, "VERSION"))
	if err != nil {
		t.Fatalf("ReadFile VERSION: %v", err)
	}
	if got := string(bytesTrimSpace(raw)); got != version.Default {
		t.Fatalf("VERSION=%q, want %q", got, version.Default)
	}
}

func TestIsValid(t *testing.T) {
	for _, v := range []string{"0.2.0", "1.2.3", "1.2.3-rc.1", "1.2.3+build.1"} {
		if !version.IsValid(v) {
			t.Fatalf("IsValid(%q)=false", v)
		}
	}
	for _, v := range []string{"", "1", "1.2", "v1.2.3", "1.02.3"} {
		if version.IsValid(v) {
			t.Fatalf("IsValid(%q)=true", v)
		}
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

func bytesTrimSpace(raw []byte) []byte {
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\n' || raw[start] == '\t' || raw[start] == '\r') {
		start++
	}
	end := len(raw)
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\n' || raw[end-1] == '\t' || raw[end-1] == '\r') {
		end--
	}
	return raw[start:end]
}
