package supervisor

import (
	"context"
	"errors"
)

type Cycle struct {
	Poll    func(context.Context) []error
	Decide  func(context.Context) error
	Publish func(context.Context) error
	DryRun  bool
}

func (c Cycle) Once(ctx context.Context) error {
	if c.Poll == nil {
		return errors.New("poll step is required")
	}
	if errs := c.Poll(ctx); len(errs) > 0 {
		return errors.Join(errs...)
	}
	if c.Decide != nil {
		if err := c.Decide(ctx); err != nil {
			return err
		}
	}
	if c.Publish != nil {
		if c.DryRun {
			return nil
		}
		return c.Publish(ctx)
	}
	return nil
}
