package loader

import "time"

// Question is a single prompt within a questionnaire.
type Question struct {
	Question string `yaml:"question"`
	Example  string `yaml:"example,omitempty"`
}

// Questionnaire is a YAML-defined recurring prompt set with a cron schedule.
type Questionnaire struct {
	Slug      string         `yaml:"-"`
	Name      string         `yaml:"name"`
	Schedule  string         `yaml:"schedule"`
	Timezone  string         `yaml:"timezone"`
	Questions []Question     `yaml:"questions"`
	Location  *time.Location `yaml:"-"`
}

// LoadError reports a failure to load or validate a specific questionnaire file.
type LoadError struct {
	Path   string
	Reason string
}

func (e *LoadError) Error() string {
	return e.Path + ": " + e.Reason
}
