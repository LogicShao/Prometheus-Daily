package scheduler

import (
	"testing"
	"time"
)

func TestParseTimeOfDay(t *testing.T) {
	got, err := ParseTimeOfDay("09:00")
	if err != nil {
		t.Fatalf("ParseTimeOfDay: %v", err)
	}
	if got.Hour != 9 || got.Minute != 0 {
		t.Fatalf("got %#v, want 09:00", got)
	}
}

func TestParseTimeOfDayRejectsInvalidTime(t *testing.T) {
	if _, err := ParseTimeOfDay("24:00"); err == nil {
		t.Fatalf("expected invalid time error")
	}
}

func TestNextRunUsesCurrentLocationAndRollsForward(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	s := NewDaily(nil, TimeOfDay{Hour: 9})

	before := time.Date(2026, 6, 1, 8, 30, 0, 0, loc)
	next := s.nextRun(before)
	if want := time.Date(2026, 6, 1, 9, 0, 0, 0, loc); !next.Equal(want) {
		t.Fatalf("next=%s, want %s", next, want)
	}

	after := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)
	next = s.nextRun(after)
	if want := time.Date(2026, 6, 2, 9, 0, 0, 0, loc); !next.Equal(want) {
		t.Fatalf("next=%s, want %s", next, want)
	}
}
