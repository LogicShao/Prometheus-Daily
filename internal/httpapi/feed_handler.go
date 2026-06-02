package httpapi

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"
)

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string        `xml:"title"`
	Link          string        `xml:"link"`
	Description   string        `xml:"description"`
	LastBuildDate string        `xml:"lastBuildDate,omitempty"`
	Items         []rssFeedItem `xml:"item"`
}

type rssFeedItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	Description string  `xml:"description"`
	PubDate     string  `xml:"pubDate,omitempty"`
	GUID        rssGUID `xml:"guid"`
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

func (s *Server) Feed(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Success: false, Error: err.Error()})
		return
	}

	baseURL := requestBaseURL(r)
	feedItems := make([]rssFeedItem, 0, len(items))
	for _, item := range items {
		link := baseURL + "/api/daily/" + item.Date + "/raw"
		feedItems = append(feedItems, rssFeedItem{
			Title:       "Prometheus Daily " + item.Date,
			Link:        link,
			Description: item.Summary,
			PubDate:     rssDate(item.Date),
			GUID:        rssGUID{IsPermaLink: "true", Value: link},
		})
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       "Prometheus Daily",
			Link:        baseURL + "/",
			Description: "AI 技术日报更新",
			Items:       feedItems,
		},
	}
	if len(feedItems) > 0 {
		feed.Channel.LastBuildDate = feedItems[0].PubDate
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(feed)
}

func rssDate(date string) string {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return ""
	}
	return t.Format(time.RFC1123Z)
}

func requestBaseURL(r *http.Request) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}
