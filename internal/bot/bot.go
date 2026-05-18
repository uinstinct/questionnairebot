// Package bot wraps the Telegram long-polling client.
//
// The Run loop drops unauthorised updates silently and dispatches the rest to
// a pluggable Dispatcher provided by the caller (typically internal/handler).
package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Sender is the subset of *Bot used by the dispatcher and question flow.
// Decoupled so unit tests can supply a recording sender without touching
// telegram-bot-api.
type Sender interface {
	Send(text string) error
	SendMarkdown(text string) error
	SendPicker(text string, options []PickerOption) error
	AckCallback(callbackID string) error
}

// PickerOption renders one inline-keyboard button on a picker message.
type PickerOption struct {
	Label        string
	CallbackData string
}

// Dispatcher receives every authorised update.
type Dispatcher interface {
	Handle(ctx context.Context, sender Sender, update tgbotapi.Update)
}

type Bot struct {
	API        *tgbotapi.BotAPI
	ChatID     int64
	dispatcher Dispatcher
}

func New(token string, chatID int64, dispatcher Dispatcher) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("bot: NewBotAPI: %w", err)
	}
	return &Bot{API: api, ChatID: chatID, dispatcher: dispatcher}, nil
}

func (b *Bot) Run(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := b.API.GetUpdatesChan(u)
	defer b.API.StopReceivingUpdates()

	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if !IsAuthorised(update, b.ChatID) {
				continue
			}
			b.dispatcher.Handle(ctx, b, update)
		}
	}
}

func (b *Bot) Send(text string) error {
	msg := tgbotapi.NewMessage(b.ChatID, text)
	_, err := b.API.Send(msg)
	return err
}

func (b *Bot) SendMarkdown(text string) error {
	msg := tgbotapi.NewMessage(b.ChatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err := b.API.Send(msg)
	return err
}

func (b *Bot) SendPicker(text string, options []PickerOption) error {
	if len(options) == 0 {
		return b.Send(text)
	}
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(options))
	for _, opt := range options {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(opt.Label, opt.CallbackData),
		))
	}
	msg := tgbotapi.NewMessage(b.ChatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, err := b.API.Send(msg)
	return err
}

func (b *Bot) AckCallback(callbackID string) error {
	_, err := b.API.Request(tgbotapi.NewCallback(callbackID, ""))
	return err
}
