package commands

import "github.com/aditya-mitra/questionnairebot/internal/bot"

// Adapter bundles the four per-slash-command handlers into the
// handler.CommandHandler shape without forcing internal/handler to import
// internal/commands (which would be cyclic).
type Adapter struct {
	Pull   *Pull
	Status *Status
	List   *List
}

// NewAdapter wires the per-command handlers into a single handler.CommandHandler.
func NewAdapter(p *Pull, s *Status, l *List) *Adapter {
	return &Adapter{Pull: p, Status: s, List: l}
}

// HandlePull dispatches the /pull slash-command to the underlying handler.
func (a *Adapter) HandlePull(sender bot.Sender) error {
	return a.Pull.Handle(sender)
}

// RenderStatus returns the /status message body.
func (a *Adapter) RenderStatus() string {
	return a.Status.Render()
}

// RenderList returns the /list message body.
func (a *Adapter) RenderList() string {
	return a.List.Render()
}

// HandleStartCallback dispatches an inline-keyboard "start:" callback to /pull.
func (a *Adapter) HandleStartCallback(sender bot.Sender, data string) error {
	return a.Pull.HandleCallback(sender, data)
}
