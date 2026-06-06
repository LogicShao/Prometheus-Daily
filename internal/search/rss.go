package search

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type RSSProvider struct {
	client *http.Client
	feeds  []string
	now    func() time.Time
}

func NewRSSProvider(client *http.Client, feeds []string) *RSSProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &RSSProvider{client: client, feeds: feeds, now: time.Now}
}

func (p *RSSProvider) Name() string { return "rss" }

func (p *RSSProvider) Search(ctx context.Context, _ string, opts Options) ([]Result, error) {
	if len(p.feeds) == 0 {
		return nil, errors.New("rss feeds not configured")
	}

	var results []Result
	var lastErr error
	for _, feedURL := range p.feeds {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		closeErr := resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = errors.New(resp.Status)
			continue
		}
		items := parseFeed(body, feedURL, opts)
		results = append(results, items...)
	}
	if len(results) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return results, nil
}

type rssFeed struct {
	Title   string `xml:"title"`
	Channel struct {
		Title string    `xml:"title"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Summary string     `xml:"summary"`
	Updated string     `xml:"updated"`
	Links   []atomLink `xml:"link"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

func parseFeed(data []byte, source string, opts Options) []Result {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil
	}

	sourceName := strings.TrimSpace(firstNonEmpty(feed.Channel.Title, feed.Title, source))
	var results []Result
	for _, item := range feed.Channel.Items {
		published := parseTime(item.PubDate)
		if !published.IsZero() && !opts.Since.IsZero() && published.Before(opts.Since) {
			continue
		}
		results = append(results, Result{
			Title:       strings.TrimSpace(item.Title),
			URL:         strings.TrimSpace(item.Link),
			Snippet:     strings.TrimSpace(stripSpace(item.Description)),
			Source:      sourceName,
			Provider:    "rss",
			PublishedAt: published,
		})
	}
	for _, entry := range feed.Entries {
		published := parseTime(entry.Updated)
		if !published.IsZero() && !opts.Since.IsZero() && published.Before(opts.Since) {
			continue
		}
		results = append(results, Result{
			Title:       strings.TrimSpace(entry.Title),
			URL:         atomURL(entry.Links),
			Snippet:     strings.TrimSpace(stripSpace(entry.Summary)),
			Source:      sourceName,
			Provider:    "rss",
			PublishedAt: published,
		})
	}

	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		return results[:opts.MaxResults]
	}
	return results
}

func atomURL(links []atomLink) string {
	for _, link := range links {
		if link.Rel == "" || link.Rel == "alternate" {
			return strings.TrimSpace(link.Href)
		}
	}
	if len(links) > 0 {
		return strings.TrimSpace(links[0].Href)
	}
	return ""
}

func parseTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{time.RFC1123Z, time.RFC1123, time.RFC3339, "Mon, 02 Jan 2006 15:04:05 MST"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func stripSpace(raw string) string {
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.ReplaceAll(raw, "\t", " ")
	return strings.Join(strings.Fields(raw), " ")
}
