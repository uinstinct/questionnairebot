package commands

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

// maxPastDueSkips caps how many skipped entries a single past-due-skip pass can
// prepend, guarding against a fresh questionnaire whose baseline lookback would
// otherwise generate ~365 entries for a daily cron — and against typos in cron
// expressions producing dense schedules.
const maxPastDueSkips = 365

func parseSchedule(q *loader.Questionnaire) (cron.Schedule, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(q.Schedule)
	if err != nil {
		return nil, fmt.Errorf("commands: parse %q: %w", q.Schedule, err)
	}
	return sched, nil
}

// NextTrigger returns the first cron tick strictly after `after` in the
// questionnaire's timezone.
func NextTrigger(q *loader.Questionnaire, after time.Time) (time.Time, error) {
	sched, err := parseSchedule(q)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(after.In(q.Location)), nil
}

// ApplyPastDueSkips walks cron ticks between the baseline (last answers.yaml
// entry, or now-1year for fresh files) and `now`, prepending a `skipped` entry
// for each unanswered tick. Returns the number of skips prepended (clamped at
// maxPastDueSkips).
func ApplyPastDueSkips(dataDir string, q *loader.Questionnaire, now time.Time, clock func() time.Time) (int, error) {
	if clock == nil {
		clock = time.Now
	}
	sched, err := parseSchedule(q)
	if err != nil {
		return 0, err
	}
	last, err := storage.LastEntry(dataDir, q.Slug)
	if err != nil {
		return 0, fmt.Errorf("commands: last entry: %w", err)
	}
	var baseline time.Time
	if last != nil {
		baseline, err = time.Parse(time.RFC3339, last.ScheduledFor)
		if err != nil {
			return 0, fmt.Errorf("commands: parse last scheduled_for: %w", err)
		}
	} else {
		baseline = now.Add(-365 * 24 * time.Hour)
	}
	baseline = baseline.In(q.Location)
	nowLocal := now.In(q.Location)

	count := 0
	tick := sched.Next(baseline)
	for tick.Before(nowLocal) && count < maxPastDueSkips {
		if err := storage.PrependSkipped(dataDir, q.Slug, tick, clock(), q.Location); err != nil {
			return count, fmt.Errorf("commands: prepend skipped: %w", err)
		}
		count++
		tick = sched.Next(tick)
	}
	return count, nil
}
