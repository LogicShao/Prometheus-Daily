package daily

import (
	"errors"
	"strings"
)

type Frontmatter struct {
	Date    string
	Summary string
	Tags    []string
}

var ErrFrontmatter = errors.New("invalid frontmatter")

func ParseFrontmatter(raw string) (Frontmatter, string, error) {
	if !strings.HasPrefix(raw, "---\n") {
		return Frontmatter{}, "", ErrFrontmatter
	}

	rest := raw[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return Frontmatter{}, "", ErrFrontmatter
	}

	block := rest[:end]
	body := rest[end+len("\n---\n"):]
	var fm Frontmatter

	for _, line := range strings.Split(block, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		switch strings.TrimSpace(key) {
		case "date":
			fm.Date = value
		case "summary":
			fm.Summary = value
		case "tags":
			fm.Tags = parseTags(value)
		}
	}

	return fm, body, nil
}

func parseTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		tag = strings.Trim(tag, `"'`)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
