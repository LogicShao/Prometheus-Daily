package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"m-daily-news/internal/reportmode"
	"m-daily-news/internal/search"
)

type DeepSeekClient struct {
	apiKey  string
	baseURL string
	model   string
	prompt  string
	client  *http.Client
}

func NewDeepSeekClient(apiKey, baseURL, model, prompt string, client *http.Client) *DeepSeekClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &DeepSeekClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		prompt:  prompt,
		client:  client,
	}
}

func (c *DeepSeekClient) WriteDaily(ctx context.Context, date string, results []search.Result) (string, error) {
	return c.WriteDailyWithMode(ctx, date, reportmode.Balanced, results)
}

func (c *DeepSeekClient) WriteDailyWithMode(ctx context.Context, date string, mode reportmode.Mode, results []search.Result) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("DEEPSEEK_API_KEY is required")
	}
	if c.baseURL == "" {
		return "", errors.New("LLM base URL is required")
	}
	if c.model == "" {
		return "", errors.New("LLM model is required")
	}

	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: c.prompt},
			{Role: "user", Content: buildUserPrompt(date, mode, results)},
		},
		Temperature: 0.3,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	slog.Info("llm request started", "model", c.model, "date", date, "results", len(results))
	resp, err := c.client.Do(req)
	if err != nil {
		slog.Error("llm request failed", "model", c.model, "date", date, "duration", time.Since(start).String(), "error", err.Error())
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		slog.Error("llm response read failed", "model", c.model, "date", date, "status", resp.StatusCode, "duration", time.Since(start).String(), "error", err.Error())
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("llm request returned non-2xx", "model", c.model, "date", date, "status", resp.StatusCode, "duration", time.Since(start).String(), "bytes", len(data))
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, string(data))
	}
	slog.Info("llm request completed", "model", c.model, "date", date, "status", resp.StatusCode, "duration", time.Since(start).String(), "bytes", len(data))

	var decoded chatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("llm returned no choices")
	}
	content := stripMarkdownFence(strings.TrimSpace(decoded.Choices[0].Message.Content))
	if content == "" {
		return "", errors.New("llm returned empty content")
	}
	return content, nil
}

func stripMarkdownFence(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "```") {
		return content
	}
	firstLineEnd := strings.Index(content, "\n")
	if firstLineEnd < 0 {
		return content
	}
	content = strings.TrimSpace(content[firstLineEnd+1:])
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSpace(strings.TrimSuffix(content, "```"))
	}
	return content
}

func buildUserPrompt(date string, mode reportmode.Mode, results []search.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "请根据以下搜索结果生成 %s 的 AI 技术日报 Markdown。\n", date)
	fmt.Fprintf(&b, "当前生成模式: %s。\n", mode)
	if mode == reportmode.Research {
		fmt.Fprintf(&b, "研究优先模式要求：正文优先选择 arXiv 或其他研究论文，目标 2 到 4 条研究内容；产品、开源、安全只保留与研究落地、模型能力、Agent 基准或安全评估直接相关的内容；默认不要选择产业、政策、合作、融资、榜单或市场报告。\n")
	} else {
		fmt.Fprintf(&b, "均衡模式要求：优先选择跨类型、跨来源的 3 到 6 条内容，主体必须聚焦 AI 技术、工程实现、开发者工具、开源、研究、安全或基础设施，至少选择 1 条研究或 arXiv 内容，避免同一公司或同一域名占比过高。\n")
		fmt.Fprintf(&b, "产业、政策、合作、融资、榜单、市场报告、垂直行业应用最多选择 1 条，并放在正文最后；如果技术细节不足，标题使用 `附加新闻：` 前缀。\n")
	}
	fmt.Fprintf(&b, "来源要求：优先使用原始来源、官方 changelog、官方博客、GitHub 仓库或 arXiv 原文；不要把 zhipu、tavily 写成来源；如果只能使用二手转载或聚合站，必须在不确定性/风险中说明来源可信度限制。\n\n")
	fmt.Fprintf(&b, "输出风格要求：frontmatter 的 summary 要写成完整摘要，控制在 80 到 160 个汉字，最多 220 个汉字，不要逐条罗列全部新闻；正文用 `# 日报 YYYY-MM-DD` + `## 标题` 的段落式结构，不要使用有序列表或项目符号列表。\n")
	fmt.Fprintf(&b, "frontmatter 必须严格使用单行格式：`---`、`date: %s`、`summary: 一段不换行中文摘要`、`tags: [AI, 日报, 研究]`、`---`；不要使用多行 YAML、缩进数组或对象。`app_version` 由服务写入前补充，不要自行编写。\n", date)
	fmt.Fprintf(&b, "每条新闻都要用自然段写清楚事实、摘要、为什么重要、不确定性/风险，信息要比搜索结果更完整。\n")
	fmt.Fprintf(&b, "每条新闻必须逐行包含固定标签，标签文字不能改写或省略：`URL:`、`来源:`、`发布日期:`、`类型:`、`摘要:`、`为什么重要:`、`不确定性/风险:`。\n")
	fmt.Fprintf(&b, "最终输出前自检：frontmatter 有 date、summary、tags，且没有 app_version；正文有 3 到 6 个 `##` 条目；每个条目都有 URL、来源、发布日期、类型、摘要、为什么重要、不确定性/风险。\n\n")
	for i, result := range results {
		if i >= 20 {
			break
		}
		fmt.Fprintf(&b, "%d. %s\nURL: %s\nSource: %s\n",
			i+1, result.Title, result.URL, result.Source)
		if result.Provider != "" {
			fmt.Fprintf(&b, "SearchProvider: %s\n", result.Provider)
		}
		if result.Category != "" {
			fmt.Fprintf(&b, "Category: %s\n", result.Category)
		}
		if !result.PublishedAt.IsZero() {
			fmt.Fprintf(&b, "Published: %s\n", result.PublishedAt.Format("2006-01-02 15:04"))
		}
		fmt.Fprintf(&b, "Snippet: %s\n\n", result.Snippet)
	}
	return b.String()
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
