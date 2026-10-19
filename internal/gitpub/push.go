package gitpub

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (r *Repository) PushFastForward(ctx context.Context, maxAttempts int) (string, error) {
	if maxAttempts < 1 || maxAttempts > 5 {
		return "", errors.New("push attempts must be 1–5")
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		head, err := r.run(ctx, "rev-parse", "HEAD")
		if err != nil {
			return "", err
		}
		_, err = r.run(ctx, "push", "--porcelain", r.Remote, "HEAD:refs/heads/"+r.Branch)
		if err == nil {
			return strings.TrimSpace(head), nil
		}
		lastErr = err
		if attempt+1 < maxAttempts {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
			}
		}
	}
	return "", lastErr
}
