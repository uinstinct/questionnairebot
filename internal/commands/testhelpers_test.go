package commands

import (
	"sync"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
)

type pickerCall struct {
	Text    string
	Options []bot.PickerOption
}

type recordingSender struct {
	mu       sync.Mutex
	msgs     []string
	markdown []string
	pickers  []pickerCall
	acks     []string
}

func (r *recordingSender) Send(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, text)
	return nil
}

func (r *recordingSender) SendMarkdown(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markdown = append(r.markdown, text)
	return nil
}

func (r *recordingSender) SendPicker(text string, options []bot.PickerOption) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pickers = append(r.pickers, pickerCall{Text: text, Options: append([]bot.PickerOption(nil), options...)})
	return nil
}

func (r *recordingSender) AckCallback(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.acks = append(r.acks, id)
	return nil
}

func (r *recordingSender) snapshot() (msgs []string, markdown []string, pickers []pickerCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	msgs = append([]string(nil), r.msgs...)
	markdown = append([]string(nil), r.markdown...)
	pickers = append([]pickerCall(nil), r.pickers...)
	return
}
