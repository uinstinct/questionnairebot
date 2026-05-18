//go:build integration

package loader_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aditya-mitra/questionnairebot/internal/loader"
)

// TEST-08: malformed questionnaire.yaml → loader.Load returns a *LoadError naming
// the file path and the reason. cmd/bot/main.go's fatal() wraps this as
// "FATAL: <path>: <reason>" before exiting 1.
func TestLoadFatalsOnMalformedQuestionnaire(t *testing.T) {
	t.Run("missing schedule field", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "bad_q"), 0o755))
		// Valid YAML, valid structure, but schedule is empty → cron parse must fail.
		yaml := "" +
			"name: Bad Q\n" +
			"timezone: UTC\n" +
			"schedule: \"\"\n" +
			"questions:\n" +
			"  - question: hello?\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "bad_q", "questionnaire.yaml"),
			[]byte(yaml), 0o644))

		_, err := loader.Load(dir)
		require.Error(t, err)

		var lerr *loader.LoadError
		require.True(t, errors.As(err, &lerr), "must be a *loader.LoadError, got %T", err)
		require.Equal(t, "data/bad_q/questionnaire.yaml", lerr.Path)
		require.Contains(t, strings.ToLower(lerr.Reason), "schedule")
		// fatal() in cmd/bot/main.go prints "FATAL: <err>"; assert the wrapped form too.
		require.Contains(t, err.Error(), "data/bad_q/questionnaire.yaml")
	})

	t.Run("invalid YAML syntax", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "broken"), 0o755))
		// Unterminated string → yaml parser error.
		yaml := "name: \"unterminated\nschedule: 0 9 * * *\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "broken", "questionnaire.yaml"),
			[]byte(yaml), 0o644))

		_, err := loader.Load(dir)
		require.Error(t, err)

		var lerr *loader.LoadError
		require.True(t, errors.As(err, &lerr))
		require.Equal(t, "data/broken/questionnaire.yaml", lerr.Path)
		require.Contains(t, strings.ToLower(lerr.Reason), "parse")
	})
}
