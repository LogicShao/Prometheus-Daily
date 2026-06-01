package daily

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidMarkdown = errors.New("invalid daily markdown")
	itemHeadingRE      = regexp.MustCompile(`(?m)^##\s+\S`)
	linkRE             = regexp.MustCompile(`https?://[^\s)]+`)
	htmlRE             = regexp.MustCompile(`(?i)<\s*/?\s*[a-z][^>]*>`)
	bulletRE           = regexp.MustCompile(`(?m)^\s*[-*]\s+\S`)
	itemLabels         = []string{"URL", "来源", "发布日期", "类型", "摘要", "为什么重要", "不确定性/风险"}
)

func ValidateFile(path, expectedDate string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Validate(string(data), expectedDate)
}

func Validate(raw, expectedDate string) error {
	if !utf8.ValidString(raw) {
		return fmt.Errorf("%w: not utf-8", ErrInvalidMarkdown)
	}
	if strings.Contains(raw, "\r\n") || strings.Contains(raw, "\r") {
		return fmt.Errorf("%w: line endings must be LF", ErrInvalidMarkdown)
	}

	fm, body, err := ParseFrontmatter(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMarkdown, err)
	}
	if fm.Date != expectedDate {
		return fmt.Errorf("%w: date mismatch", ErrInvalidMarkdown)
	}
	if strings.TrimSpace(fm.Summary) == "" {
		return fmt.Errorf("%w: summary required", ErrInvalidMarkdown)
	}
	if len([]rune(fm.Summary)) < 50 {
		return fmt.Errorf("%w: summary too short", ErrInvalidMarkdown)
	}
	if len([]rune(fm.Summary)) > 220 {
		return fmt.Errorf("%w: summary too long", ErrInvalidMarkdown)
	}
	if len(fm.Tags) == 0 {
		return fmt.Errorf("%w: tags required", ErrInvalidMarkdown)
	}
	if htmlRE.MatchString(body) {
		return fmt.Errorf("%w: raw html is not allowed", ErrInvalidMarkdown)
	}
	if strings.Contains(strings.ToLower(body), "javascript:") {
		return fmt.Errorf("%w: javascript links are not allowed", ErrInvalidMarkdown)
	}
	if bulletRE.MatchString(body) {
		return fmt.Errorf("%w: bullet lists are not allowed", ErrInvalidMarkdown)
	}
	sections := splitItemSections(body)
	if count := len(sections); count < 3 || count > 6 {
		return fmt.Errorf("%w: item count out of range", ErrInvalidMarkdown)
	}

	uniqueHosts := make(map[string]struct{})
	for _, section := range sections {
		for _, label := range itemLabels {
			if !hasItemLabel(section, label) {
				return fmt.Errorf("%w: item missing %s", ErrInvalidMarkdown, label)
			}
		}
		if len([]rune(labelValue(section, "摘要"))) < 60 {
			return fmt.Errorf("%w: item summary too short", ErrInvalidMarkdown)
		}
		links := linkRE.FindAllString(section, -1)
		if len(links) == 0 {
			return fmt.Errorf("%w: each item needs a link", ErrInvalidMarkdown)
		}
		for _, rawURL := range links {
			u, err := url.Parse(rawURL)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return fmt.Errorf("%w: invalid link", ErrInvalidMarkdown)
			}
			uniqueHosts[strings.ToLower(u.Host)] = struct{}{}
		}
	}
	if len(uniqueHosts) < 2 {
		return fmt.Errorf("%w: source diversity too low", ErrInvalidMarkdown)
	}

	return nil
}

func hasItemLabel(section, label string) bool {
	return strings.Contains(section, label+":") || strings.Contains(section, label+"：")
}

func labelValue(section, label string) string {
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		for _, sep := range []string{":", "："} {
			prefix := label + sep
			if strings.HasPrefix(line, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
	}
	return ""
}

func splitItemSections(body string) []string {
	indexes := itemHeadingRE.FindAllStringIndex(body, -1)
	if len(indexes) == 0 {
		return nil
	}
	sections := make([]string, 0, len(indexes))
	for i, idx := range indexes {
		start := idx[0]
		end := len(body)
		if i+1 < len(indexes) {
			end = indexes[i+1][0]
		}
		section := strings.TrimSpace(body[start:end])
		if section != "" {
			sections = append(sections, section)
		}
	}
	return sections
}
