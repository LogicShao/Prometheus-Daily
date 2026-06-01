package daily_test

import (
	"errors"
	"testing"
	"time"

	"m-daily-news/internal/daily"
)

func TestNormalizeDate(t *testing.T) {
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)

	got, err := daily.NormalizeDate("", now)
	if err != nil {
		t.Fatalf("NormalizeDate empty: %v", err)
	}
	if got != "2026-05-30" {
		t.Fatalf("got %q", got)
	}

	for _, input := range []string{"2026-05-30", "2024-02-29"} {
		if _, err := daily.NormalizeDate(input, now); err != nil {
			t.Fatalf("NormalizeDate(%q): %v", input, err)
		}
	}

	for _, input := range []string{"2026-02-30", "../2026-05-30", "2026/05/30", " 2026-05-30"} {
		if _, err := daily.NormalizeDate(input, now); !errors.Is(err, daily.ErrInvalidDate) {
			t.Fatalf("NormalizeDate(%q) err=%v, want ErrInvalidDate", input, err)
		}
	}
}
