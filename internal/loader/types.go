package loader

import "time"

type Question struct {
	Question string `yaml:"question"`
	Example  string `yaml:"example,omitempty"`
}

type Questionnaire struct {
	Slug      string         `yaml:"-"`
	Name      string         `yaml:"name"`
	Schedule  string         `yaml:"schedule"`
	Timezone  string         `yaml:"timezone"`
	Questions []Question     `yaml:"questions"`
	Location  *time.Location `yaml:"-"`
}

type LoadError struct {
	Path   string
	Reason string
}

func (e *LoadError) Error() string {
	return e.Path + ": " + e.Reason
}
