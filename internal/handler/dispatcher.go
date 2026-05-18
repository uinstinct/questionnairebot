package handler

import (
	"context"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
)

const HelpText = "Send /pull to start a questionnaire now, /status for state, or /list to see schedules."

type Dispatcher struct {
	Flow *QuestionFlow
}

func NewDispatcher(flow *QuestionFlow) *Dispatcher {
	return &Dispatcher{Flow: flow}
}

func (d *Dispatcher) Handle(ctx context.Context, sender bot.Sender, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}
	if update.Message.IsCommand() {
		d.handleCommand(sender, update.Message.Command())
		return
	}
	d.handleFreeText(sender, update.Message.Text)
}

func (d *Dispatcher) handleCommand(sender bot.Sender, cmd string) {
	switch cmd {
	case "start", "help":
		send(sender, HelpText)
	case "pull", "status", "list":
		send(sender, "Phase 4 will implement /"+cmd+".")
	default:
		send(sender, "Unknown command.")
	}
}

func (d *Dispatcher) handleFreeText(sender bot.Sender, text string) {
	active := d.activeSlugs()
	switch len(active) {
	case 0:
		send(sender, HelpText)
	case 1:
		if err := d.Flow.HandleAnswer(active[0], text); err != nil {
			log.Printf("handler: HandleAnswer(%s): %v", active[0], err)
		}
	default:
		send(sender, HelpText+"\nMultiple active sessions — use /pull to choose.")
	}
}

func (d *Dispatcher) activeSlugs() []string {
	var slugs []string
	for slug := range d.Flow.Questionnaires {
		if d.Flow.Sessions.Get(slug) != nil {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

func send(sender bot.Sender, text string) {
	if err := sender.Send(text); err != nil {
		log.Printf("handler: send: %v", err)
	}
}
