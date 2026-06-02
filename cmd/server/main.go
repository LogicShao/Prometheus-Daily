package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"m-daily-news/internal/config"
	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
	"m-daily-news/internal/httpapi"
	"m-daily-news/internal/llm"
	"m-daily-news/internal/scheduler"
	"m-daily-news/internal/search"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(log.Writer(), &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg := config.FromEnv()
	if err := cfg.ValidateRuntime(); err != nil {
		log.Fatal(err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	store := daily.NewStore(cfg.Workspace)
	searcher := search.NewService(
		[]search.Provider{
			search.NewRSSProvider(client, defaultFeeds()),
			search.NewZhipuProvider(cfg.ZhipuKey, cfg.ZhipuSearchURL, client),
		},
		[]search.Provider{
			search.NewTavilyProvider(cfg.TavilyKey, client),
		},
	)

	prompt, err := generate.LoadPrompt(filepath.Join(cfg.Workspace, "prompt.md"))
	if err != nil {
		log.Fatal(err)
	}
	llmClient := llm.NewDeepSeekClient(cfg.DeepSeekKey, cfg.DeepSeekURL, cfg.DeepSeekModel, prompt, client)
	runner := generate.NewRunner(store, searcher, llmClient)
	router := httpapi.NewRouter(store, runner, cfg.AdminToken, cfg.Workspace, time.Now())
	if cfg.ScheduleDaily != "" {
		at, err := scheduler.ParseTimeOfDay(cfg.ScheduleDaily)
		if err != nil {
			log.Fatal(err)
		}
		scheduler.NewDaily(runner, at).Start(context.Background())
		slog.Info("daily scheduler enabled", "time", cfg.ScheduleDaily)
	}

	slog.Info("m-daily-news listening", "port", cfg.Port, "workspace", cfg.Workspace)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, router))
}

func defaultFeeds() []string {
	return []string{
		"https://export.arxiv.org/rss/cs.AI",
		"https://export.arxiv.org/rss/cs.CL",
		"https://github.blog/changelog/feed/",
		"https://openai.com/news/rss.xml",
	}
}
