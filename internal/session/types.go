package session

import "github.com/aditya-mitra/questionnairebot/internal/storage"

type AnswerPair = storage.AnswerPair

type Session struct {
	QuestionnaireID      string       `yaml:"questionnaire_id"`
	ScheduledFor         string       `yaml:"scheduled_for"`
	StartedAt            string       `yaml:"started_at"`
	CurrentQuestionIndex int          `yaml:"current_question_index"`
	Answers              []AnswerPair `yaml:"answers,omitempty"`
}
