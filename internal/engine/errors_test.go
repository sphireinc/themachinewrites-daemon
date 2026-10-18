package engine

import (
	"context"
	"errors"
	"testing"
)

func TestRetryClassification(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		action RetryAction
	}{
		{name: "quota", err: errors.New("HTTP 429 rate limit"), action: RetryAfterBackoff},
		{name: "timeout", err: context.DeadlineExceeded, action: RetryAfterBackoff},
		{name: "network", err: errors.New("temporary network unavailable"), action: RetryAfterBackoff},
		{name: "auth", err: errors.New("HTTP 401 unauthorized"), action: DisableAndAlert},
		{name: "config", err: errors.New("unknown configuration field"), action: DisableAndAlert},
		{name: "parse", err: ErrMalformedProposal, action: DoNotRetry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.err); got != tt.action {
				t.Fatalf("Classify(%v) = %q, want %q", tt.err, got, tt.action)
			}
		})
	}
}
