package reportmode

import (
	"errors"
	"strings"
)

type Mode string

const (
	Balanced Mode = "balanced"
	Research Mode = "research"
)

var ErrInvalidMode = errors.New("invalid report mode")

func Normalize(raw string) (Mode, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return Balanced, nil
	}
	switch Mode(mode) {
	case Balanced, Research:
		return Mode(mode), nil
	default:
		return "", ErrInvalidMode
	}
}

func Default(mode Mode) Mode {
	normalized, err := Normalize(string(mode))
	if err != nil {
		return Balanced
	}
	return normalized
}

func AllowedValues() string {
	return string(Balanced) + ", " + string(Research)
}
