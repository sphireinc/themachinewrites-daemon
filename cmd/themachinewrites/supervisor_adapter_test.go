package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
)

func TestRunBacksOffFailureStormsAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	waits := []time.Duration{}
	adapter := loopAdapter{
		cycle:    supervisor.Cycle{Poll: func(context.Context) []error { attempts++; return []error{errors.New("failure")} }},
		interval: time.Minute,
		backoff:  supervisor.NewBackoff(time.Second, time.Minute, nil),
		after: func(delay time.Duration) <-chan time.Time {
			waits = append(waits, delay)
			cancel()
			return make(chan time.Time)
		},
	}
	err := adapter.Run(ctx)
	if !errors.Is(err, context.Canceled) || attempts != 1 || len(waits) != 1 || waits[0] != time.Second {
		t.Fatalf("run err=%v attempts=%d waits=%v", err, attempts, waits)
	}
}
