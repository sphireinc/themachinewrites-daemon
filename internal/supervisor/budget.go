package supervisor

import (
	"errors"
	"sync"
	"time"
)

type BudgetLimits struct {
	Invocations  int
	InputTokens  int
	OutputTokens int
	MaxRetries   int
}
type Budget struct {
	mu                                  sync.Mutex
	day                                 string
	limits                              BudgetLimits
	invocations, input, output, retries int
}

func NewBudget(now time.Time, limits BudgetLimits) *Budget {
	return &Budget{day: now.UTC().Format("2006-01-02"), limits: limits}
}
func (b *Budget) reserve(now time.Time, in, out int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	day := now.UTC().Format("2006-01-02")
	if day != b.day {
		b.day = day
		b.invocations, b.input, b.output, b.retries = 0, 0, 0, 0
	}
	if b.invocations+1 > b.limits.Invocations || b.input+in > b.limits.InputTokens || b.output+out > b.limits.OutputTokens {
		return errors.New("daily Codex budget depleted")
	}
	b.invocations++
	b.input += in
	b.output += out
	return nil
}

func (b *Budget) Reserve(now time.Time, inputTokens, outputTokens int) error {
	return b.reserve(now, inputTokens, outputTokens)
}
