package handler

import (
	"context"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
)

// HelpText is the default response for /start, /help, and unrecognised input.
const HelpText = "Send /pull to start a questionnaire now, /status for state, or /list to see schedules."

// CommandHandler abstracts the per-slash-command handlers so the dispatcher
// avoids importing internal/commands directly (which already depends on
// handler, so a back-import would create a cycle).
type CommandHandler interface {
	HandlePull(sender bot.Sender) error
	RenderStatus() string
	RenderList() string
	HandleStartCallback(sender bot.Sender, data string) error
}

// Dispatcher routes Telegram updates to slash-command handlers or the
// question-flow free-text handler.
type Dispatcher struct {
	Flow     *QuestionFlow
	Commands CommandHandler
}

// NewDispatcher constructs a Dispatcher with no commands attached.
func NewDispatcher(flow *QuestionFlow) *Dispatcher {
	return &Dispatcher{Flow: flow}
}

// Attach is called from main.go after the commands wiring is built. Optional —
// dispatchers without commands attached still handle text and slash-stubs.
func (d *Dispatcher) Attach(cmds CommandHandler) {
	d.Commands = cmds
}

// Handle routes one Telegram update — callback, slash-command, or free text.
func (d *Dispatcher) Handle(ctx context.Context, sender bot.Sender, update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		d.handleCallback(sender, update.CallbackQuery)
		return
	}
	if update.Message == nil {
		return
	}
	if update.Message.IsCommand() {
		d.handleCommand(sender, update.Message.Command())
		return
	}
	d.handleFreeText(sender, update.Message.Text)
}

func (d *Dispatcher) handleCallback(sender bot.Sender, cb *tgbotapi.CallbackQuery) {
	if err := sender.AckCallback(cb.ID); err != nil {
		log.Printf("handler: ack callback: %v", err)
	}
	if !strings.HasPrefix(cb.Data, "start:") {
		send(sender, "❌ Invalid selection.")
		return
	}
	if d.Commands == nil {
		send(sender, "❌ Invalid selection.")
		return
	}
	if err := d.Commands.HandleStartCallback(sender, cb.Data); err != nil {
		log.Printf("handler: callback start: %v", err)
	}
}

func (d *Dispatcher) handleCommand(sender bot.Sender, cmd string) {
	switch cmd {
	case "start", "help":
		send(sender, HelpText)
	case "pull":
		if d.Commands == nil {
			send(sender, "Phase 4 will implement /pull.")
			return
		}
		if err := d.Commands.HandlePull(sender); err != nil {
			log.Printf("handler: /pull: %v", err)
		}
	case "status":
		if d.Commands == nil {
			send(sender, "Phase 4 will implement /status.")
			return
		}
		send(sender, d.Commands.RenderStatus())
	case "list":
		if d.Commands == nil {
			send(sender, "Phase 4 will implement /list.")
			return
		}
		send(sender, d.Commands.RenderList())
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
