package main

import (
	"context"
	"errors"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
)

type loopAdapter struct {
	cycle    supervisor.Cycle
	interval time.Duration
}

func (a loopAdapter) Once(ctx context.Context) error { return a.cycle.Once(ctx) }
func (a loopAdapter) Run(ctx context.Context) error {
	for {
		if err := a.cycle.Once(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(a.interval):
		}
	}
}
