package config

import (
	"errors"
	"os"
)

type Config struct {
	Workspace      string
	Port           string
	AdminToken     string
	DeepSeekKey    string
	ZhipuKey       string
	TavilyKey      string
	DeepSeekURL    string
	DeepSeekModel  string
	ZhipuSearchURL string
	ScheduleDaily  string
}

func FromEnv() Config {
	return Config{
		Workspace:      getenv("WORKSPACE", "."),
		Port:           getenv("PORT", "8080"),
		AdminToken:     os.Getenv("ADMIN_TOKEN"),
		DeepSeekKey:    os.Getenv("DEEPSEEK_API_KEY"),
		ZhipuKey:       os.Getenv("ZHIPU_API_KEY"),
		TavilyKey:      os.Getenv("TAVILY_API_KEY"),
		DeepSeekURL:    getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com/chat/completions"),
		DeepSeekModel:  getenv("DEEPSEEK_MODEL", "deepseek-v4-flash"),
		ZhipuSearchURL: getenv("ZHIPU_SEARCH_URL", "https://open.bigmodel.cn/api/paas/v4/web_search"),
		ScheduleDaily:  os.Getenv("SCHEDULE_DAILY"),
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
