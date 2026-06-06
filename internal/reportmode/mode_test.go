package reportmode

import (
	"errors"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		raw  string
		want Mode
	}{
		{"", Balanced},
		{"balanced", Balanced},
		{" Research ", Research},
	}

	for _, tt := range tests {
		got, err := Normalize(tt.raw)
		if err != nil {
			t.Fatalf("Normalize(%q): %v", tt.raw, err)
		}
		if got != tt.want {
			t.Fatalf("Normalize(%q)=%q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestNormalizeRejectsInvalidMode(t *testing.T) {
	if _, err := Normalize("invalid"); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("err=%v, want ErrInvalidMode", err)
	}
}
