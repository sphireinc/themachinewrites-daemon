package engine

import (
	"context"
	"errors"
	"strings"
)

type RetryAction string

const (
	RetryAfterBackoff RetryAction = "retry_after_backoff"
	DisableAndAlert   RetryAction = "disable_and_alert"
	DoNotRetry        RetryAction = "do_not_retry"
)

func Classify(err error) RetryAction {
	if err == nil {
		return DoNotRetry
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrMalformedProposal) || errors.Is(err, ErrInvalidProposal) {
		if errors.Is(err, ErrMalformedProposal) || errors.Is(err, ErrInvalidProposal) {
			return DoNotRetry
		}
		return RetryAfterBackoff
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"429", "rate limit", "temporary network", "connection reset", "timeout"} {
		if strings.Contains(message, marker) {
			return RetryAfterBackoff
		}
	}
	for _, marker := range []string{"401", "403", "unauthorized", "authentication", "configuration", "executable not found"} {
		if strings.Contains(message, marker) {
			return DisableAndAlert
		}
	}
	return DoNotRetry
}
