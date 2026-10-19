package main

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
)

type loopAdapter struct {
	cycle    supervisor.Cycle
	interval time.Duration
	backoff  *supervisor.Backoff
	after    func(time.Duration) <-chan time.Time
}

var terminateSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func (a loopAdapter) Once(ctx context.Context) error { return a.cycle.Once(ctx) }
func (a loopAdapter) Run(ctx context.Context) error {
	after := a.after
	if after == nil {
		after = time.After
	}
	for {
		err := a.cycle.Once(ctx)
		wait := a.interval
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return ctx.Err()
			}
			wait = a.backoff.Next()
		} else if a.backoff != nil {
			a.backoff.Reset()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-after(wait):
		}
	}
}
