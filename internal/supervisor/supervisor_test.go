package supervisor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
	mu  sync.Mutex
}

func (c *fakeClock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

func TestPollSchedulerHonorsPerSourceIntervalAndGlobalMinimum(t *testing.T) {
	now := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	scheduler := NewPollScheduler(clock, time.Minute)
	var called []string
	scheduler.Add(SourceSchedule{ID: "fast", Interval: 30 * time.Second, Enabled: true, Run: func(context.Context) error { called = append(called, "fast"); return nil }})
	scheduler.Add(SourceSchedule{ID: "slow", Interval: 2 * time.Minute, Enabled: true, Run: func(context.Context) error { called = append(called, "slow"); return nil }})
	scheduler.Tick(context.Background())
	if len(called) != 2 {
		t.Fatalf("initial fanout=%v", called)
	}
	called = nil
	clock.Advance(30 * time.Second)
	scheduler.Tick(context.Background())
	if len(called) != 0 {
		t.Fatalf("minimum interval bypassed: %v", called)
	}
	clock.Advance(31 * time.Second)
	scheduler.Tick(context.Background())
	if len(called) != 1 || called[0] != "fast" {
		t.Fatalf("bounded schedule=%v", called)
	}
}

func TestDecisionGateAvoidsForcedPublication(t *testing.T) {
	gate := DecisionGate{}
	if gate.ShouldInvoke(DecisionInputs{}) {
		t.Fatal("empty input invoked agent")
	}
	if !gate.ShouldInvoke(DecisionInputs{NewCandidates: 1}) {
		t.Fatal("novel candidate did not invoke")
	}
}

func TestPublishingLeaseExcludesSecondWorker(t *testing.T) {
	directory := t.TempDir()
	first, err := AcquireLease(directory, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := AcquireLease(directory, "worker-b"); err == nil {
		t.Fatal("second worker acquired same publishing lease")
	}
}

func TestDailyBudgetCapsAndResetsAtUTCMidnight(t *testing.T) {
	now := time.Date(2026, 10, 18, 23, 0, 0, 0, time.UTC)
	budget := NewBudget(now, BudgetLimits{Invocations: 1, InputTokens: 100, OutputTokens: 50, MaxRetries: 2})
	if err := budget.reserve(now, 80, 40); err != nil {
		t.Fatal(err)
	}
	if err := budget.reserve(now, 1, 1); err == nil {
		t.Fatal("budget cap not enforced")
	}
	next := now.Add(2 * time.Hour)
	if err := budget.reserve(next, 80, 40); err != nil {
		t.Fatalf("UTC day reset: %v", err)
	}
}

func TestBackoffBoundedAndResets(t *testing.T) {
	b := NewBackoff(time.Second, 10*time.Second, func(int64) int64 { return 0 })
	last := time.Duration(0)
	for range 10 {
		if d := b.Next(); d > 10*time.Second || d <= 0 || d < last {
			t.Fatalf("delay=%s", d)
		} else {
			last = d
		}
	}
	b.Reset()
	if b.Next() != time.Second {
		t.Fatal("backoff reset failed")
	}
}

func TestGracefulShutdownCancelsAndClosesOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	closed := 0
	done := GracefulShutdown(ctx, time.Second, func() error { closed++; return nil })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("close count=%d", closed)
	}
}

func TestDryRunCycleNeverPublishes(t *testing.T) {
	published := false
	cycle := Cycle{DryRun: true, Poll: func(context.Context) []error { return nil }, Publish: func(context.Context) error { published = true; return nil }}
	if err := cycle.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if published {
		t.Fatal("dry-run published")
	}
}

var _ = errors.Is
