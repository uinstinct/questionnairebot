//go:build integration

package commands_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aditya-mitra/questionnairebot/internal/commands"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

// TEST-07: /status reports name, last-answered, next-scheduled (in tz), and state
// (✅ Done / 🔄 In Progress / ⏳ Pending) for every questionnaire.
func TestStatusReportsAllQuestionnaireStates(t *testing.T) {
	dir := t.TempDir()
	sessions := session.NewManager(dir)

	utc := time.UTC
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	require.NoError(t, err)

	qs := map[string]*loader.Questionnaire{
		"done_q": {
			Slug: "done_q", Name: "Done Q", Schedule: "0 9 * * *",
			Timezone: "Asia/Kolkata", Location: kolkata,
			Questions: []loader.Question{{Question: "Q?"}},
		},
		"pending_q": {
			Slug: "pending_q", Name: "Pending Q", Schedule: "0 9 * * *",
			Timezone: "UTC", Location: utc,
			Questions: []loader.Question{{Question: "Q?"}},
		},
		"inprogress_q": {
			Slug: "inprogress_q", Name: "Inprogress Q", Schedule: "0 9 * * *",
			Timezone: "UTC", Location: utc,
			Questions: []loader.Question{{Question: "Q?"}},
		},
	}

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, utc)

	// Seed done_q with a completed entry in its tz.
	completedAt := now.Add(-3 * time.Hour)
	require.NoError(t, storage.PrependCompleted(dir, "done_q",
		completedAt, completedAt, kolkata,
		[]storage.AnswerPair{{Question: "Q?", Answer: "yes"}}))

	// Seed inprogress_q via the session manager (writes session.yaml under the lock).
	_, err = sessions.Start("inprogress_q", now, now, utc)
	require.NoError(t, err)
	require.NoError(t, sessions.RecordAnswer("inprogress_q", "Q?", "partial"))

	// pending_q has no answers and no session.

	s := commands.NewStatus(dir, sessions, qs, func() time.Time { return now })
	out := s.Render()

	require.True(t, strings.HasPrefix(out, "📊 Status:"), "must start with status header, got: %q", out)

	// Each questionnaire's display name appears exactly once.
	require.Contains(t, out, "Done Q |")
	require.Contains(t, out, "Pending Q |")
	require.Contains(t, out, "Inprogress Q |")

	// Per-state labels.
	require.Contains(t, lineFor(t, out, "Done Q"), "✅ Done")
	require.Contains(t, lineFor(t, out, "Pending Q"), "⏳ Pending")
	require.Contains(t, lineFor(t, out, "Inprogress Q"), "🔄 In Progress")

	// last= field: Never for pending, RFC3339 for the others.
	require.Contains(t, lineFor(t, out, "Pending Q"), "last=Never")
	require.Contains(t, lineFor(t, out, "Done Q"), "last=2026-05-18T")

	// next= field per row in the questionnaire's own timezone.
	doneLine := lineFor(t, out, "Done Q")
	require.Regexp(t, `next=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\+05:30`, doneLine,
		"Done Q row must format next= in Asia/Kolkata (+05:30): %s", doneLine)
	pendingLine := lineFor(t, out, "Pending Q")
	require.Regexp(t, `next=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`, pendingLine,
		"Pending Q row must format next= in UTC: %s", pendingLine)
}

func lineFor(t *testing.T, body, needle string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("no line containing %q in:\n%s", needle, body)
	return ""
}
