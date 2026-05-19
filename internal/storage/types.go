package storage

// AnswerPair is one (question, answer) tuple within an answers.yaml entry.
type AnswerPair struct {
	Question string `yaml:"question"`
	Answer   string `yaml:"answer"`
}

// Entry is a single record in data/<slug>/answers.yaml.
type Entry struct {
	Status       string       `yaml:"status"`
	ScheduledFor string       `yaml:"scheduled_for"`
	CompletedAt  string       `yaml:"completed_at,omitempty"`
	SkippedAt    string       `yaml:"skipped_at,omitempty"`
	Answers      []AnswerPair `yaml:"answers,omitempty"`
}
