package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type ZhipuProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewZhipuProvider(apiKey, baseURL string, client *http.Client) *ZhipuProvider {
	if client == nil {
		client = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4/web_search"
	}
	return &ZhipuProvider{apiKey: apiKey, baseURL: baseURL, client: client}
}

func (p *ZhipuProvider) Name() string { return "zhipu" }

func (p *ZhipuProvider) Search(ctx context.Context, query string, opts Options) ([]Result, error) {
	if p.apiKey == "" {
		return nil, errors.New("ZHIPU_API_KEY is required")
	}

	count := opts.MaxResults
	if count <= 0 {
		count = 8
	}

	payload := map[string]any{
		"search_engine":         "search_std",
		"search_query":          query,
		"count":                 count,
		"search_recency_filter": "oneWeek",
		"content_size":          "medium",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zhipu status %d: %s", resp.StatusCode, string(data))
	}

	var decoded zhipuResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}

	rawResults := decoded.SearchResult
	if len(rawResults) == 0 {
		rawResults = decoded.Results
	}

	results := make([]Result, 0, len(rawResults))
	for _, item := range rawResults {
		link := firstNonEmpty(item.Link, item.URL)
		snippet := firstNonEmpty(item.Content, item.Snippet)
		results = append(results, Result{
			Title:   item.Title,
			URL:     link,
			Snippet: snippet,
			Source:  "zhipu",
		})
	}
	return results, nil
}

type zhipuResponse struct {
	SearchResult []zhipuItem `json:"search_result"`
	Results      []zhipuItem `json:"results"`
}

type zhipuItem struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Snippet string `json:"snippet"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
