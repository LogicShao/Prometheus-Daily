package llm

import (
	"context"

	"m-daily-news/internal/search"
)

type Client interface {
	WriteDaily(ctx context.Context, date string, results []search.Result) (string, error)
}
