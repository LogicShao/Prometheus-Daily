package version

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const Default = "0.2.0"

var semverRE = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:[-+][0-9A-Za-z.-]+)?$`)

func Read(workspace string) string {
	raw, err := os.ReadFile(filepath.Join(workspace, "VERSION"))
	if err != nil {
		return Default
	}
	v := strings.TrimSpace(string(raw))
	if IsValid(v) {
		return v
	}
	return Default
}

func IsValid(v string) bool {
	return semverRE.MatchString(strings.TrimSpace(v))
}
