package controllers

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// authCredentialSourceFiles are operator auth paths that must never log credential values.
var authCredentialSourceFiles = []string{
	"controllers/controlplanes/k8s.go",
	"controllers/controlplanes/reconcile.go",
	"controllers/controlplanes/auth_bootstrap.go",
	"internal/auth/controllerlogin/embedded.go",
	"internal/auth/controllerlogin/external.go",
}

// logCredentialPatterns match logging calls that would emit credential variables.
var logCredentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\blog\.\w+\([^)]*\bpassword\b`),
	regexp.MustCompile(`\blog\.\w+\([^)]*\bPassword\b`),
	regexp.MustCompile(`\blog\.\w+\([^)]*clientSecret\b`),
	regexp.MustCompile(`\blog\.\w+\([^)]*Client\.Secret\b`),
	regexp.MustCompile(`fmt\.Sprintf\([^)]*,\s*password\b`),
	regexp.MustCompile(`fmt\.Sprintf\([^)]*,\s*clientSecret\b`),
}

func TestAuthSourcesDoNotLogCredentialValues(t *testing.T) {
	root := moduleRoot(t)

	for _, rel := range authCredentialSourceFiles {
		path := filepath.Join(root, rel)
		lines, err := readSourceLines(path)
		require.NoError(t, err, "read %s", rel)

		for lineNum, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			for _, pattern := range logCredentialPatterns {
				if pattern.MatchString(line) {
					t.Errorf("%s:%d: potential credential logging: %s", rel, lineNum+1, trimmed)
				}
			}
		}
	}
}

func TestLoginAuthLogMessagesAreStatic(t *testing.T) {
	// loginIofogClient must only emit fixed messages; credentials are never log fields.
	require.Contains(t, readFileString(t, "controllers/controlplanes/k8s.go"),
		`r.log.Info("Logging in to Controller with bootstrap credentials")`)
	require.Contains(t, readFileString(t, "controllers/controlplanes/k8s.go"),
		`r.log.Info("Generating Client Access Token")`)
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func readSourceLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func readFileString(t *testing.T, rel string) string {
	t.Helper()

	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)
	return string(data)
}
