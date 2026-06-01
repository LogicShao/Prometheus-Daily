package daily

import (
	"errors"
	"regexp"
	"time"
)

var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

var ErrInvalidDate = errors.New("invalid date")

func NormalizeDate(raw string, now time.Time) (string, error) {
	if raw == "" {
		return now.Format("2006-01-02"), nil
	}
	if !IsDate(raw) {
		return "", ErrInvalidDate
	}
	return raw, nil
}

func IsDate(raw string) bool {
	if !dateRE.MatchString(raw) {
		return false
	}
	_, err := time.Parse("2006-01-02", raw)
	return err == nil
}
