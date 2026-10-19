package supervisor

import (
	"context"
	"errors"
	"sync"
	"time"
)

func GracefulShutdown(ctx context.Context, timeout time.Duration, closeFn func() error) <-chan error {
	ch := make(chan error, 1)
	var once sync.Once
	go func() {
		<-ctx.Done()
		once.Do(func() {
			if closeFn == nil {
				ch <- nil
				return
			}
			done := make(chan error, 1)
			go func() { done <- closeFn() }()
			select {
			case err := <-done:
				ch <- err
			case <-time.After(timeout):
				ch <- errors.New("graceful shutdown deadline exceeded")
			}
		})
	}()
	return ch
}
