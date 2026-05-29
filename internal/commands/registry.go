// Package commands defines the app-layer command catalog.
package commands

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// Commands returns the ordered list of user-facing slash commands to register
// with Telegram via setMyCommands. "start" is intentionally excluded — it is an
// internal Telegram lifecycle command, not a user-visible action in this bot.
func Commands() []tgbotapi.BotCommand {
	return []tgbotapi.BotCommand{
		{Command: "pull", Description: "Start a questionnaire now"},
		{Command: "status", Description: "Show current questionnaire state"},
		{Command: "list", Description: "List questionnaire schedules"},
		{Command: "help", Description: "Show available commands"},
	}
}
