package publication

import (
	"errors"
	"fmt"
	"time"
)

func ValidateCorrection(posts []PostRecord, newID, targetID string) error {
	if newID == targetID {
		return errors.New("correction cannot target itself")
	}
	var target *PostRecord
	for index := range posts {
		if posts[index].ID == targetID {
			target = &posts[index]
			break
		}
	}
	if target == nil {
		return errors.New("correction target does not exist")
	}
	var current *PostRecord
	for index := range posts {
		if posts[index].ID == newID {
			current = &posts[index]
			break
		}
	}
	if current != nil && !target.PublishedAt.Before(current.PublishedAt) {
		return errors.New("correction target must be earlier than correction")
	}
	if current == nil {
		return errors.New("correction draft must exist before target ordering can be checked")
	}
	return ValidateCorrectionByTime(target.PublishedAt, current.PublishedAt)
}

func ValidateCorrectionByTime(targetAt, newAt time.Time) error {
	if !targetAt.Before(newAt) {
		return fmt.Errorf("correction target must precede new publication")
	}
	return nil
}
