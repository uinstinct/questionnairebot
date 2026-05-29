//go:build integration

// This test mutates global state on the test bot: setMyCommands overwrites
// whatever was previously registered. Acceptable for a dedicated test bot.
package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/commands"
)

// TestE2ERegisterCommands proves that setMyCommands round-trips against the
// real test bot: after calling RegisterCommands, GetMyCommands returns exactly
// the four commands defined in the app-layer catalog (pull, status, list, help)
// in registry order.
func TestE2ERegisterCommands(t *testing.T) {
	token, chatID := requireTestEnv(t)

	// nil dispatcher is fine — Run is never started.
	b, err := bot.New(token, chatID, nil)
	require.NoError(t, err)

	require.NoError(t, b.RegisterCommands(commands.Commands()))

	got, err := b.API.GetMyCommands()
	require.NoError(t, err)

	want := commands.Commands()
	require.Equal(t, len(want), len(got), "expected %d commands, got %d", len(want), len(got))
	for i, w := range want {
		require.Equal(t, w.Command, got[i].Command, "command[%d] name mismatch", i)
		require.Equal(t, w.Description, got[i].Description, "command[%d] description mismatch", i)
	}
}
