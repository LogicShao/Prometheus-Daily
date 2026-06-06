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

type TavilyProvider struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

func NewTavilyProvider(apiKey string, client *http.Client) *TavilyProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &TavilyProvider{
		apiKey:  apiKey,
		client:  client,
		baseURL: "https://api.tavily.com/search",
	}
}

func (p *TavilyProvider) Name() string { return "tavily" }

func (p *TavilyProvider) Search(ctx context.Context, query string, opts Options) ([]Result, error) {
	if p.apiKey == "" {
		return nil, errors.New("TAVILY_API_KEY is required")
	}

	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 8
	}

	payload := map[string]any{
		"api_key":             p.apiKey,
		"query":               query,
		"max_results":         maxResults,
		"include_answer":      false,
		"include_raw_content": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("tavily status %d: %s", resp.StatusCode, string(data))
	}

	var decoded tavilyResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(decoded.Results))
	for _, item := range decoded.Results {
		results = append(results, Result{
			Title:    item.Title,
			URL:      item.URL,
			Snippet:  item.Content,
			Source:   sourceFromURL(item.URL, "tavily"),
			Provider: "tavily",
		})
	}
	return results, nil
}

type tavilyResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}
