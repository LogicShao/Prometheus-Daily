package daily

import (
	"errors"
	"strings"
)

type Frontmatter struct {
	Date       string
	AppVersion string
	Summary    string
	Tags       []string
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
		case "app_version":
			fm.AppVersion = value
		case "summary":
			fm.Summary = value
		case "tags":
			fm.Tags = parseTags(value)
		}
	}

	return fm, body, nil
}

func InjectAppVersion(raw, appVersion string) (string, error) {
	if !strings.HasPrefix(raw, "---\n") {
		return "", ErrFrontmatter
	}

	rest := raw[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", ErrFrontmatter
	}

	block := rest[:end]
	body := rest[end+len("\n---\n"):]
	lines := strings.Split(block, "\n")
	out := make([]string, 0, len(lines)+1)
	inserted := false

	for _, line := range lines {
		key, _, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "app_version" {
			if !inserted {
				out = append(out, "app_version: "+appVersion)
				inserted = true
			}
			continue
		}

		out = append(out, line)
		if ok && strings.TrimSpace(key) == "date" && !inserted {
			out = append(out, "app_version: "+appVersion)
			inserted = true
		}
	}

	if !inserted {
		out = append(out, "app_version: "+appVersion)
	}
	return "---\n" + strings.Join(out, "\n") + "\n---\n" + body, nil
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
