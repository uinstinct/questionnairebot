// Package loader reads questionnaire YAML definitions from the data directory.
package loader

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

// Load scans dataDir for subdirectories containing a questionnaire.yaml,
// validates each, and returns them sorted by slug.
func Load(dataDir string) ([]*Questionnaire, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, &LoadError{Path: dataDir, Reason: err.Error()}
	}

	var loaded []*Questionnaire
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		yamlPath := filepath.Join(dataDir, slug, "questionnaire.yaml")
		relPath := "data/" + slug + "/questionnaire.yaml"

		raw, err := os.ReadFile(yamlPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, &LoadError{Path: relPath, Reason: err.Error()}
		}

		var q Questionnaire
		if err := yaml.Unmarshal(raw, &q); err != nil {
			return nil, &LoadError{Path: relPath, Reason: "parse: " + err.Error()}
		}

		if strings.TrimSpace(q.Name) == "" {
			return nil, &LoadError{Path: relPath, Reason: "name is required"}
		}

		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(q.Schedule); err != nil {
			return nil, &LoadError{Path: relPath, Reason: "invalid schedule: " + err.Error()}
		}

		if strings.TrimSpace(q.Timezone) == "" {
			return nil, &LoadError{Path: relPath, Reason: "timezone is required"}
		}
		loc, err := time.LoadLocation(q.Timezone)
		if err != nil {
			return nil, &LoadError{Path: relPath, Reason: "invalid timezone: " + err.Error()}
		}
		q.Location = loc

		if len(q.Questions) == 0 {
			return nil, &LoadError{Path: relPath, Reason: "questions is required and must be non-empty"}
		}
		for i, item := range q.Questions {
			if strings.TrimSpace(item.Question) == "" {
				return nil, &LoadError{Path: relPath, Reason: fmt.Sprintf("questions[%d]: question text is required", i)}
			}
		}

		q.Slug = slug
		loaded = append(loaded, &q)
	}

	if len(loaded) == 0 {
		return nil, &LoadError{Path: dataDir, Reason: "no questionnaires found"}
	}

	sort.Slice(loaded, func(i, j int) bool { return loaded[i].Slug < loaded[j].Slug })

	slugs := make([]string, len(loaded))
	for i, q := range loaded {
		slugs[i] = q.Slug
	}
	log.Printf("Loaded %d questionnaire(s): [%s]", len(loaded), strings.Join(slugs, ", "))

	return loaded, nil
}
