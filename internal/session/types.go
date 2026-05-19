package session

import "github.com/aditya-mitra/questionnairebot/internal/storage"

// AnswerPair re-exports storage.AnswerPair so session callers can use a stable
// name without importing storage.
type AnswerPair = storage.AnswerPair

// Session is the in-progress questionnaire state persisted under
// data/<slug>/session.yaml.
type Session struct {
	QuestionnaireID      string       `yaml:"questionnaire_id"`
	ScheduledFor         string       `yaml:"scheduled_for"`
	StartedAt            string       `yaml:"started_at"`
	CurrentQuestionIndex int          `yaml:"current_question_index"`
	Answers              []AnswerPair `yaml:"answers,omitempty"`
}
