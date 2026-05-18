package handler

import (
	"fmt"
	"log"
)

// Restore rehydrates in-progress sessions from disk on bot startup.
// If a restored session is already fully answered (crash after writing the last
// answer but before finalisation), finalise it immediately (PRD US-008 AC-3).
func Restore(flow *QuestionFlow) error {
	for slug, q := range flow.Questionnaires {
		s, err := flow.Sessions.LoadFromDisk(slug)
		if err != nil {
			return fmt.Errorf("restore %s: %w", slug, err)
		}
		if s == nil {
			continue
		}
		if s.CurrentQuestionIndex >= len(q.Questions) {
			if _, err := flow.FinalizeIfDone(slug); err != nil {
				return fmt.Errorf("finalize orphan %s: %w", slug, err)
			}
			log.Printf("Finalised orphan session: %s", slug)
			continue
		}
		log.Printf("Restored session: %s (q=%d/%d)", slug, s.CurrentQuestionIndex+1, len(q.Questions))
	}
	return nil
}
