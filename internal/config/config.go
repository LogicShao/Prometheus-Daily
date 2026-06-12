package config

import (
	"errors"
	"os"
	"strings"

	"m-daily-news/internal/reportmode"
)

type Config struct {
	Workspace                string
	Port                     string
	AdminToken               string
	DeepSeekKey              string
	ZhipuKey                 string
	TavilyKey                string
	DeepSeekURL              string
	DeepSeekModel            string
	ZhipuSearchURL           string
	ScheduleDaily            string
	ReportMode               reportmode.Mode
	LowPriorityKeywords      []string
	LowPriorityURLSubstrings []string
	LowPriorityDomains       []string
	HighValueKeywords        []string
}

func FromEnv() Config {
	reportMode, err := reportmode.Normalize(os.Getenv("DAILY_REPORT_MODE"))
	if err != nil {
		reportMode = ""
	}
	return Config{
		Workspace:                getenv("WORKSPACE", "."),
		Port:                     getenv("PORT", "8080"),
		AdminToken:               os.Getenv("ADMIN_TOKEN"),
		DeepSeekKey:              os.Getenv("DEEPSEEK_API_KEY"),
		ZhipuKey:                 os.Getenv("ZHIPU_API_KEY"),
		TavilyKey:                os.Getenv("TAVILY_API_KEY"),
		DeepSeekURL:              getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com/chat/completions"),
		DeepSeekModel:            getenv("DEEPSEEK_MODEL", "deepseek-v4-flash"),
		ZhipuSearchURL:           getenv("ZHIPU_SEARCH_URL", "https://open.bigmodel.cn/api/paas/v4/web_search"),
		ScheduleDaily:            os.Getenv("SCHEDULE_DAILY"),
		ReportMode:               reportMode,
		LowPriorityKeywords:      splitList(os.Getenv("DAILY_LOW_PRIORITY_KEYWORDS")),
		LowPriorityURLSubstrings: splitList(os.Getenv("DAILY_LOW_PRIORITY_URLS")),
		LowPriorityDomains:       splitList(os.Getenv("DAILY_LOW_PRIORITY_DOMAINS")),
		HighValueKeywords:        splitList(os.Getenv("DAILY_HIGH_VALUE_KEYWORDS")),
	}
}

func (c Config) ValidateRuntime() error {
	switch {
	case c.AdminToken == "":
		return errors.New("ADMIN_TOKEN is required")
	case c.DeepSeekKey == "":
		return errors.New("DEEPSEEK_API_KEY is required")
	case c.ZhipuKey == "":
		return errors.New("ZHIPU_API_KEY is required")
	case c.ReportMode == "":
		return errors.New("DAILY_REPORT_MODE must be one of: " + reportmode.AllowedValues())
	default:
		return nil
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
