package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

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
			{Role: "user", Content: buildUserPrompt(date, results)},
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

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, string(data))
	}

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

func buildUserPrompt(date string, results []search.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "请根据以下搜索结果生成 %s 的 AI 技术日报 Markdown。\n", date)
	fmt.Fprintf(&b, "优先选择跨类型、跨来源的 3 到 6 条内容，避免同一公司或同一域名占比过高。\n\n")
	for i, result := range results {
		if i >= 20 {
			break
		}
		fmt.Fprintf(&b, "%d. %s\nURL: %s\nSource: %s\n",
			i+1, result.Title, result.URL, result.Source)
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
