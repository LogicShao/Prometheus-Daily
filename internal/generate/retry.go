package generate

import (
	"context"
	"errors"
	"time"

	"m-daily-news/internal/daily"
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   2 * time.Second,
}

func (c RetryConfig) normalized() RetryConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultRetryConfig.MaxAttempts
	}
	if c.BaseDelay < 0 {
		c.BaseDelay = 0
	}
	return c
}

func retryable(stage string, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, daily.ErrExists) ||
		errors.Is(err, daily.ErrInvalidDate) ||
		errors.Is(err, ErrRunning) {
		return false
	}
	if stage == "write" {
		return errors.Is(err, daily.ErrInvalidMarkdown)
	}
	return stage == "search" || stage == "llm"
}

func sleepRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
