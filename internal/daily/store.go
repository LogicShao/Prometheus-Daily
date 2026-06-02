package daily

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Item struct {
	Date    string `json:"date"`
	Summary string `json:"summary"`
	File    string `json:"file"`
}

type Store struct {
	workspace string
}

var ErrExists = errors.New("daily already exists")

func NewStore(workspace string) *Store {
	return &Store{workspace: workspace}
}

func (s *Store) Dir() string {
	return filepath.Join(s.workspace, "content", "daily")
}

func (s *Store) Path(date string) (string, error) {
	if !IsDate(date) {
		return "", ErrInvalidDate
	}
	base := s.Dir()
	path := filepath.Join(base, date+".md")
	if filepath.Dir(path) != base {
		return "", ErrInvalidDate
	}
	return path, nil
}

func (s *Store) List() ([]Item, error) {
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Item{}, nil
		}
		return nil, err
	}

	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		date := strings.TrimSuffix(entry.Name(), ".md")
		if !IsDate(date) {
			continue
		}
		summary, _ := s.summary(filepath.Join(s.Dir(), entry.Name()))
		items = append(items, Item{Date: date, Summary: summary, File: entry.Name()})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Date > items[j].Date
	})
	return items, nil
}

func (s *Store) ReadRaw(date string) ([]byte, error) {
	path, err := s.Path(date)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (s *Store) Exists(date string) (bool, error) {
	path, err := s.Path(date)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (s *Store) WriteValidated(date, markdown string) (string, error) {
	return s.writeValidated(date, markdown, false)
}

func (s *Store) ReplaceValidated(date, markdown string) (string, error) {
	return s.writeValidated(date, markdown, true)
}

func (s *Store) writeValidated(date, markdown string, replace bool) (string, error) {
	target, err := s.Path(date)
	if err != nil {
		return "", err
	}
	if !replace {
		if exists, err := s.Exists(date); err != nil {
			return "", err
		} else if exists {
			return "", ErrExists
		}
	}

	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, "."+date+"-*.md")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(markdown); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := ValidateFile(tmpPath, date); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return "", err
	}
	return target, nil
}

func (s *Store) summary(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	fm, _, err := ParseFrontmatter(string(data))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(fm.Summary), nil
}

func ExtractSummary(raw string) string {
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		return ""
	}
	return fm.Summary
}

func FormatDateError(err error) error {
	if errors.Is(err, ErrInvalidDate) {
		return fmt.Errorf("%w: date must be YYYY-MM-DD", err)
	}
	return err
}
