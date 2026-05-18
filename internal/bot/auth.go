package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// IsAuthorised returns true only when the update originates from the configured
// chat ID. Mismatches are dropped silently — no log line, no reply — per
// PRD FR-14 / US-012.
func IsAuthorised(update tgbotapi.Update, chatID int64) bool {
	chat := update.FromChat()
	if chat == nil {
		return false
	}
	return chat.ID == chatID
}
